package authentication_test

import (
 "context"
 "crypto"
 "crypto/ed25519"
 "crypto/rand"
 "crypto/sha256"
 "encoding/base64"
 "encoding/json"
 "errors"
 "net"
 "net/http"
 "os"
 "path/filepath"
 "sync"
 "sync/atomic"
 "testing"
 "time"

 "github.com/bogachenko/bfe-chat-service/internal/authentication"
 jose "github.com/go-jose/go-jose/v4"
)

const authURL = "https://chat.example.test/connection"
const fakeAccount = "11111111-1111-4111-8111-111111111111"

type fixture struct {
 validator *authentication.Validator
 private ed25519.PrivateKey
 public ed25519.PublicKey
 jkt string
 token string
 hits atomic.Int64
 status atomic.Int64
}

func newFixture(t *testing.T)*fixture{
 t.Helper()
 dir:=t.TempDir();socket:=filepath.Join(dir,"auth.sock")
 listener,err:=net.Listen("unix",socket);if err!=nil{t.Fatal(err)}
 t.Cleanup(func(){_ =listener.Close()})
 public,private,err:=ed25519.GenerateKey(rand.Reader);if err!=nil{t.Fatal(err)}
 jwk:=jose.JSONWebKey{Key:public}
 thumb,err:=jwk.Thumbprint(crypto.SHA256);if err!=nil{t.Fatal(err)}
 f:=&fixture{private:private,public:public,jkt:base64.RawURLEncoding.EncodeToString(thumb),token:"opaque-xyz-456"}
 f.status.Store(http.StatusOK)
 mux:=http.NewServeMux()
 mux.HandleFunc("/internal/auth/validate",func(w http.ResponseWriter,r *http.Request){
  f.hits.Add(1)
  if r.Method!="POST"||r.Header.Get("Authorization")!="DPoP "+f.token{w.WriteHeader(http.StatusUnauthorized);return}
  status:=int(f.status.Load());w.WriteHeader(status)
  if status==http.StatusOK{_ =json.NewEncoder(w).Encode(map[string]string{"accountId":fakeAccount,"dpopJkt":f.jkt,"administrativeStatus":"NON_ADMIN"})}
 })
 srv:=&http.Server{Handler:mux}
 go func(){_ =srv.Serve(listener)}()
 t.Cleanup(func(){_ =srv.Close()})
 f.validator,err=authentication.NewValidator(socket,5*time.Minute);if err!=nil{t.Fatal(err)}
 return f
}

func (f *fixture) proof(t *testing.T,jti,method,target,token string,issued time.Time)string{
 t.Helper()
 signer,err:=jose.NewSigner(jose.SigningKey{Algorithm:jose.EdDSA,Key:jose.JSONWebKey{Key:f.private,Algorithm:string(jose.EdDSA)}},&jose.SignerOptions{EmbedJWK:true,ExtraHeaders:map[jose.HeaderKey]interface{}{jose.HeaderType:"dpop+jwt"}})
 if err!=nil{t.Fatal(err)}
 ath:=sha256.Sum256([]byte(token))
 body,err:=json.Marshal(map[string]interface{}{"jti":jti,"htm":method,"htu":target,"iat":issued.Unix(),"ath":base64.RawURLEncoding.EncodeToString(ath[:])})
 if err!=nil{t.Fatal(err)}
 result,err:=signer.Sign(body);if err!=nil{t.Fatal(err)}
 encoded,err:=result.CompactSerialize();if err!=nil{t.Fatal(err)}
 return encoded
}
func (f *fixture) valid(t *testing.T,jti string)string{
 t.Helper();return f.proof(t,jti,"GET",authURL,f.token,time.Now())
}
func TestFC01UDSAndSecurity(t *testing.T){
 f:=newFixture(t)
 cases:=[]struct{name string;makeProof func(*testing.T)string;want error}{
  {"valid",func(t *testing.T)string{return f.valid(t,"first")},nil},
  {"wrong ATH",func(t *testing.T)string{return f.proof(t,"ath","GET",authURL,"wrong-token",time.Now())},authentication.ErrProofInvalid},
  {"wrong HTM",func(t *testing.T)string{return f.proof(t,"method","POST",authURL,f.token,time.Now())},authentication.ErrProofInvalid},
  {"wrong HTU",func(t *testing.T)string{return f.proof(t,"url","GET","https://wrong.example/connection",f.token,time.Now())},authentication.ErrProofInvalid},
  {"expired IAT",func(t *testing.T)string{return f.proof(t,"old","GET",authURL,f.token,time.Now().Add(-10*time.Minute))},authentication.ErrProofInvalid},
  {"future IAT",func(t *testing.T)string{return f.proof(t,"future","GET",authURL,f.token,time.Now().Add(3*time.Minute))},authentication.ErrProofInvalid},
  {"malformed JWS",func(t *testing.T)string{return "invalid"},authentication.ErrProofInvalid},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){
  got,err:=f.validator.ValidateCaller(context.Background(),f.token,tc.makeProof(t),authentication.RequestBinding{Method:"GET",Target:authURL})
  if tc.want!=nil{if !errors.Is(err,tc.want)||got!=(authentication.Caller{}){t.Fatalf("caller=%+v error=%v",got,err)};return}
  if err!=nil||got.AccountID!=fakeAccount||got.AdministrativeStatus!="NON_ADMIN"{t.Fatalf("caller=%+v error=%v",got,err)}
 })}
 if f.hits.Load()!=int64(len(cases)){t.Fatalf("UDS requests=%d",f.hits.Load())}
}
func TestFC01FailClosed(t *testing.T){
 f:=newFixture(t)
 f.status.Store(http.StatusUnauthorized)
 if _,err:=f.validator.ValidateCaller(context.Background(),f.token,f.valid(t,"revoked"),authentication.RequestBinding{Method:"GET",Target:authURL});!errors.Is(err,authentication.ErrAuthInvalid){t.Fatalf("revoked: %v",err)}
 f.status.Store(http.StatusServiceUnavailable)
 if _,err:=f.validator.ValidateCaller(context.Background(),f.token,f.valid(t,"down"),authentication.RequestBinding{Method:"GET",Target:authURL});!errors.Is(err,authentication.ErrAuthUnavailable){t.Fatalf("unavailable: %v",err)}
 absent,err:=authentication.NewValidator(filepath.Join(t.TempDir(),"missing.sock"),time.Minute);if err!=nil{t.Fatal(err)}
 if _,err:=absent.ValidateCaller(context.Background(),f.token,f.valid(t,"uds-down"),authentication.RequestBinding{Method:"GET",Target:authURL});!errors.Is(err,authentication.ErrAuthUnavailable){t.Fatalf("UDS down: %v",err)}
 if _,err:=f.validator.ValidateCaller(context.Background(),"",f.valid(t,"empty"),authentication.RequestBinding{Method:"GET",Target:authURL});!errors.Is(err,authentication.ErrAuthInvalid){t.Fatalf("no token: %v",err)}
}
func TestMSF01ReplayConcurrencyReconnect(t *testing.T){
 f:=newFixture(t)
 const workers=24
 proof:=f.valid(t,"shared-jti")
 var wg sync.WaitGroup
 start:=make(chan struct{})
 var success atomic.Int64
 for i:=0;i<workers;i++{wg.Add(1);go func(){defer wg.Done();<-start;_,err:=f.validator.ValidateCaller(context.Background(),f.token,proof,authentication.RequestBinding{Method:"GET",Target:authURL});if err==nil{success.Add(1)}else if !errors.Is(err,authentication.ErrProofInvalid){t.Errorf("unexpected error: %v",err)}}()}
 close(start);wg.Wait()
 if n:=success.Load();n!=1{t.Fatalf("same JKT/JTI accepted %d times",n)}
 fresh:=f.valid(t,"new-connect-jti")
 if _,err:=f.validator.ValidateCaller(context.Background(),f.token,fresh,authentication.RequestBinding{Method:"GET",Target:authURL});err!=nil{t.Fatalf("fresh proof rejected: %v",err)}
 if _,err:=f.validator.ValidateCaller(context.Background(),f.token,fresh,authentication.RequestBinding{Method:"GET",Target:authURL});!errors.Is(err,authentication.ErrProofInvalid){t.Fatalf("reconnect reused JTI: %v",err)}
}
func TestFC02WebSocketBinding(t *testing.T){
 f:=newFixture(t)
 proof:=f.proof(t,"websocket","GET",authURL,f.token,time.Now())
 caller,err:=f.validator.ValidateOnConnecting(context.Background(),f.token,proof,"wss://chat.example.test/connection")
 if err!=nil||caller.AccountID!=fakeAccount{t.Fatalf("connected=%+v error=%v",caller,err)}
 if _,err:=f.validator.ValidateOnConnecting(context.Background(),f.token,proof,"wss://chat.example.test/connection");!errors.Is(err,authentication.ErrProofInvalid){t.Fatalf("reused websocket proof: %v",err)}
 for _,bad:=range []string{"ws://chat.example.test/connection","wss://chat.example.test/connection?token=secret","wss://user@chat.example.test/connection"}{
  if _,err:=f.validator.ValidateOnConnecting(context.Background(),f.token,f.valid(t,"bad-ws-"+bad),bad);!errors.Is(err,authentication.ErrProofInvalid){t.Fatalf("bad WSS URL %q: %v",bad,err)}
 }
}
func TestFC01Initialization(t *testing.T){
 for _,tc:=range []struct{name,path string;window time.Duration}{{"missing socket","",time.Minute},{"zero freshness","/tmp/fake",0}}{
  t.Run(tc.name,func(t *testing.T){if _,err:=authentication.NewValidator(tc.path,tc.window);!errors.Is(err,authentication.ErrAuthUnavailable){t.Fatal(err)}})
 }
 _ =os.ErrClosed
}

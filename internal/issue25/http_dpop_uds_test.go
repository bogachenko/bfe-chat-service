package issue25

import (
 "crypto"
 "crypto/ed25519"
 "crypto/rand"
 "crypto/sha256"
 "database/sql"
 "encoding/base64"
 "encoding/json"
 "net"
 "net/http"
 "net/http/httptest"
 "path/filepath"
 "strings"
 "sync/atomic"
 "testing"
 "time"

 "github.com/bogachenko/bfe-chat-service/internal/authentication"
 jose "github.com/go-jose/go-jose/v4"
)

func TestHTTPv2RealValidatorUDS(t *testing.T) {
 const token="opaque-session-token"
 const target="https://chat.example.test/v2/conversations/unknown"
 public,private,err:=ed25519.GenerateKey(rand.Reader)
 if err!=nil { t.Fatal(err) }
 thumb,err:=(&jose.JSONWebKey{Key:public}).Thumbprint(crypto.SHA256)
 if err!=nil { t.Fatal(err) }
 jkt:=base64.RawURLEncoding.EncodeToString(thumb)
 socket:=filepath.Join(t.TempDir(),"auth.sock")
 listener,err:=net.Listen("unix",socket)
 if err!=nil { t.Fatal(err) }
 var status atomic.Int64
 var hits atomic.Int64
 status.Store(http.StatusOK)
 server:=&http.Server{Handler:http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  hits.Add(1)
  if r.Method!=http.MethodPost || r.URL.Path!="/internal/auth/validate" || r.Header.Get("Authorization")!="DPoP "+token {
   w.WriteHeader(http.StatusUnauthorized);return
  }
  w.WriteHeader(int(status.Load()))
  if status.Load()==http.StatusOK {
   _=json.NewEncoder(w).Encode(map[string]string{"accountId":"11111111-1111-4111-8111-111111111111","dpopJkt":jkt,"administrativeStatus":"NON_ADMIN"})
  }
 })}
 go func(){_ =server.Serve(listener)}()
 t.Cleanup(func(){_ =server.Close();_ =listener.Close()})
 validator,err:=authentication.NewValidator(socket,5*time.Minute)
 if err!=nil { t.Fatal(err) }
 unavailable,err:=authentication.NewValidator(filepath.Join(t.TempDir(),"missing.sock"),time.Minute)
 if err!=nil { t.Fatal(err) }
 sign:=func(t *testing.T,jti,method,uri,athToken string,issued time.Time,alterSignature bool)string{
  t.Helper()
  signer,err:=jose.NewSigner(jose.SigningKey{Algorithm:jose.EdDSA,Key:jose.JSONWebKey{Key:private,Algorithm:string(jose.EdDSA)}},&jose.SignerOptions{EmbedJWK:true,ExtraHeaders:map[jose.HeaderKey]interface{}{jose.HeaderType:"dpop+jwt"}})
  if err!=nil {t.Fatal(err)}
  hash:=sha256.Sum256([]byte(athToken))
  body,err:=json.Marshal(map[string]any{"jti":jti,"htm":method,"htu":uri,"iat":issued.Unix(),"ath":base64.RawURLEncoding.EncodeToString(hash[:])})
  if err!=nil {t.Fatal(err)}
  signed,err:=signer.Sign(body)
  if err!=nil {t.Fatal(err)}
  compact,err:=signed.CompactSerialize()
  if err!=nil {t.Fatal(err)}
  if alterSignature {
   parts:=strings.Split(compact,".")
   if len(parts)!=3 || len(parts[2])==0 {t.Fatal("bad fixture")}
   if parts[2][0]=='A' {parts[2]="B"+parts[2][1:]} else {parts[2]="A"+parts[2][1:]}
   compact=strings.Join(parts,".")
  }
  return compact
 }
 cases:=[]struct{name,scheme,method,uri,athToken string; age time.Duration; broken,revoked,down bool;want int}{
  {"valid DPoP","DPoP",http.MethodGet,target,token,0,false,false,false,http.StatusNotFound},
  {"valid legacy Bearer","Bearer",http.MethodGet,target,token,0,false,false,false,http.StatusNotFound},
  {"bad signature","DPoP",http.MethodGet,target,token,0,true,false,false,http.StatusUnauthorized},
  {"wrong htm","DPoP",http.MethodPost,target,token,0,false,false,false,http.StatusUnauthorized},
  {"wrong htu","DPoP",http.MethodGet,"https://other.invalid/v2/conversations/unknown",token,0,false,false,false,http.StatusUnauthorized},
  {"expired","DPoP",http.MethodGet,target,token,-10*time.Minute,false,false,false,http.StatusUnauthorized},
  {"revoked","DPoP",http.MethodGet,target,token,0,false,true,false,http.StatusUnauthorized},
  {"bad ath","DPoP",http.MethodGet,target,"other-token",0,false,false,false,http.StatusUnauthorized},
  {"UDS down","DPoP",http.MethodGet,target,token,0,false,false,true,http.StatusServiceUnavailable},
 }
 for i,tc:=range cases {
  t.Run(tc.name,func(t *testing.T){
   status.Store(http.StatusOK)
   if tc.revoked {status.Store(http.StatusUnauthorized)}
   h:=HTTPHandler{Store:Store{DB:&sql.DB{}},Auth:validator,PublicOrigin:"https://chat.example.test"}
   if tc.down {h.Auth=unavailable}
   proof:=sign(t,tc.name+string(rune('a'+i)),tc.method,tc.uri,tc.athToken,time.Now().Add(tc.age),tc.broken)
   req:=httptest.NewRequest(http.MethodGet,"/v2/conversations/unknown",nil)
   req.Header.Set("Authorization",tc.scheme+" "+token)
   req.Header.Set("DPoP",proof)
   recorder:=httptest.NewRecorder()
   h.ServeHTTP(recorder,req)
   if recorder.Code!=tc.want {t.Fatalf("status=%d want=%d body=%s",recorder.Code,tc.want,recorder.Body.String())}
   if tc.want!=http.StatusNotFound && !strings.Contains(recorder.Body.String(),"AUTH_") && !strings.Contains(recorder.Body.String(),"PROOF_") {t.Fatalf("unexpected auth error: %s",recorder.Body.String())}
  })
 }
 if hits.Load()!=int64(len(cases)-1) {t.Fatalf("UDS hits=%d",hits.Load())}
}

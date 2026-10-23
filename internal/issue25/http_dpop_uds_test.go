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

const udsTestToken25 = "opaque-session-token"
const udsTestTarget25 = "https://chat.example.test/conversations/unknown"

type udsCase25 struct {
 name,scheme,method,uri,athToken string
 age time.Duration
 broken,revoked,down bool
 want int
}

func udsAuthHandler25(jkt string, status *atomic.Int64, hits *atomic.Int64) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  hits.Add(1)
  if r.Method!=http.MethodPost || r.URL.Path!="/internal/auth/validate" || r.Header.Get("Authorization")!="DPoP "+udsTestToken25 {
   w.WriteHeader(http.StatusUnauthorized); return
  }
  w.WriteHeader(int(status.Load()))
  if status.Load()==http.StatusOK {
   _=json.NewEncoder(w).Encode(map[string]string{"accountId":"11111111-1111-4111-8111-111111111111","dpopJkt":jkt,"administrativeStatus":"NON_ADMIN"})
  }
 })
}

func signUDSProof25(t *testing.T,private ed25519.PrivateKey,tc udsCase25) string {
 t.Helper()
 signer,err:=jose.NewSigner(jose.SigningKey{Algorithm:jose.EdDSA,Key:jose.JSONWebKey{Key:private,Algorithm:string(jose.EdDSA)}},&jose.SignerOptions{EmbedJWK:true,ExtraHeaders:map[jose.HeaderKey]interface{}{jose.HeaderType:"dpop+jwt"}})
 if err!=nil {t.Fatal(err)}
 hash:=sha256.Sum256([]byte(tc.athToken))
 body,err:=json.Marshal(map[string]any{"jti":tc.name,"htm":tc.method,"htu":tc.uri,"iat":time.Now().Add(tc.age).Unix(),"ath":base64.RawURLEncoding.EncodeToString(hash[:])})
 if err!=nil {t.Fatal(err)}
 signed,err:=signer.Sign(body)
 if err!=nil {t.Fatal(err)}
 compact,err:=signed.CompactSerialize()
 if err!=nil {t.Fatal(err)}
 if tc.broken {
  parts:=strings.Split(compact,".")
  if parts[2][0]=='A' {parts[2]="B"+parts[2][1:]} else {parts[2]="A"+parts[2][1:]}
  compact=strings.Join(parts,".")
 }
 return compact
}

func runUDSCase25(t *testing.T,tc udsCase25,private ed25519.PrivateKey,status *atomic.Int64,validator,unavailable *authentication.Validator) {
 t.Helper()
 status.Store(http.StatusOK)
 if tc.revoked {status.Store(http.StatusUnauthorized)}
 h:=HTTPHandler{Store:Store{DB:&sql.DB{}},Auth:validator,PublicOrigin:"https://chat.example.test"}
 if tc.down {h.Auth=unavailable}
 req:=httptest.NewRequest(http.MethodGet,"/conversations/unknown",nil)
 req.Header.Set("Authorization",tc.scheme+" "+udsTestToken25)
 req.Header.Set("DPoP",signUDSProof25(t,private,tc))
 recorder:=httptest.NewRecorder()
 h.ServeHTTP(recorder,req)
 if recorder.Code!=tc.want {t.Fatalf("status=%d want=%d body=%s",recorder.Code,tc.want,recorder.Body.String())}
 if tc.want!=http.StatusNotFound && !strings.Contains(recorder.Body.String(),"AUTH_") && !strings.Contains(recorder.Body.String(),"PROOF_") {
  t.Fatalf("unexpected authorization error: %s",recorder.Body.String())
 }
}

func TestHTTPv2RealValidatorUDS(t *testing.T) {
 public,private,err:=ed25519.GenerateKey(rand.Reader)
 if err!=nil {t.Fatal(err)}
 thumb,err:=(&jose.JSONWebKey{Key:public}).Thumbprint(crypto.SHA256)
 if err!=nil {t.Fatal(err)}
 socket:=filepath.Join(t.TempDir(),"auth.sock")
 listener,err:=net.Listen("unix",socket)
 if err!=nil {t.Fatal(err)}
 var status,hits atomic.Int64
 status.Store(http.StatusOK)
 server:=&http.Server{Handler:udsAuthHandler25(base64.RawURLEncoding.EncodeToString(thumb),&status,&hits)}
 go func(){_ =server.Serve(listener)}()
 t.Cleanup(func(){_ =server.Close();_ =listener.Close()})
 validator,err:=authentication.NewValidator(socket,5*time.Minute)
 if err!=nil {t.Fatal(err)}
 unavailable,err:=authentication.NewValidator(filepath.Join(t.TempDir(),"missing.sock"),time.Minute)
 if err!=nil {t.Fatal(err)}
 cases:=[]udsCase25{
  {"valid DPoP","DPoP",http.MethodGet,udsTestTarget25,udsTestToken25,0,false,false,false,http.StatusNotFound},
  {"Bearer rejected before UDS","Bearer",http.MethodGet,udsTestTarget25,udsTestToken25,0,false,false,false,http.StatusUnauthorized},
  {"bad signature","DPoP",http.MethodGet,udsTestTarget25,udsTestToken25,0,true,false,false,http.StatusUnauthorized},
  {"wrong htm","DPoP",http.MethodPost,udsTestTarget25,udsTestToken25,0,false,false,false,http.StatusUnauthorized},
  {"wrong htu","DPoP",http.MethodGet,"https://other.invalid/conversations/unknown",udsTestToken25,0,false,false,false,http.StatusUnauthorized},
  {"expired","DPoP",http.MethodGet,udsTestTarget25,udsTestToken25,-10*time.Minute,false,false,false,http.StatusUnauthorized},
  {"revoked","DPoP",http.MethodGet,udsTestTarget25,udsTestToken25,0,false,true,false,http.StatusUnauthorized},
  {"bad ath","DPoP",http.MethodGet,udsTestTarget25,"other-udsTestToken25",0,false,false,false,http.StatusUnauthorized},
  {"UDS down","DPoP",http.MethodGet,udsTestTarget25,udsTestToken25,0,false,false,true,http.StatusServiceUnavailable},
 }
 for _,tc:=range cases {t.Run(tc.name,func(t *testing.T){runUDSCase25(t,tc,private,&status,validator,unavailable)})}
 if hits.Load()!=int64(len(cases)-2) {t.Fatalf("UDS hits=%d",hits.Load())}
}

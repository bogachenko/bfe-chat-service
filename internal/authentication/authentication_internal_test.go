package authentication

import (
 "crypto"
 "crypto/ed25519"
 "crypto/rand"
 "encoding/base64"
 "net/http"
 "testing"
 "time"

 jose "github.com/go-jose/go-jose/v4"
)

func TestCanonicalTargetCases(t *testing.T){
 cases:=[]struct{name,url,want string;bad bool}{
  {"normal","https://CHAT.example.test/path","https://chat.example.test/path",false},
  {"default port","https://CHAT.example.test:443","https://chat.example.test/",false},
  {"alternate port","https://chat.example.test:9443/","https://chat.example.test:9443/",false},
  {"ipv6","https://[::1]/c","https://[::1]/c",false},
  {"ipv6 port","https://[::1]:8443/c","https://[::1]:8443/c",false},
  {"empty path","https://chat.example.test","https://chat.example.test/",false},
  {"http","http://chat.example.test","",true},
  {"empty host","https:///no-host","",true},
  {"userinfo","https://user:secret@chat.example.test","",true},
  {"fragment","https://chat.example.test/#secret","",true},
  {"query","https://chat.example.test/?token=secret","",true},
  {"bad escape","https://chat.example.test/%zz","",true},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){
  got,err:=canonicalTarget(tc.url,false)
  if tc.bad{if err==nil{t.Fatalf("accepted %q: %q",tc.url,got)};return}
  if err!=nil||got!=tc.want{t.Fatalf("canonical=%q err=%v want=%q",got,err,tc.want)}
 })}
}
func TestReplayStoreExpiryAndSeparation(t *testing.T){
 v:=&Validator{freshness:time.Minute,seen:make(map[replayKey]time.Time)}
 now:=time.Now().UTC()
 a:=replayKey{JKT:"a",JTI:"same"}
 b:=replayKey{JKT:"b",JTI:"same"}
 if err:=v.checkReplay(a,now,now.Add(time.Minute));err!=nil{t.Fatal(err)}
 if err:=v.checkReplay(a,now,now.Add(time.Minute));err!=ErrProofInvalid{t.Fatalf("accepted duplicate: %v",err)}
 if err:=v.checkReplay(b,now,now.Add(time.Minute));err!=nil{t.Fatalf("separate key rejected: %v",err)}
 if err:=v.checkReplay(a,now.Add(2*time.Minute),now.Add(3*time.Minute));err!=nil{t.Fatalf("expiry not swept: %v",err)}
 if len(v.seen)!=1{t.Fatalf("stale entries not removed: %d",len(v.seen))}
}
func TestVerifiedClaimsHeaderAndThumbprint(t *testing.T){
 pub,priv,err:=ed25519.GenerateKey(rand.Reader)
 if err!=nil{t.Fatal(err)}
 thumb,err:=(&jose.JSONWebKey{Key:pub}).Thumbprint(crypto.SHA256)
 if err!=nil{t.Fatal(err)}
 jkt:=base64.RawURLEncoding.EncodeToString(thumb)
 for _,tc:=range []struct{name,typ string;body string;expected string;valid bool}{
  {"valid","dpop+jwt",`{"jti":"abc","iat":1700000000}`,jkt,true},
  {"wrong type","JWT",`{"jti":"abc","iat":1700000000}`,jkt,false},
  {"missing jti","dpop+jwt",`{"iat":1700000000}`,jkt,false},
  {"missing iat","dpop+jwt",`{"jti":"abc"}`,jkt,false},
  {"bad json","dpop+jwt","{broken",jkt,false},
  {"wrong JKT","dpop+jwt",`{"jti":"abc","iat":1700000000}`,"incorrect",false},
 }{
  t.Run(tc.name,func(t *testing.T){
   compact:=makeHeaderTestJWS(t,priv,tc.typ,tc.body)
   assertHeaderClaims(t,compact,tc.expected,tc.valid)
  })
 }
}
func makeHeaderTestJWS(t *testing.T,priv ed25519.PrivateKey,typ,body string)string{
 t.Helper()
 signer,err:=jose.NewSigner(jose.SigningKey{Algorithm:jose.EdDSA,Key:jose.JSONWebKey{Key:priv,Algorithm:string(jose.EdDSA)}},&jose.SignerOptions{EmbedJWK:true,ExtraHeaders:map[jose.HeaderKey]interface{}{jose.HeaderType:typ}})
 if err!=nil{t.Fatal(err)}
 signed,err:=signer.Sign([]byte(body));if err!=nil{t.Fatal(err)}
 compact,err:=signed.CompactSerialize();if err!=nil{t.Fatal(err)}
 return compact
}
func assertHeaderClaims(t *testing.T,compact,expected string,valid bool){
 t.Helper()
 got,err:=verifiedClaims(compact,expected)
 if valid{
  if err!=nil||got.JTI!="abc"{t.Fatalf("valid: %+v %v",got,err)}
  return
 }
 if err==nil{t.Fatalf("invalid proof accepted: %+v",got)}
}
func TestValidateProofInvalidRequestBoundary(t *testing.T){
 v:=&Validator{freshness:time.Minute,seen:make(map[replayKey]time.Time)}
 for _,tc:=range []RequestBinding{{Method:http.MethodGet,Target:"http://example.com"},{Method:"",Target:"https://example.com"},{Method:http.MethodGet,Target:"https://user:secret@example.com"},{Method:http.MethodGet,Target:"https://example.com?a=b"}}{
  if err:=v.validateProof("invalid","token","jkt",tc);err!=ErrProofInvalid{t.Fatalf("accepted %+v: %v",tc,err)}
 }
}

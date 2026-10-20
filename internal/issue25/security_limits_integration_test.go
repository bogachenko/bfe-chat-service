package issue25

import (
 "context"
 "fmt"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"

 "github.com/bogachenko/bfe-chat-service/internal/authentication"
)
type rejectCaller struct{err error}
func (v rejectCaller) ValidateCaller(context.Context,string,string,authentication.RequestBinding)(authentication.Caller,error){return authentication.Caller{},v.err}
func TestPGMS2513NewHTTPFailClosed(t *testing.T){
 db:=integrationDB(t)
 for _,tc:=range []struct{err error;status int}{
  {authentication.ErrAuthInvalid,401},
  {authentication.ErrProofInvalid,401},
  {authentication.ErrAuthUnavailable,503},
 }{
  h:=HTTPHandler{Store:Store{DB:db},Auth:rejectCaller{err:tc.err},PublicOrigin:"https://example.invalid"}
  w:=httptest.NewRecorder();r:=httptest.NewRequest("POST","/v2/conversations/direct",strings.NewReader(`{"peerId":"00000000-0000-4000-8000-000000000001"}`))
  r.Header.Set("Authorization","Bearer token");r.Header.Set("DPoP","proof")
  h.ServeHTTP(w,r)
  if w.Code!=tc.status{t.Fatalf("err %v: status %d expected %d",tc.err,w.Code,tc.status)}
 }
 outsider:=fixtureID(t);owner:=fixtureID(t)
 group,err:=(Store{DB:db}).CreateGroup(context.Background(),owner,"private",nil);if err!=nil{t.Fatal(err)}
 h:=HTTPHandler{Store:Store{DB:db},Auth:fakeCaller{id:outsider},PublicOrigin:"https://example.invalid"}
 w:=httptest.NewRecorder();r:=httptest.NewRequest("GET","/v2/conversations/"+group+"/history",nil)
 r.Header.Set("Authorization","Bearer test-token");r.Header.Set("DPoP","test-proof");h.ServeHTTP(w,r)
 if w.Code!=403{t.Fatalf("nonmember history code=%d body=%s",w.Code,w.Body.String())}
}
func TestPGMS2515RequestLimitsAndAtomicity(t *testing.T){
 db:=integrationDB(t);user:=fixtureID(t)
 h:=HTTPHandler{Store:Store{DB:db},Auth:fakeCaller{id:user},PublicOrigin:"https://example.invalid"}
 call:=func(path,body string)int{
  t.Helper()
  w:=httptest.NewRecorder();r:=httptest.NewRequest(http.MethodPost,path,strings.NewReader(body))
  r.Header.Set("Authorization","Bearer test-token");r.Header.Set("DPoP","test-proof")
  h.ServeHTTP(w,r);return w.Code
 }
 equal:=`{"name":"group"}`+strings.Repeat(" ",65536-len(`{"name":"group"}`))
 if got:=call("/v2/conversations/group",equal);got!=200{t.Fatalf("65536 bytes status=%d",got)}
 above:=equal+" "
 if got:=call("/v2/conversations/group",above);got!=400{t.Fatalf("65537 bytes status=%d",got)}
 if got:=call("/v2/conversations/group",fmt.Sprintf(`{"name":"%s"}`,strings.Repeat("a",256)));got!=200{t.Fatalf("256 name status=%d",got)}
 if got:=call("/v2/conversations/group",fmt.Sprintf(`{"name":"%s"}`,strings.Repeat("a",257)));got!=400{t.Fatalf("257 name status=%d",got)}
}

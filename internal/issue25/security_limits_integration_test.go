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
  qaRequiref(t, !(w.Code!=tc.status), "err %v: status %d expected %d",tc.err,w.Code,tc.status)
 }
 outsider:=fixtureID(t);owner:=fixtureID(t)
 group,err:=(Store{DB:db}).CreateGroup(context.Background(),owner,"private",nil);qaRequire(t, !(err!=nil), err)
 h:=HTTPHandler{Store:Store{DB:db},Auth:fakeCaller{id:outsider},PublicOrigin:"https://example.invalid"}
 w:=httptest.NewRecorder();r:=httptest.NewRequest("GET","/v2/conversations/"+group+"/history",nil)
 r.Header.Set("Authorization","Bearer test-token");r.Header.Set("DPoP","test-proof");h.ServeHTTP(w,r)
 qaRequiref(t, !(w.Code!=403), "nonmember history code=%d body=%s",w.Code,w.Body.String())
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
 func() { got:=call("/v2/conversations/group",equal); qaRequiref(t, !(got!=200), "65536 bytes status=%d",got) }()
 above:=equal+" "
 func() { got:=call("/v2/conversations/group",above); qaRequiref(t, !(got!=400), "65537 bytes status=%d",got) }()
 if got:=call("/v2/conversations/group",fmt.Sprintf(`{"name":"%s"}`,strings.Repeat("a",256)));got!=200{t.Fatalf("256 name status=%d",got)}
 if got:=call("/v2/conversations/group",fmt.Sprintf(`{"name":"%s"}`,strings.Repeat("a",257)));got!=400{t.Fatalf("257 name status=%d",got)}
}

func TestPGMS2515DomainLimitEdges(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner:=fixtureID(t)
 members:=make([]string,99)
 for i:=range members{members[i]=fixtureID(t)}
 group,err:=s.CreateGroup(ctx,owner,"exactly hundred",members)
 qaRequiref(t, !(err!=nil), "100 members rejected: %v",err)
 func() { _,err=s.CreateGroup(ctx,owner,"too many",append(members,fixtureID(t))); qaRequiref(t, !(err!=ErrInvalid), "101 members should reject VALIDATION_ERROR: %v",err) }()
 validBody:=strings.Repeat("a",16384)
 if _,err=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:validBody});err!=nil{t.Fatalf("16384-byte body rejected: %v",err)}
 if _,err=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:validBody+"a"});err!=ErrInvalid{t.Fatalf("16385-byte body: %v",err)}
 documentID,documentTitle:="file-7","Title 7"
 if _,err=s.Send(ctx,group,owner,fixtureID(t),Payload{DocumentID:&documentID,DocumentTitle:&documentTitle});err!=nil{t.Fatalf("document-only message: %v",err)}
 if _,err=s.Send(ctx,group,owner,fixtureID(t),Payload{DocumentID:&documentID});err!=ErrInvalid{t.Fatalf("half document pair: %v",err)}
 func() { _,err=s.ReadHistoryDetailed(ctx,group,owner,0,100); qaRequiref(t, !(err!=nil), "history limit100: %v",err) }()
 func() { _,err=s.ReadHistoryDetailed(ctx,group,owner,0,101); qaRequiref(t, !(err!=ErrInvalid), "history limit101: %v",err) }()
 var count int
 func() { err=db.QueryRowContext(ctx,`SELECT count(*) FROM issue25_messages WHERE conversation_id=$1`,group).Scan(&count); qaRequire(t, !(err!=nil), err) }()
 qaRequiref(t, !(count!=2), "invalid requests wrote %d messages (expected2)",count)
}

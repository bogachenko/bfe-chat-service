package transport

import (
 "context"
 "database/sql"
 "errors"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "time"

 "github.com/bogachenko/bfe-chat-service/internal/assignments"
 "github.com/bogachenko/bfe-chat-service/internal/authentication"
 "github.com/bogachenko/bfe-chat-service/internal/authorization"
 "github.com/bogachenko/bfe-chat-service/internal/chat"
 "github.com/bogachenko/bfe-chat-service/internal/conversations"
 "github.com/bogachenko/bfe-chat-service/internal/realtime"
 "github.com/bogachenko/bfe-chat-service/internal/storage"
 "github.com/centrifugal/centrifuge"
)
const testOwner="11111111-1111-1111-1111-111111111111"
const testRecipient="22222222-2222-2222-2222-222222222222"
type protocolAuth struct{ err error }
func (a protocolAuth)ValidateCaller(_ context.Context,token,proof string,b authentication.RequestBinding)(authentication.Caller,error){
 if a.err!=nil{return authentication.Caller{},a.err}
 if token!="opaque"||proof!="signed"||b.Method==""||!strings.HasPrefix(b.Target,"https://chat.example.test/v1/"){return authentication.Caller{},authentication.ErrAuthInvalid}
 return authentication.Caller{AccountID:testOwner,AdministrativeStatus:"ADMIN"},nil
}
type protocolPolicy struct{ err error }
func (p protocolPolicy)AuthorizeSend(context.Context,authentication.Caller,chat.AccountID)error{return p.err}
func (p protocolPolicy)AuthorizeRead(context.Context,authentication.Caller,chat.AccountID)error{return p.err}
func (p protocolPolicy)AuthorizeOperation(context.Context,authentication.Caller,chat.AccountID,authorization.Operation)error{return p.err}
type protocolStore struct{ err error }
func (s protocolStore)CommitMessage(_ context.Context,_,_,_,_ string,id,title sql.NullString)(storage.Message,error){
 if s.err!=nil{return storage.Message{},s.err}
 if id.Valid!=title.Valid{return storage.Message{},errors.New("invalid pair")}
 return storage.Message{ID:7,RecipientID:testRecipient},nil
}
func (s protocolStore)ReadHistory(context.Context,string,int64,int)([]storage.HistoryMessage,error){
 if s.err!=nil{return nil,s.err}
 return []storage.HistoryMessage{{ID:7,Body:"hello",DocumentID:sql.NullString{String:"file",Valid:true},DocumentTitle:sql.NullString{String:"title",Valid:true}}},nil
}
func (s protocolStore)CommitAssignment(context.Context,string,string)error{return s.err}
func (s protocolStore)GetAssignment(context.Context,string)(string,error){return testRecipient,s.err}
func (s protocolStore)WithAssignmentLock(_ context.Context,_ string,callback func(string)error)error{return callback(testRecipient)}
func protocolHandler(authErr,policyErr,storeErr error)Handler{
 a:=protocolAuth{err:authErr};p:=protocolPolicy{err:policyErr};s:=protocolStore{err:storeErr}
 return Handler{PublicOrigin:"https://chat.example.test",Messages:conversations.Service{Auth:a,Permissions:p,Store:s},Assignments:assignments.Service{Auth:a,Store:s}}
}
func request(t *testing.T,h http.Handler,method,path,body string,headers map[string]string) *httptest.ResponseRecorder{
 t.Helper()
 req:=httptest.NewRequest(method,path,strings.NewReader(body))
 req.Header.Set("Authorization","DPoP opaque")
 req.Header.Set("DPoP","signed")
 if body!=""{req.Header.Set("Content-Type","application/json")}
 for k,v:=range headers{req.Header.Set(k,v)}
 w:=httptest.NewRecorder();h.ServeHTTP(w,req);return w
}
func TestFC13ServiceRouting(t *testing.T){
 for _,tc:=range []struct{name,method,path,body string;headers map[string]string;want int}{
  {"message text",http.MethodPost,"messages",`{"idempotencyKey":"33333333-3333-3333-3333-333333333333","text":"hello"}`,nil,200},
  {"message document",http.MethodPost,"messages",`{"idempotencyKey":"33333333-3333-3333-3333-333333333333","document":{"id":"file","title":"Title"}}`,nil,200},
  {"message empty",http.MethodPost,"messages",`{"idempotencyKey":"33333333-3333-3333-3333-333333333333"}`,nil,400},
  {"message invalid JSON",http.MethodPost,"messages",`{"extra":1}`,nil,400},
  {"message duplicated JSON",http.MethodPost,"messages",`{}{}`,nil,400},
  {"message too large",http.MethodPost,"messages",strings.Repeat("x",70<<10),nil,400},
  {"message content type",http.MethodPost,"messages",`{}`,map[string]string{"Content-Type":"text/plain"},415},
  {"message method",http.MethodGet,"messages","",nil,405},
  {"history",http.MethodGet,"history","",nil,200},
  {"history cursor",http.MethodGet,"history","",map[string]string{"X-After-ID":"12","X-Limit":"20"},200},
  {"history invalid cursor",http.MethodGet,"history","",map[string]string{"X-After-ID":"wat"},400},
  {"history invalid limit",http.MethodGet,"history","",map[string]string{"X-Limit":"wat"},400},
  {"history invalid method",http.MethodPost,"history","",nil,405},
  {"assignment",http.MethodPut,"assignment",`{"recipientAccountId":"22222222-2222-2222-2222-222222222222"}`,nil,200},
  {"assignment invalid recipient",http.MethodPut,"assignment",`{"recipientAccountId":"bad"}`,nil,400},
  {"assignment invalid method",http.MethodGet,"assignment","",nil,405},
  {"unknown endpoint",http.MethodGet,"unexpected","",nil,404},
 } {
  t.Run(tc.name,func(t *testing.T){
   path:="https://chat.example.test/v1/conversations/"+testOwner+"/"+tc.path
   w:=request(t,protocolHandler(nil,nil,nil),tc.method,path,tc.body,tc.headers)
   if w.Code!=tc.want{t.Fatalf("status=%d want=%d body=%s",w.Code,tc.want,w.Body.String())}
  })
 }
}
func TestFC13AuthorizationAndInfrastructureFailures(t *testing.T){
 for _,tc:=range []struct{name string;auth,policy,store error;want int}{
  {"invalid token",authentication.ErrAuthInvalid,nil,nil,401},
  {"DPoP mismatch",authentication.ErrProofInvalid,nil,nil,401},
  {"Drive unavailable",authentication.ErrAuthUnavailable,nil,nil,503},
  {"forbidden",nil,authorization.ErrForbidden,nil,403},
  {"storage unavailable",nil,nil,errors.New("postgres down"),503},
 }{
  t.Run(tc.name,func(t *testing.T){
   w:=request(t,protocolHandler(tc.auth,tc.policy,tc.store),http.MethodGet,"https://chat.example.test/v1/conversations/"+testOwner+"/history","",nil)
   if w.Code!=tc.want{t.Fatalf("status=%d want=%d",w.Code,tc.want)}
   if strings.Contains(w.Body.String(),"postgres down"){t.Fatal("internal error leaked")}
  })
 }
}
func TestFC14WSHandlerWiringAndOriginValidation(t *testing.T){
 node,err:=centrifuge.New(centrifuge.Config{});if err!=nil{t.Fatal(err)}
 v,err:=authentication.NewValidator("/nonexistent/socket",time.Minute);if err!=nil{t.Fatal(err)}
 st:=protocolStore{};p:=protocolPolicy{}
 msg:=conversations.Service{Auth:protocolAuth{},Permissions:p,Store:st}
 assign:=assignments.Service{Auth:protocolAuth{},Store:st}
 live:=realtime.Service{Auth:v,Permissions:p,Assignments:st,Publisher:node}
 for _,bad:=range []struct{ws,origin string}{
  {"wss://bad.test/ws","https://chat.example.test"},
  {"wss://chat.example.test/ws?token=secret","https://chat.example.test"},
  {"wss://chat.example.test/ws","http://chat.example.test"},
  {"wss://chat.example.test/other","https://chat.example.test"},
  {"wss://chat.example.test/ws","https://chat.example.test/path"},
 }{
  if _,err:=NewHandler(msg,assign,live,node,bad.ws,bad.origin);err==nil{t.Fatal("accepted invalid origin")}
 }
 h,err:=NewHandler(msg,assign,live,node,"wss://chat.example.test/ws","https://chat.example.test")
 if err!=nil{t.Fatalf("WS binding: %v",err)}
 req:=httptest.NewRequest(http.MethodPost,"https://chat.example.test/ws",nil)
 w:=httptest.NewRecorder();h.ServeHTTP(w,req)
 if w.Code!=405{t.Fatalf("WS method accepted: %d",w.Code)}
}

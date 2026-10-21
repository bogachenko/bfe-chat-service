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
func TestFC13RetiredHTTPRoutes(t *testing.T) {
 t.Parallel()
 h:=protocolHandler(nil,nil,nil)
 for _,tc:=range []struct{name,method,path string}{
  {"v1 messages",http.MethodPost,"/v1/conversations/"+testOwner+"/messages"},
  {"v1 history",http.MethodGet,"/v1/conversations/"+testOwner+"/history"},
  {"v1 assignment",http.MethodPut,"/v1/conversations/"+testOwner+"/assignment"},
  {"v2 direct",http.MethodPost,"/v2/conversations/direct"},
  {"v2 history",http.MethodGet,"/v2/conversations/"+testOwner+"/history"},
  {"v2 group rename",http.MethodPut,"/v2/conversations/"+testOwner+"/name"},
  {"v2 root",http.MethodGet,"/v2/conversations"},
 }{
  t.Run(tc.name,func(t *testing.T){
   t.Parallel()
   w:=request(t,h,tc.method,"https://chat.example.test"+tc.path,"",nil)
   if w.Code!=http.StatusNotFound{t.Fatalf("retired path returned %d: %s",w.Code,w.Body.String())}
   if !strings.Contains(w.Body.String(),"NOT_FOUND"){t.Fatalf("unexpected response %q",w.Body.String())}
  })
 }
}
func TestFC13RetiredPathsFailBeforeLegacyDependencies(t *testing.T){
 t.Parallel()
 for _,err:=range []error{authentication.ErrAuthInvalid,authentication.ErrProofInvalid,authentication.ErrAuthUnavailable}{
  h:=protocolHandler(err,authorization.ErrForbidden,errors.New("postgres down"))
  w:=request(t,h,http.MethodGet,"https://chat.example.test/v1/conversations/"+testOwner+"/history","",nil)
  if w.Code!=http.StatusNotFound{t.Fatalf("retired route reached old dependency %v: %d",err,w.Code)}
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

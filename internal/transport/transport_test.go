package transport

import (
 "context"
 "errors"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "time"

 "github.com/bogachenko/bfe-chat-service/internal/authentication"
 "github.com/centrifugal/centrifuge"
)

type issue32EpochAuthorizer struct { calls int }
func (e *issue32EpochAuthorizer) AuthorizeEpochChannel(_ context.Context, channel,user string) error {
 e.calls++
 if channel==""||user=="" {return errors.New("denied")}
 return nil
}

func issue32Validator(t *testing.T) *authentication.Validator {
 t.Helper()
 validator,err:=authentication.NewValidator("/nonexistent/drive-auth-issue32.sock",time.Minute)
 if err!=nil{t.Fatal(err)}
 return validator
}

func issue32Node(t *testing.T)*centrifuge.Node{
 t.Helper()
 node,err:=centrifuge.New(centrifuge.Config{})
 if err!=nil{t.Fatal(err)}
 return node
}

func TestIssue32NewCurrentHandlerRequiredDependencies(t *testing.T){
 cases:=[]struct{name string;missing string}{
  {"auth","auth"},{"epoch ACL","epochs"},{"centrifuge","node"},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){
  auth:=issue32Validator(t)
  epochs:=&issue32EpochAuthorizer{}
  node:=issue32Node(t)
  switch tc.missing{
  case "auth":auth=nil
  case "epochs":epochs=nil
  case "node":node=nil
  }
  // A typed nil interface is not a nil interface; check the truly absent ACL explicitly.
  if tc.missing=="epochs"{
   _,err:=NewCurrentHandler(auth,nil,node,"wss://chat.example.test/ws","https://chat.example.test")
   if err==nil{t.Fatal("missing epoch gate accepted")}
   return
  }
  if _,err:=NewCurrentHandler(auth,epochs,node,"wss://chat.example.test/ws","https://chat.example.test");err==nil{
   t.Fatal("missing production dependency accepted")
  }
 })}
}

func TestIssue32NewCurrentHandlerOriginValidation(t *testing.T){
 validator:=issue32Validator(t)
 cases:=[]struct{name,ws,origin string}{
  {"wrong WS host","wss://bad.test/ws","https://chat.example.test"},
  {"WS query credential","wss://chat.example.test/ws?token=secret","https://chat.example.test"},
  {"insecure public origin","wss://chat.example.test/ws","http://chat.example.test"},
  {"wrong WS path","wss://chat.example.test/other","https://chat.example.test"},
  {"origin has path","wss://chat.example.test/ws","https://chat.example.test/path"},
  {"WS userinfo","wss://alice@chat.example.test/ws","https://chat.example.test"},
  {"WS fragment","wss://chat.example.test/ws#fragment","https://chat.example.test"},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){
  if _,err:=NewCurrentHandler(validator,&issue32EpochAuthorizer{},issue32Node(t),tc.ws,tc.origin);err==nil{
   t.Fatalf("accepted unsafe origin %q endpoint %q",tc.origin,tc.ws)
  }
 })}
}

func TestIssue32NewCurrentHandlerWebSocketAndRetiredRoutes(t *testing.T){
 h,err:=NewCurrentHandler(issue32Validator(t),&issue32EpochAuthorizer{},issue32Node(t),"wss://chat.example.test/ws","https://chat.example.test/")
 if err!=nil{t.Fatal(err)}
 cases:=[]struct{name,method,path string;want int}{
  {"WS wrong method",http.MethodPost,"/ws",http.StatusMethodNotAllowed},
  {"WS query credentials",http.MethodGet,"/ws?token=secret",http.StatusBadRequest},
  {"retired v1 history",http.MethodGet,"/v1/conversations/11111111-1111-4111-8111-111111111111/history",http.StatusNotFound},
  {"retired v1 assignment",http.MethodPut,"/v1/conversations/11111111-1111-4111-8111-111111111111/assignment",http.StatusNotFound},
  {"retired v2 direct",http.MethodPost,"/v2/conversations/direct",http.StatusNotFound},
  {"retired v2 group rename",http.MethodPut,"/v2/conversations/11111111-1111-4111-8111-111111111111/name",http.StatusNotFound},
  {"unknown path",http.MethodGet,"/not-a-route",http.StatusNotFound},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){
  req:=httptest.NewRequest(tc.method,tc.path,nil)
  rec:=httptest.NewRecorder()
  h.ServeHTTP(rec,req)
  if rec.Code!=tc.want{t.Fatalf("%s %s: status=%d want=%d body=%s",tc.method,tc.path,rec.Code,tc.want,rec.Body.String())}
  if rec.Header().Get("Cache-Control")!="no-store"{t.Fatal("missing no-store cache policy")}
  if rec.Header().Get("X-Content-Type-Options")!="nosniff"{t.Fatal("missing nosniff policy")}
  if strings.Contains(rec.Body.String(),"secret"){t.Fatal("query credential reflected in response")}
  if tc.name=="WS wrong method"&&rec.Header().Get("Allow")!=http.MethodGet{t.Fatal("missing GET allow header")}
 })}
}

func TestIssue32CurrentHandlerSocketFailureClosed(t *testing.T){
 h:=Handler{PublicOrigin:"https://chat.example.test"}
 req:=httptest.NewRequest(http.MethodGet,"/ws",nil)
 rec:=httptest.NewRecorder()
 h.ServeHTTP(rec,req)
 if rec.Code!=http.StatusServiceUnavailable{t.Fatalf("socket without transport returned %d",rec.Code)}
}

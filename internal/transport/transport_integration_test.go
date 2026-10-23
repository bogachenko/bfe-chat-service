package transport

import (
 "net/http"
 "net/http/httptest"
 "testing"
)

// The current WebSocket entrypoint is intentionally independent of retired
// /v1 conversation, assignment, and storage services.
func TestIssue32WebSocketWiringDoesNotRequireLegacyServices(t *testing.T){
 auth:=issue32Validator(t)
 epochs:=&issue32EpochAuthorizer{}
 h,err:=NewCurrentHandler(auth,epochs,issue32Node(t),"wss://chat.example.test/ws","https://chat.example.test")
 if err!=nil{t.Fatal(err)}
 for _,path:=range []string{"/v1/conversations/x/messages","/v1/conversations/x/assignment","/v2/conversations/direct"}{
  t.Run(path,func(t *testing.T){
   req:=httptest.NewRequest(http.MethodGet,path,nil)
   rec:=httptest.NewRecorder()
   h.ServeHTTP(rec,req)
   if rec.Code!=http.StatusNotFound{t.Fatalf("%s: got %d want 404",path,rec.Code)}
   if epochs.calls!=0{t.Fatalf("obsolete route reached epoch ACL (%d calls)",epochs.calls)}
  })
 }
}

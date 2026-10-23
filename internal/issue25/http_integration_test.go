package issue25

import (
 "context"
 "encoding/json"
 "fmt"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"

 "github.com/bogachenko/bfe-chat-service/internal/authentication"
)

type fakeCaller struct{ id string }
func (f fakeCaller) ValidateCaller(_ context.Context,token,proof string,_ authentication.RequestBinding)(authentication.Caller,error){
 if token!="test-token"||proof!="test-proof"{return authentication.Caller{},authentication.ErrProofInvalid}
 return authentication.Caller{AccountID:f.id},nil
}
func TestHTTPv2DirectAndIdempotency(t *testing.T){
 db:=integrationDB(t)
 alice,bob:=fixtureID(t),fixtureID(t)
 h:=HTTPHandler{Store:Store{DB:db},Auth:fakeCaller{id:alice},PublicOrigin:"https://example.invalid"}
 request:=func(method,path,body string)*httptest.ResponseRecorder{
  t.Helper()
  r:=httptest.NewRequest(method,path,strings.NewReader(body))
  r.Header.Set("Authorization","DPoP test-token");r.Header.Set("DPoP","test-proof")
  w:=httptest.NewRecorder()
  h.ServeHTTP(w,r)
  return w
 }
 directBody:=fmt.Sprintf(`{"peerId":"%s"}`,bob)
 w:=request("POST","/conversations/direct",directBody)
 qaRejectf(t, (w.Code!=200), "direct status=%d body=%s",w.Code,w.Body.String())
 var d struct{ConversationID string `json:"conversationId"`}
 func() { err:=json.Unmarshal(w.Body.Bytes(),&d); qaRejectf(t, (err!=nil||d.ConversationID==""), "direct response: %v %s",err,w.Body.String()) }()
 w=request("POST","/conversations/direct",directBody)
 var replay struct{ConversationID string `json:"conversationId"`}
 _=json.Unmarshal(w.Body.Bytes(),&replay)
 qaRejectf(t, (w.Code!=200||replay.ConversationID!=d.ConversationID), "duplicate direct: %d %s",w.Code,w.Body.String())
 key:=fixtureID(t)
 msgPath:="/conversations/"+d.ConversationID+"/messages"
 body:=fmt.Sprintf(`{"idempotencyKey":"%s","text":"abc"}`,key)
 w=request("POST",msgPath,body)
 qaRejectf(t, (w.Code!=200), "send=%d %s",w.Code,w.Body.String())
 w=request("POST",msgPath,body)
 qaRejectf(t, (w.Code!=200), "replay=%d %s",w.Code,w.Body.String())
 w=request("POST",msgPath,fmt.Sprintf(`{"idempotencyKey":"%s","text":"changed"}`,key))
 qaRejectf(t, (w.Code!=409), "conflict=%d %s",w.Code,w.Body.String())
 w=request("POST",msgPath,strings.Repeat("a",65537))
 qaRejectf(t, (w.Code!=400), "oversize=%d %s",w.Code,w.Body.String())
 w=request("GET","/conversations/"+d.ConversationID+"/history","")
 qaRejectf(t, (w.Code!=200), "history=%d %s",w.Code,w.Body.String())
}
func TestHTTPv2AuthRequired(t *testing.T){
 h:=HTTPHandler{}
 w:=httptest.NewRecorder()
 r:=httptest.NewRequest(http.MethodPost,"/conversations/direct",strings.NewReader("{}"))
 h.ServeHTTP(w,r)
 qaRejectf(t, (w.Code!=503), "missing dependency status=%d",w.Code)
}

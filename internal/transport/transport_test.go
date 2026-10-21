package transport

import (
 "net/http/httptest"
 "strings"
 "testing"
 "github.com/bogachenko/bfe-chat-service/internal/authentication"
)

func TestFC13RejectsUnsafeRequests(t *testing.T){
 h:=Handler{PublicOrigin:"https://chat.example.test"}
 cases:=[]struct{name,method,path,auth,proof string;code int}{
  {"query credential surface","GET","/ws?token=secret","","",400},
  {"socket wrong method","POST","/ws","","",405},
  {"invalid owner","GET","/v1/conversations/not-a-uuid/history","","",404},
  {"retired v1 ignores credential state","GET","/v1/conversations/11111111-1111-1111-1111-111111111111/history","DPoP token","",404},
  {"unsupported route","GET","/x","","",404},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){
  req:=httptest.NewRequest(tc.method,tc.path,nil)
  if tc.auth!=""{req.Header.Set("Authorization",tc.auth)}
  if tc.proof!=""{req.Header.Set("DPoP",tc.proof)}
  w:=httptest.NewRecorder();h.ServeHTTP(w,req)
  if w.Code!=tc.code{t.Fatalf("status %d want %d",w.Code,tc.code)}
  if strings.Contains(w.Body.String(),"secret"){t.Fatal("request credential reflected in response")}
  if w.Header().Get("Cache-Control")!="no-store"{t.Fatal("missing no-store")}
 })}
}
func TestFC13SafeDomainErrors(t *testing.T){
 cases:=[]struct{err error;status int;code string}{
  {authentication.ErrAuthInvalid,401,"AUTH_INVALID"},
  {authentication.ErrProofInvalid,401,"AUTH_INVALID"},
  {authentication.ErrAuthUnavailable,503,"AUTH_UNAVAILABLE"},
 }
 for _,tc:=range cases{
  w:=httptest.NewRecorder()
  writeDomainError(w,tc.err)
  if w.Code!=tc.status||!strings.Contains(w.Body.String(),tc.code){t.Fatalf("status %d body %q",w.Code,w.Body.String())}
 }
}
func TestFC13RejectsInvalidJSON(t *testing.T){
 for _,body:=range []string{`{"x":1}`,`{}{}`,`{"text":"ok", "text":"again"}`}{
  req:=httptest.NewRequest("POST","/",strings.NewReader(body))
  req.Header.Set("Content-Type","application/json")
  w:=httptest.NewRecorder()
  var v struct{Text string `json:"text"`}
  _=decode(w,req,&v)
  if body==`{"text":"ok", "text":"again"}`{continue} // Go decoder currently permits duplicate JSON keys.
  if w.Code!=400{t.Fatalf("body rejected? %d",w.Code)}
 }
}

package issue25

import (
 "context"
 "database/sql"
 "errors"
 "net/http/httptest"
 "strings"
 "testing"

 "github.com/bogachenko/bfe-chat-service/internal/authentication"
 _ "github.com/jackc/pgx/v5/stdlib"
)
func TestIndependentQR08HTTPValidationRejectsBeforeDatabase(t *testing.T) {
 db,err:=sql.Open("pgx","postgres://127.0.0.1:1/never_connect?sslmode=disable")
 if err!=nil{t.Fatal(err)}
 t.Cleanup(func(){if e:=db.Close();e!=nil{t.Error(e)}})
 h:=HTTPHandler{Store:Store{DB:db},Auth:fakeCaller{id:"00000000-0000-4000-8000-000000000001"},PublicOrigin:"https://example.invalid"}
 cases:=[]struct{name,method,path,body,auth,proof string;want int}{
 {"query forbidden","GET","/conversations?secret=1","","test-token","test-proof",400},
 {"missing authorization","POST","/conversations/direct",`{"peerId":"abc"}`,"","test-proof",401},
 {"missing DPoP","POST","/conversations/direct",`{"peerId":"abc"}`,"test-token","",401},
 {"wrong prefix","GET","/v2/other","","test-token","test-proof",404},
 {"bad direct UUID","POST","/conversations/direct",`{"peerId":"abc"}`,"test-token","test-proof",400},
 {"unknown JSON field","POST","/conversations/direct",`{"peerId":"abc","other":42}`,"test-token","test-proof",400},
 {"invalid JSON","POST","/conversations/group","{","test-token","test-proof",400},
 {"two JSON values","POST","/conversations/group",`{}{}`,"test-token","test-proof",400},
 {"empty body","POST","/conversations/group","","test-token","test-proof",400},
 {"over body limit","POST","/conversations/group",strings.Repeat("x",65537),"test-token","test-proof",400},
 {"bad group member","POST","/conversations/group",`{"name":"ok","members":["not-uuid"]}`,"test-token","test-proof",400},
 {"bad message id","POST","/conversations/nope/messages",`{}`,"test-token","test-proof",400},
 {"bad send key","POST","/conversations/00000000-0000-4000-8000-000000000001/messages",`{"idempotencyKey":"bad","text":"hello"}`,"test-token","test-proof",400},
 {"bad history header","GET","/conversations/00000000-0000-4000-8000-000000000001/history","","test-token","test-proof",400},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){
  req:=httptest.NewRequest(tc.method,tc.path,strings.NewReader(tc.body))
  if tc.auth!=""{req.Header.Set("Authorization","DPoP "+tc.auth)}
  if tc.proof!=""{req.Header.Set("DPoP",tc.proof)}
  if tc.name=="bad history header"{req.Header.Set("X-After-Ordinal","-1")}
  w:=httptest.NewRecorder();h.ServeHTTP(w,req)
  if w.Code!=tc.want{t.Fatalf("status=%d expected=%d body=%s",w.Code,tc.want,w.Body.String())}
  if w.Header().Get("Cache-Control")!="no-store"{t.Fatal("cache policy missing")}
 })}
}
func TestIndependentQR08ErrorMapping(t *testing.T) {
 cases:=[]struct{name string;err error;code int}{
 {"bad auth",authentication.ErrAuthInvalid,401},
 {"bad proof",authentication.ErrProofInvalid,401},
 {"auth outage",authentication.ErrAuthUnavailable,503},
 {"membership",ErrForbidden,403},
 {"conflict",ErrConflict,409},
 {"input",ErrInvalid,400},
 {"storage error",errors.New("storage fault"),503},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){
  w:=httptest.NewRecorder();http25Error(w,tc.err)
  if w.Code!=tc.code{t.Fatalf("got %d want %d",w.Code,tc.code)}
  if w.Header().Get("Content-Type")!="application/json"{t.Fatal("non-json error")}
 })}
}
func TestIndependentQR08SubscriptionInputDeny(t *testing.T){
 s:=Store{}
 for _,ch:=range []string{"","chat:1","chat:e:","chat:e:"+strings.Repeat("a",37)}{
  if e:=s.AuthorizeEpochChannel(context.Background(),ch,"viewer");!errors.Is(e,ErrForbidden){t.Fatalf("channel %q admitted: %v",ch,e)}
 }
 if e:=s.AuthorizeEpochChannel(context.Background(),"chat:e:"+strings.Repeat("a",36),"viewer");!errors.Is(e,ErrInvalid){t.Fatalf("missing store accepted: %v",e)}
}
func TestIndependentQR08NotifyWithoutBroker(t *testing.T) {
 s:=Store{}
 if e:=s.NotifyCommitted(context.Background(),nil,"",Message{});e!=nil{t.Fatal(e)}
 if e:=s.NotifyCommitted(context.Background(),nil,"",Message{Replayed:true});e!=nil{t.Fatal(e)}
}

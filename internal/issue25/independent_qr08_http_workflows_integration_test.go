package issue25

import (
 "context"
 "encoding/json"
 "fmt"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
)

func TestIndependentQR08HTTPConversationWorkflow(t *testing.T) {
 db:=integrationDB(t)
 owner,member,outsider:=fixtureID(t),fixtureID(t),fixtureID(t)
 s:=Store{DB:db}
 group,err:=s.CreateGroup(context.Background(),owner,"qa http workflow",[]string{member})
 if err!=nil{t.Fatal(err)}
 call:=func(user,method,route,body string,headers map[string]string)(int,[]byte){
  t.Helper()
  h:=HTTPHandler{Store:s,Auth:fakeCaller{id:user},PublicOrigin:"https://example.invalid"}
  req:=httptest.NewRequest(method,"/v2/conversations"+route,strings.NewReader(body))
  req.Header.Set("Authorization","Bearer test-token")
  req.Header.Set("DPoP","test-proof")
  for k,v:=range headers{req.Header.Set(k,v)}
  w:=httptest.NewRecorder();h.ServeHTTP(w,req)
  if w.Header().Get("Cache-Control")!="no-store"{t.Fatal("cacheable response")}
  return w.Code,w.Body.Bytes()
 }
 route:="/"+group
 for _,tc:=range []struct{name,user,method,path,body string;headers map[string]string;status int}{
  {"list member",member,"GET","","",nil,200},
  {"list nonmember",outsider,"GET","","",nil,200},
  {"send forbidden",outsider,"POST",route+"/messages",fmt.Sprintf(`{"idempotencyKey":"%s","text":"no"}`,fixtureID(t)),nil,403},
  {"history forbidden",outsider,"GET",route+"/history","",nil,403},
  {"unread forbidden",outsider,"GET",route+"/unread","",nil,403},
  {"invalid history after",member,"GET",route+"/history","",map[string]string{"X-After-Ordinal":"-1"},400},
  {"invalid history limit",member,"GET",route+"/history","",map[string]string{"X-Limit":"abc"},400},
  {"history limit too high",member,"GET",route+"/history","",map[string]string{"X-Limit":"101"},400},
  {"list wrong method",member,"DELETE","","",nil,405},
  {"route does not exist",member,"GET",route+"/nonexistent","",nil,404},
 }{
  t.Run(tc.name,func(t *testing.T){
   status,body:=call(tc.user,tc.method,tc.path,tc.body,tc.headers)
   if status!=tc.status{t.Fatalf("status=%d want=%d body=%s",status,tc.status,body)}
  })
 }
 key:=fixtureID(t)
 send:=fmt.Sprintf(`{"idempotencyKey":"%s","text":"hello"}`,key)
 status,raw:=call(owner,"POST",route+"/messages",send,nil)
 if status!=200{t.Fatalf("send=%d %s",status,raw)}
 var first Message
 if e:=json.Unmarshal(raw,&first);e!=nil||first.Ordinal!=1{t.Fatalf("send result: %+v %v",first,e)}
 status,raw=call(owner,"POST",route+"/messages",send,nil)
 var replay Message
 if e:=json.Unmarshal(raw,&replay);e!=nil||status!=200||!replay.Replayed||replay.ID!=first.ID{t.Fatalf("idempotency: %+v %d %v",replay,status,e)}
 status,raw=call(owner,"POST",route+"/messages",fmt.Sprintf(`{"idempotencyKey":"%s","text":"changed"}`,key),nil)
 if status!=409{t.Fatalf("changed replay=%d %s",status,raw)}
 status,raw=call(member,"GET",route+"/history","",map[string]string{"X-Limit":"1","X-After-Ordinal":"0"})
 if status!=200{t.Fatalf("history=%d %s",status,raw)}
 var history []HistoryItem
 if e:=json.Unmarshal(raw,&history);e!=nil||len(history)!=1||history[0].Ordinal!=1||history[0].Text!="hello"{t.Fatalf("history: %+v %v",history,e)}
 status,raw=call(member,"GET",route+"/unread","",nil)
 if status!=200{t.Fatalf("unread=%d %s",status,raw)}
 var unread struct{Unread int64 `json:"unread"`}
 if e:=json.Unmarshal(raw,&unread);e!=nil||unread.Unread!=1{t.Fatalf("unread=%d %v",unread.Unread,e)}
 status,raw=call(member,"PUT",route+"/read",`{"ordinal":1}`,nil)
 if status!=200{t.Fatalf("read=%d %s",status,raw)}
 status,raw=call(member,"GET",route+"/unread","",nil)
 if e:=json.Unmarshal(raw,&unread);e!=nil||status!=200||unread.Unread!=0{t.Fatalf("after read=%d %v",unread.Unread,e)}
 status,raw=call(owner,http.MethodPut,route+"/hidden",`{"hidden":true}`,nil)
 if status!=400{t.Fatalf("group hide must fail validation: %d %s",status,raw)}
}

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
 qaReject(t, (err!=nil), err)
 call:=func(user,method,route,body string,headers map[string]string)(int,[]byte){
  t.Helper()
  h:=HTTPHandler{Store:s,Auth:fakeCaller{id:user},PublicOrigin:"https://example.invalid"}
  req:=httptest.NewRequest(method,"/v2/conversations"+route,strings.NewReader(body))
  req.Header.Set("Authorization","Bearer test-token")
  req.Header.Set("DPoP","test-proof")
  for k,v:=range headers{req.Header.Set(k,v)}
  w:=httptest.NewRecorder();h.ServeHTTP(w,req)
  qaReject(t, (w.Header().Get("Cache-Control")!="no-store"), "cacheable response")
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
   qaRejectf(t, (status!=tc.status), "status=%d want=%d body=%s",status,tc.status,body)
  })
 }
 key:=fixtureID(t)
 send:=fmt.Sprintf(`{"idempotencyKey":"%s","text":"hello"}`,key)
 status,raw:=call(owner,"POST",route+"/messages",send,nil)
 qaRejectf(t, (status!=200), "send=%d %s",status,raw)
 var first Message
 func() { e:=json.Unmarshal(raw,&first); qaRejectf(t, (e!=nil||first.Ordinal!=1), "send result: %+v %v",first,e) }()
 status,raw=call(owner,"POST",route+"/messages",send,nil)
 var replay Message
 func() { e:=json.Unmarshal(raw,&replay); qaRejectf(t, (e!=nil||status!=200||!replay.Replayed||replay.ID!=first.ID), "idempotency: %+v %d %v",replay,status,e) }()
 status,raw=call(owner,"POST",route+"/messages",fmt.Sprintf(`{"idempotencyKey":"%s","text":"changed"}`,key),nil)
 qaRejectf(t, (status!=409), "changed replay=%d %s",status,raw)
 status,raw=call(member,"GET",route+"/history","",map[string]string{"X-Limit":"1","X-After-Ordinal":"0"})
 qaRejectf(t, (status!=200), "history=%d %s",status,raw)
 var history []HistoryItem
 func() { e:=json.Unmarshal(raw,&history); qaRejectf(t, (e!=nil||len(history)!=1||history[0].Ordinal!=1||history[0].Text!="hello"), "history: %+v %v",history,e) }()
 status,raw=call(member,"GET",route+"/unread","",nil)
 qaRejectf(t, (status!=200), "unread=%d %s",status,raw)
 var unread struct{Unread int64 `json:"unread"`}
 func() { e:=json.Unmarshal(raw,&unread); qaRejectf(t, (e!=nil||unread.Unread!=1), "unread=%d %v",unread.Unread,e) }()
 status,raw=call(member,"PUT",route+"/read",`{"ordinal":1}`,nil)
 qaRejectf(t, (status!=200), "read=%d %s",status,raw)
 status,raw=call(member,"GET",route+"/unread","",nil)
 func() { e:=json.Unmarshal(raw,&unread); qaRejectf(t, (e!=nil||status!=200||unread.Unread!=0), "after read=%d %v",unread.Unread,e) }()
 status,raw=call(owner,http.MethodPut,route+"/hidden",`{"hidden":true}`,nil)
 qaRejectf(t, (status!=400), "group hide must fail validation: %d %s",status,raw)
}

package realtime_test

import (
 "bufio"
 "context"
 "crypto/rand"
 "encoding/base64"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net"
 "net/http"
 "net/http/httptest"
 "strings"
 "sync"
 "testing"
 "time"

 "github.com/bogachenko/bfe-chat-service/internal/authentication"
 "github.com/bogachenko/bfe-chat-service/internal/authorization"
 "github.com/bogachenko/bfe-chat-service/internal/chat"
 "github.com/bogachenko/bfe-chat-service/internal/realtime"
 "github.com/centrifugal/centrifuge"
)

type subscriptionPolicy struct {
 mu sync.Mutex
 recipient chat.AccountID
 calls int
}
func (p *subscriptionPolicy) AuthorizeOperation(_ context.Context, caller authentication.Caller, target chat.AccountID, action authorization.Operation) error {
 p.mu.Lock()
 defer p.mu.Unlock()
 p.calls++
 if action!=authorization.Subscribe || target!=owner {return authorization.ErrForbidden}
 if caller.AccountID==string(owner)||caller.AccountID==string(p.recipient){return nil}
 return authorization.ErrForbidden
}
func (p *subscriptionPolicy) change(recipient chat.AccountID) {
 p.mu.Lock();defer p.mu.Unlock();p.recipient=recipient
}
func newRealtimeNode(t *testing.T, viewer chat.AccountID, policy *subscriptionPolicy) (string,*centrifuge.Node) {
 t.Helper()
 v,err:=authentication.NewValidator("/nonexistent-bfe-auth.sock",time.Minute)
 if err!=nil {t.Fatal(err)}
 node,err:=centrifuge.New(centrifuge.Config{})
 if err!=nil {t.Fatal(err)}
 svc:=realtime.Service{Auth:v,Permissions:policy,Assignments:&assignmentStore{recipient:string(first)},Publisher:&publisher{}}
 if err:=svc.Bind(node,"wss://chat.example.test/ws");err!=nil {t.Fatalf("Bind: %v",err)}
 // Authentication has independent real DPoP tests in DP-M01. This callback
 // provides test credentials so the FC-08 subscription boundary can be exercised.
 node.OnConnecting(func(_ context.Context,_ centrifuge.ConnectEvent)(centrifuge.ConnectReply,error){
  return centrifuge.ConnectReply{Credentials:&centrifuge.Credentials{UserID:string(viewer)}},nil
 })
 if err:=node.Run();err!=nil{t.Fatal(err)}
 t.Cleanup(func(){ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();_ =node.Shutdown(ctx)})
 server:=httptest.NewServer(centrifuge.NewWebsocketHandler(node,centrifuge.WebsocketConfig{}))
 t.Cleanup(server.Close)
 return strings.TrimPrefix(server.URL,"http://"),node
}
func realtimeSocket(t *testing.T,address string) (net.Conn,*bufio.Reader) {
 t.Helper()
 c,err:=net.DialTimeout("tcp",address,5*time.Second)
 if err!=nil {t.Fatal(err)}
 t.Cleanup(func(){_ =c.Close()})
 key:=make([]byte,16)
 if _,err=rand.Read(key);err!=nil{t.Fatal(err)}
 request:=fmt.Sprintf("GET /ws HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: %s\r\n\r\n",address,base64.StdEncoding.EncodeToString(key))
 if _,err=io.WriteString(c,request);err!=nil{t.Fatal(err)}
 reader:=bufio.NewReader(c)
 res,err:=http.ReadResponse(reader,&http.Request{Method:http.MethodGet})
 if err!=nil{t.Fatal(err)}
 if res.StatusCode!=http.StatusSwitchingProtocols{t.Fatalf("upgrade: %s",res.Status)}
 return c,reader
}
func socketCommand(t *testing.T,c net.Conn,reader *bufio.Reader,id int,command string,data map[string]any) map[string]json.RawMessage {
 t.Helper()
 raw,err:=json.Marshal(map[string]any{"id":id,command:data})
 if err!=nil{t.Fatal(err)}
 sendRealtimeFrame(t,c,append(raw,'\n'))
 body:=readRealtimeFrame(t,c,reader)
 var reply map[string]json.RawMessage
 if err=json.Unmarshal(body,&reply);err!=nil{t.Fatalf("WS reply %q: %v",body,err)}
 return reply
}
func sendRealtimeFrame(t *testing.T,c net.Conn,raw []byte) {
 t.Helper()
 if len(raw)>=126{t.Fatalf("command too long: %d",len(raw))}
 mask:=make([]byte,4)
 if _,err:=rand.Read(mask);err!=nil{t.Fatal(err)}
 frame:=[]byte{0x81,0x80|byte(len(raw))}
 frame=append(frame,mask...)
 for i,v:=range raw{frame=append(frame,v^mask[i%4])}
 if err:=c.SetWriteDeadline(time.Now().Add(5*time.Second));err!=nil{t.Fatal(err)}
 if _,err:=c.Write(frame);err!=nil{t.Fatal(err)}
}
func readRealtimeFrame(t *testing.T,c net.Conn,reader *bufio.Reader) []byte {
 t.Helper()
 if err:=c.SetReadDeadline(time.Now().Add(5*time.Second));err!=nil{t.Fatal(err)}
 header:=make([]byte,2)
 if _,err:=io.ReadFull(reader,header);err!=nil{t.Fatal(err)}
 n:=int(header[1]&0x7f)
 if n==126{extended:=make([]byte,2);if _,err:=io.ReadFull(reader,extended);err!=nil{t.Fatal(err)};n=int(extended[0])<<8|int(extended[1])}
 if n==127{t.Fatal("unexpected large WS frame")}
 body:=make([]byte,n)
 if _,err:=io.ReadFull(reader,body);err!=nil{t.Fatal(err)}
 return body
}
func TestFC08SubscriptionAuthorizationAndReassignment(t *testing.T) {
 policy:=&subscriptionPolicy{recipient:first}
 address,_:=newRealtimeNode(t,first,policy)
 conn,reader:=realtimeSocket(t,address)
 if reply:=socketCommand(t,conn,reader,1,"connect",map[string]any{});len(reply["connect"])==0{t.Fatalf("connection refused: %v",reply)}
 valid:=realtime.Channel(owner,first)
 if reply:=socketCommand(t,conn,reader,2,"subscribe",map[string]any{"channel":valid});len(reply["subscribe"])==0{t.Fatalf("authorized subscribe refused: %v",reply)}
 policy.change(second)
 if reply:=socketCommand(t,conn,reader,3,"subscribe",map[string]any{"channel":valid});len(reply["error"])==0{t.Fatalf("former recipient subscribe accepted: %v",reply)}
 if reply:=socketCommand(t,conn,reader,4,"subscribe",map[string]any{"channel":realtime.Channel(owner,second)});len(reply["error"])==0{t.Fatalf("foreign viewer channel accepted: %v",reply)}
 if reply:=socketCommand(t,conn,reader,5,"subscribe",map[string]any{"channel":"chat:invalid"});len(reply["error"])==0{t.Fatalf("malformed channel accepted: %v",reply)}
}
func TestFC08OwnerAndRecipientChannelIsolation(t *testing.T) {
 for _,tc:=range []struct{name string;viewer chat.AccountID;channel string;allowed bool}{
  {"owner own channel",owner,realtime.Channel(owner,owner),true},
  {"current recipient",first,realtime.Channel(owner,first),true},
  {"other recipient",second,realtime.Channel(owner,second),false},
  {"owner cannot impersonate recipient",owner,realtime.Channel(owner,first),false},
 }{
  t.Run(tc.name,func(t *testing.T){
   policy:=&subscriptionPolicy{recipient:first}
   address,_:=newRealtimeNode(t,tc.viewer,policy)
   conn,reader:=realtimeSocket(t,address)
   if reply:=socketCommand(t,conn,reader,1,"connect",map[string]any{});len(reply["connect"])==0{t.Fatalf("connection rejected: %v",reply)}
   reply:=socketCommand(t,conn,reader,2,"subscribe",map[string]any{"channel":tc.channel})
   if accepted:=len(reply["subscribe"])>0;accepted!=tc.allowed{t.Fatalf("accepted=%v want %v response=%v",accepted,tc.allowed,reply)}
  })
 }
}

func TestMSF02ActiveWebSocketRevocationStopsPublication(t *testing.T) {
 policy:=&subscriptionPolicy{recipient:first}
 address,node:=newRealtimeNode(t,first,policy)
 conn,reader:=realtimeSocket(t,address)
 if reply:=socketCommand(t,conn,reader,1,"connect",map[string]any{});len(reply["connect"])==0{t.Fatalf("connection rejected: %v",reply)}
 channel:=realtime.Channel(owner,first)
 if reply:=socketCommand(t,conn,reader,2,"subscribe",map[string]any{"channel":channel});len(reply["subscribe"])==0{t.Fatalf("subscription rejected: %v",reply)}
 policy.change(second)
 // An already subscribed socket must not receive data after revocation.
 // This deliberately bypasses the FC-09 publisher to test FC-08's active
 // subscription lifecycle, not merely admission for new subscriptions.
 if _,err:=node.Publish(channel,[]byte(`{"revoked":true}`));err!=nil{t.Fatalf("server publish: %v",err)}
 if err:=conn.SetReadDeadline(time.Now().Add(500*time.Millisecond));err!=nil{t.Fatal(err)}
 frame:=make([]byte,2)
 _,err:=io.ReadFull(reader,frame)
 if err==nil{t.Fatalf("revoked active WebSocket received a publication frame (opcode %d)",frame[0]&0x0f)}
 var netErr net.Error
 if !errors.As(err,&netErr)||!netErr.Timeout(){t.Fatalf("unexpected socket outcome after revocation: %v",err)}
}

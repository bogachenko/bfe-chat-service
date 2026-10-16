package authentication_test

import (
 "bufio"
 "bytes"
 "context"
 "crypto/rand"
 "encoding/base64"
 "encoding/binary"
 "encoding/json"
 "fmt"
 "io"
 "net"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "time"

 "github.com/centrifugal/centrifuge"
)

func dialWebSocket(t *testing.T,address string)net.Conn{
 t.Helper()
 conn,err:=net.DialTimeout("tcp",address,5*time.Second);if err!=nil{t.Fatal(err)}
 t.Cleanup(func(){_ =conn.Close()})
 key:=make([]byte,16);if _,err:=rand.Read(key);err!=nil{t.Fatal(err)}
 req:=fmt.Sprintf("GET /ws HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: %s\r\n\r\n",address,base64.StdEncoding.EncodeToString(key))
 if _,err:=io.WriteString(conn,req);err!=nil{t.Fatal(err)}
 response,err:=http.ReadResponse(bufio.NewReader(conn),&http.Request{Method:"GET"});if err!=nil{t.Fatal(err)}
 if response.StatusCode!=http.StatusSwitchingProtocols{t.Fatalf("websocket upgrade: %s",response.Status)}
 return conn
}
func sendWSJSON(t *testing.T,c net.Conn,body []byte){
 t.Helper()
 // RFC6455 requires masking on every client-to-server frame.
 mask:=make([]byte,4);if _,err:=rand.Read(mask);err!=nil{t.Fatal(err)}
 b:=[]byte{0x81}
 n:=len(body)
 switch{case n<126:b=append(b,0x80|byte(n));case n<=65535:b=append(b,0xfe,byte(n>>8),byte(n));default:t.Fatal("unexpected large connect frame")}
 b=append(b,mask...)
 for i,x:=range body{b=append(b,x^mask[i%4])}
 if err:=c.SetWriteDeadline(time.Now().Add(5*time.Second));err!=nil{t.Fatal(err)}
 if _,err:=c.Write(b);err!=nil{t.Fatal(err)}
}
func readWSReply(t *testing.T,c net.Conn)(byte,[]byte){
 t.Helper()
 if err:=c.SetReadDeadline(time.Now().Add(5*time.Second));err!=nil{t.Fatal(err)}
 var header [2]byte
 if _,err:=io.ReadFull(c,header[:]);err!=nil{t.Fatal(err)}
 n:=int(header[1]&0x7f)
 if n==126{var l [2]byte;if _,err:=io.ReadFull(c,l[:]);err!=nil{t.Fatal(err)};n=int(binary.BigEndian.Uint16(l[:]))}
 if n==127{t.Fatal("unexpected extended frame")}
 payload:=make([]byte,n)
 if _,err:=io.ReadFull(c,payload);err!=nil{t.Fatal(err)}
 return header[0]&0x0f,payload
}
func TestFC02RealCentrifugeWebSocketHandshakeAndReconnect(t *testing.T){
 f:=newFixture(t)
 node,err:=centrifuge.New(centrifuge.Config{})
 if err!=nil{t.Fatal(err)}
 f.validator.BindOnConnecting(node,"wss://chat.example.test/connection")
 connected:=make(chan string,4)
 node.OnConnect(func(client *centrifuge.Client){connected<-client.UserID()})
 if err:=node.Run();err!=nil{t.Fatal(err)}
 t.Cleanup(func(){ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();_ =node.Shutdown(ctx)})
 server:=httptest.NewServer(centrifuge.NewWebsocketHandler(node,centrifuge.WebsocketConfig{}))
 t.Cleanup(server.Close)
 address:=strings.TrimPrefix(server.URL,"http://")
 connect:=func(jws string,payloadSize int) (byte,[]byte){
  c:=dialWebSocket(t,address)
  dataFields:=map[string]string{"dpop":jws}
  if payloadSize>0{
   dataFields["padding"]=""
   initial,marshalErr:=json.Marshal(dataFields);if marshalErr!=nil{t.Fatal(marshalErr)}
   if len(initial)>payloadSize{t.Fatal("requested payload too small")}
   dataFields["padding"]=strings.Repeat("x",payloadSize-len(initial))
  }
  data,err:=json.Marshal(map[string]any{"id":1,"connect":map[string]any{"token":f.token,"data":dataFields}})
  if err!=nil{t.Fatal(err)}
  sendWSJSON(t,c,append(data,'\n'))
  return readWSReply(t,c)
 }
 first:=f.valid(t,"real-handshake")
 opcode,reply:=connect(first,0)
 if opcode!=1||!bytes.Contains(reply,[]byte(`"connect"`)){t.Fatalf("first connect opcode=%d body=%s",opcode,reply)}
 select{case account:=<-connected:if account!=fakeAccount{t.Fatalf("wrong account %s",account)};case <-time.After(3*time.Second):t.Fatal("OnConnect not called")}
 // A reconnect is a new connection; the old proof must not authorize it.
 opcode,reply=connect(first,0)
 if opcode==1&&bytes.Contains(reply,[]byte(`"connect"`)){t.Fatalf("reused proof accepted: %s",reply)}
 fresh:=f.valid(t,"real-handshake-new")
 opcode,reply=connect(fresh,0)
 if opcode!=1||!bytes.Contains(reply,[]byte(`"connect"`)){t.Fatalf("fresh reconnect rejected opcode=%d body=%s",opcode,reply)}
 select{case account:=<-connected:if account!=fakeAccount{t.Fatalf("wrong reconnect identity %s",account)};case <-time.After(3*time.Second):t.Fatal("reconnect OnConnect not called")}
 atLimit:=f.valid(t,"payload-exact-16kb")
 opcode,reply=connect(atLimit,16*1024)
 if opcode!=1||!bytes.Contains(reply,[]byte(`"connect"`)){t.Fatalf("exact maximum Connect payload rejected opcode=%d body=%s",opcode,reply)}
 select{case account:=<-connected:if account!=fakeAccount{t.Fatalf("wrong at-boundary identity %s",account)};case <-time.After(3*time.Second):t.Fatal("at-boundary OnConnect not called")}
 beyondLimit:=f.valid(t,"payload-over-16kb")
 opcode,reply=connect(beyondLimit,16*1024+1)
 if opcode==1&&bytes.Contains(reply,[]byte(`"connect"`)){t.Fatalf("oversized Connect payload accepted: %s",reply)}
}

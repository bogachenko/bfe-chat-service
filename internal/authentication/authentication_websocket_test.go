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
 frame:=maskedClientFrame(t,body)
 if err:=c.SetWriteDeadline(time.Now().Add(5*time.Second));err!=nil{t.Fatal(err)}
 if _,err:=c.Write(frame);err!=nil{t.Fatal(err)}
}
func maskedClientFrame(t *testing.T,body []byte)[]byte{
 t.Helper()
 mask:=make([]byte,4)
 if _,err:=rand.Read(mask);err!=nil{t.Fatal(err)}
 header:=websocketFrameHeader(t,len(body))
 header=append(header,mask...)
 for i,x:=range body{header=append(header,x^mask[i%4])}
 return header
}
func websocketFrameHeader(t *testing.T,n int)[]byte{
 t.Helper()
 if n<126{return []byte{0x81,0x80|byte(n)}}
 if n<=65535{return []byte{0x81,0xfe,byte(n>>8),byte(n)}}
 t.Fatal("unexpected large connect frame")
 return nil
}
func readWSReply(t *testing.T, c net.Conn) (byte, []byte) {
 t.Helper()
 if err:=c.SetReadDeadline(time.Now().Add(5*time.Second));err!=nil{t.Fatal(err)}
 var header [2]byte
 if _,err:=io.ReadFull(c,header[:]);err!=nil{t.Fatal(err)}
 n:=websocketPayloadLength(t,c,header[1]&0x7f)
 payload:=make([]byte,n)
 if _,err:=io.ReadFull(c,payload);err!=nil{t.Fatal(err)}
 return header[0]&0x0f,payload
}
func websocketPayloadLength(t *testing.T,c net.Conn,n byte) int{
 t.Helper()
 if n==127{t.Fatal("unexpected extended frame")}
 if n!=126{return int(n)}
 var length [2]byte
 if _,err:=io.ReadFull(c,length[:]);err!=nil{t.Fatal(err)}
 return int(binary.BigEndian.Uint16(length[:]))
}
func assertWebSocketConnect(t *testing.T,opcode byte,reply []byte,label string){
 t.Helper()
 if opcode!=1||!bytes.Contains(reply,[]byte(`"connect"`)){t.Fatalf("%s: opcode=%d body=%s",label,opcode,reply)}
}
func assertWebSocketReject(t *testing.T,opcode byte,reply []byte,label string){
 t.Helper()
 if opcode==1&&bytes.Contains(reply,[]byte(`"connect"`)){t.Fatalf("%s: rejected frame accepted: %s",label,reply)}
}
func assertConnectedAccount(t *testing.T,connected <-chan string,label string){
 t.Helper()
 select{
 case account:=<-connected:
  if account!=fakeAccount{t.Fatalf("%s: wrong account %s",label,account)}
 case <-time.After(3*time.Second):t.Fatalf("%s: OnConnect not called",label)
 }
}
func connectWSProof(t *testing.T,address,token,jws string,payloadSize int)(byte,[]byte){
 t.Helper()
 conn:=dialWebSocket(t,address)
 dataFields:=map[string]string{"dpop":jws}
 if payloadSize>0{dataFields=paddingFields(t,jws,payloadSize)}
 data,err:=json.Marshal(map[string]any{"id":1,"connect":map[string]any{"token":token,"data":dataFields}})
 if err!=nil{t.Fatal(err)}
 sendWSJSON(t,conn,append(data,'\n'))
 return readWSReply(t,conn)
}
func paddingFields(t *testing.T,jws string,payloadSize int)map[string]string{
 t.Helper()
 fields:=map[string]string{"dpop":jws,"padding":""}
 initial,err:=json.Marshal(fields);if err!=nil{t.Fatal(err)}
 if len(initial)>payloadSize{t.Fatal("requested payload too small")}
 fields["padding"]=strings.Repeat("x",payloadSize-len(initial))
 return fields
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
 connect:=func(jws string,payloadSize int)(byte,[]byte){
  return connectWSProof(t,address,f.token,jws,payloadSize)
 }
 first:=f.valid(t,"real-handshake")
 opcode,reply:=connect(first,0)
 assertWebSocketConnect(t,opcode,reply,"first connect")
 assertConnectedAccount(t,connected,"first connect")
 // A reconnect is a new connection; the old proof must not authorize it.
 opcode,reply=connect(first,0)
 assertWebSocketReject(t,opcode,reply,"replay")
 fresh:=f.valid(t,"real-handshake-new")
 opcode,reply=connect(fresh,0)
 assertWebSocketConnect(t,opcode,reply,"fresh reconnect")
 assertConnectedAccount(t,connected,"reconnect")
 atLimit:=f.valid(t,"payload-exact-16kb")
 opcode,reply=connect(atLimit,16*1024)
 assertWebSocketConnect(t,opcode,reply,"16KiB")
 assertConnectedAccount(t,connected,"16KiB")
 beyondLimit:=f.valid(t,"payload-over-16kb")
 opcode,reply=connect(beyondLimit,16*1024+1)
 assertWebSocketReject(t,opcode,reply,"oversized")
}

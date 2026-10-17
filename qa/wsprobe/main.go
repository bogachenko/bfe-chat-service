// Command wsprobe exercises real Centrifuge WebSocket authentication over BFE Drive UDS.
package main

import (
 "bufio"
 "context"
 "crypto"
 "crypto/ed25519"
 "crypto/rand"
 "crypto/sha256"
 "encoding/base64"
 "encoding/json"
 "errors"
 "flag"
 "fmt"
 "io"
 "net"
 "net/http"
 "net/http/httptest"
 "os"
 "strings"
 "time"

 "github.com/bogachenko/bfe-chat-service/internal/authentication"
 "github.com/bogachenko/bfe-chat-service/internal/authorization"
 "github.com/bogachenko/bfe-chat-service/internal/chat"
 "github.com/bogachenko/bfe-chat-service/internal/realtime"
 "github.com/centrifugal/centrifuge"
 jose "github.com/go-jose/go-jose/v4"
)

const endpoint="wss://chat.example.test/ws"
var owner chat.AccountID
type assignment struct{}
func (assignment) WithAssignmentLock(_ context.Context,_ string,action func(string)error)error{return action(string(owner))}
type policy struct{}
func (policy) AuthorizeOperation(_ context.Context, caller authentication.Caller,target chat.AccountID,action authorization.Operation)error{
 if action==authorization.Subscribe&&target==owner&&caller.AccountID==string(owner){return nil}
 return authorization.ErrForbidden
}
type publish struct{node *centrifuge.Node}
func(p publish)Publish(ch string,data []byte,opts ...centrifuge.PublishOption)(centrifuge.PublishResult,error){return p.node.Publish(ch,data,opts...)}

func main(){
 socket:=flag.String("socket","","BFE Drive UDS")
 account:=flag.String("account","","Fixture account UUID")
 expect:=flag.String("expect","ok","ok, invalid or unavailable")
 flag.Parse()
 if *socket=="" {exit(errors.New("missing socket"))}
 owner=chat.AccountID(*account)
 if !owner.Valid(){exit(errors.New("invalid fixture account"))}
 token:=os.Getenv("CHAT_WS_TOKEN")
 if token==""{exit(errors.New("missing fixture token"))}
 if *expect!="ok"&&*expect!="invalid"&&*expect!="unavailable"&&*expect!="lifecycle"{exit(errors.New("invalid expectation"))}
 v,err:=authentication.NewValidator(*socket,5*time.Minute);if err!=nil{exit(err)}
 node,err:=centrifuge.New(centrifuge.Config{});if err!=nil{exit(err)}
 svc:=realtime.Service{Auth:v,Permissions:policy{},Assignments:assignment{},Publisher:publish{node:node}}
 if err:=svc.Bind(node,endpoint);err!=nil{exit(err)}
 if err:=node.Run();err!=nil{exit(err)}
 defer func(){ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();_ =node.Shutdown(ctx)}()
 srv:=httptest.NewServer(centrifuge.NewWebsocketHandler(node,centrifuge.WebsocketConfig{}))
 defer srv.Close()
 if *expect=="lifecycle" {
  if err:=lifecycle(strings.TrimPrefix(srv.URL,"http://"),node,token);err!=nil{exit(err)}
 } else if err:=checkWebsocket(strings.TrimPrefix(srv.URL,"http://"),token,*expect);err!=nil{exit(err)}
 fmt.Println("real Centrifuge WebSocket + BFE Drive UDS: "+*expect+" PASS")
}
func checkWebsocket(addr,token,expect string)error{
 conn,err:=net.DialTimeout("tcp",addr,5*time.Second);if err!=nil{return err}
 defer conn.Close()
 key:=make([]byte,16);if _,err=rand.Read(key);err!=nil{return err}
 req:=fmt.Sprintf("GET /ws HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: %s\r\n\r\n",addr,base64.StdEncoding.EncodeToString(key))
 if _,err=io.WriteString(conn,req);err!=nil{return err}
 rd:=bufio.NewReader(conn)
 response,err:=http.ReadResponse(rd,&http.Request{Method:http.MethodGet});if err!=nil{return err}
 defer response.Body.Close()
 if response.StatusCode!=http.StatusSwitchingProtocols{return fmt.Errorf("unexpected WebSocket upgrade status: %d",response.StatusCode)}
 proof,err:=signedProof(token);if err!=nil{return err}
 data,err:=json.Marshal(map[string]string{"dpop":proof});if err!=nil{return err}
 command,err:=json.Marshal(map[string]any{"id":1,"connect":map[string]any{"token":token,"data":json.RawMessage(data)}});if err!=nil{return err}
 if err:=sendFrame(conn,append(command,'\n'));err!=nil{return err}
 if expect!="ok"{return readExplicitAuthDenial(conn,rd)}
 raw,err:=receiveFrame(conn,rd)
 if err!=nil{return err}
 var reply map[string]json.RawMessage
 if err:=json.Unmarshal(raw,&reply);err!=nil{return errors.New("connect reply malformed")}
 if len(reply["connect"])==0{return errors.New("valid session was not accepted")}
 return nil
}
func sendFrame(c net.Conn,data []byte)error{
 if len(data)>65535{return errors.New("WebSocket command too large")}
 mask:=make([]byte,4);if _,err:=rand.Read(mask);err!=nil{return err}
 head:=[]byte{0x81,0xfe,byte(len(data)>>8),byte(len(data))}
 if len(data)<126{head=[]byte{0x81,0x80|byte(len(data))}}
 head=append(head,mask...)
 for i,v:=range data{data[i]=v^mask[i%4]}
 if err:=c.SetWriteDeadline(time.Now().Add(5*time.Second));err!=nil{return err}
 _,err:=c.Write(append(head,data...));return err
}
func receiveFrame(c net.Conn,rd *bufio.Reader)([]byte,error){
 if err:=c.SetReadDeadline(time.Now().Add(5*time.Second));err!=nil{return nil,err}
 header:=make([]byte,2);if _,err:=io.ReadFull(rd,header);err!=nil{return nil,err}
 n:=int(header[1]&0x7f)
 if n==126{size:=make([]byte,2);if _,err:=io.ReadFull(rd,size);err!=nil{return nil,err};n=int(size[0])<<8|int(size[1])}
 if n==127{return nil,errors.New("large WebSocket reply")}
 if header[0]&0x0f==0x8{return nil,errors.New("websocket closed without explicit authentication error frame")}
 data:=make([]byte,n);_,err:=io.ReadFull(rd,data);return data,err
}
func signedProof(token string)(string,error){
 seed:=sha256.Sum256([]byte("dp-m01-independent-fixture-private-key"))
 private:=ed25519.NewKeyFromSeed(seed[:])
 signer,err:=jose.NewSigner(jose.SigningKey{Algorithm:jose.EdDSA,Key:jose.JSONWebKey{Key:private,Algorithm:string(jose.EdDSA)}},&jose.SignerOptions{EmbedJWK:true,ExtraHeaders:map[jose.HeaderKey]interface{}{jose.HeaderType:"dpop+jwt"}})
 if err!=nil{return "",err}
 ath:=sha256.Sum256([]byte(token))
 claims,err:=json.Marshal(map[string]any{"jti":fmt.Sprintf("ws-%d",time.Now().UnixNano()),"iat":time.Now().Unix(),"htm":"GET","htu":"https://chat.example.test/ws","ath":base64.RawURLEncoding.EncodeToString(ath[:])})
 if err!=nil{return "",err}
 signed,err:=signer.Sign(claims);if err!=nil{return "",err}
 _,err=(&jose.JSONWebKey{Key:private.Public()}).Thumbprint(crypto.SHA256);if err!=nil{return "",err}
 return signed.CompactSerialize()
}
func exit(err error){
 // Never print wrapped network errors or protocol bodies: they may contain secrets.
 fmt.Fprintln(os.Stderr,"wsprobe: scenario failed (details suppressed)")
 _ = err
 os.Exit(1)
}

func lifecycle(addr string,node *centrifuge.Node,token string)error {
 conn,reader,err:=openSocket(addr);if err!=nil{return err}
 defer conn.Close()
 if err:=connectCommand(conn,reader,token,1,true);err!=nil{return err}
 if err:=subscribeCommand(conn,reader,2);err!=nil{return err}
 fmt.Println("READY")
 commands:=bufio.NewReader(os.Stdin)
 stage,err:=commands.ReadString('\n');if err!=nil{return errors.New("missing revoke stage")}
 if strings.TrimSpace(stage)!="REVOKED"{return errors.New("invalid revoke stage")}
 // Existing connection stays subscribed after logout. A synchronous publication
 // proves that it has not been forcibly disconnected on BFE Drive revocation.
 if _,err:=node.Publish(realtime.Channel(owner,owner),[]byte(`{"phase":"after-revoke"}`));err!=nil{return err}
 event,err:=receiveFrame(conn,reader);if err!=nil{return err}
 if !strings.Contains(string(event),"after-revoke"){return errors.New("open socket lost delivery after revocation")}
 fmt.Println("OPEN_AFTER_REVOKE")
 if err:=checkWebsocket(addr,token,"invalid");err!=nil{return err}
 fmt.Println("OLD_TOKEN_DENIED")
 stage,err=commands.ReadString('\n');if err!=nil{return errors.New("missing new session stage")}
 if strings.TrimSpace(stage)!="NEW"{return errors.New("invalid new session stage")}
 fresh,err:=commands.ReadString('\n');if err!=nil{return errors.New("missing new token")}
 fresh=strings.TrimSpace(fresh)
 if fresh==""||fresh==token{return errors.New("new session credential invalid")}
 next,rd,err:=openSocket(addr);if err!=nil{return err}
 defer next.Close()
 if err:=connectCommand(next,rd,fresh,3,true);err!=nil{return err}
 if err:=subscribeCommand(next,rd,4);err!=nil{return err}
 fmt.Println("NEW_SESSION_CONNECTED")
 if _,err:=node.Publish(realtime.Channel(owner,owner),[]byte(`{"phase":"new-session"}`));err!=nil{return err}
 event,err=receiveFrame(next,rd);if err!=nil{return err}
 if !strings.Contains(string(event),"new-session"){return errors.New("fresh connection did not receive message")}
 fmt.Println("NEW_SESSION_DELIVERED")
 return nil
}
func openSocket(addr string)(net.Conn,*bufio.Reader,error){
 conn,err:=net.DialTimeout("tcp",addr,5*time.Second);if err!=nil{return nil,nil,err}
 key:=make([]byte,16);if _,err:=rand.Read(key);err!=nil{_ =conn.Close();return nil,nil,err}
 req:=fmt.Sprintf("GET /ws HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: %s\r\n\r\n",addr,base64.StdEncoding.EncodeToString(key))
 if _,err:=io.WriteString(conn,req);err!=nil{_ =conn.Close();return nil,nil,err}
 reader:=bufio.NewReader(conn)
 response,err:=http.ReadResponse(reader,&http.Request{Method:http.MethodGet});if err!=nil{_ =conn.Close();return nil,nil,err}
 if response.StatusCode!=101{_ =conn.Close();return nil,nil,errors.New("not a websocket upgrade")}
 return conn,reader,nil
}
func connectCommand(c net.Conn,rd *bufio.Reader,token string,id int,allowed bool)error{
 proof,err:=signedProof(token);if err!=nil{return err}
 d,err:=json.Marshal(map[string]string{"dpop":proof});if err!=nil{return err}
 raw,err:=json.Marshal(map[string]any{"id":id,"connect":map[string]any{"token":token,"data":json.RawMessage(d)}});if err!=nil{return err}
 if err:=sendFrame(c,append(raw,'\n'));err!=nil{return err}
 reply,err:=receiveFrame(c,rd);if err!=nil{return err}
 var parsed map[string]json.RawMessage
 if err:=json.Unmarshal(reply,&parsed);err!=nil{return err}
 if allowed&&len(parsed["connect"])==0{return errors.New("valid auth rejected")}
 return nil
}
func subscribeCommand(c net.Conn,rd *bufio.Reader,id int)error{
 raw,err:=json.Marshal(map[string]any{"id":id,"subscribe":map[string]string{"channel":realtime.Channel(owner,owner)}})
 if err!=nil{return err}
 if err:=sendFrame(c,append(raw,'\n'));err!=nil{return err}
 response,err:=receiveFrame(c,rd);if err!=nil{return err}
 var parsed map[string]json.RawMessage
 if err:=json.Unmarshal(response,&parsed);err!=nil{return err}
 if len(parsed["subscribe"])==0{return errors.New("subscription denied")}
 return nil
}

func readExplicitAuthDenial(c net.Conn,rd *bufio.Reader)error {
 if err:=c.SetReadDeadline(time.Now().Add(5*time.Second));err!=nil{return err}
 header:=make([]byte,2)
 if _,err:=io.ReadFull(rd,header);err!=nil{return errors.New("auth denial missing (transport failure)")}
 size:=int(header[1]&0x7f)
 if size==126||size==127||header[1]&0x80!=0{return errors.New("invalid auth denial frame")}
 body:=make([]byte,size)
 if _,err:=io.ReadFull(rd,body);err!=nil{return errors.New("auth denial frame truncated")}
 if header[0]&0x0f==0x8 {
  if len(body)<2||int(body[0])<<8|int(body[1])!=int(centrifuge.DisconnectInvalidToken.Code){
   return errors.New("unexpected websocket close code")
  }
  return nil
 }
 if header[0]&0x0f!=0x1{return errors.New("unexpected auth denial frame type")}
 var reply struct{Disconnect struct{Code uint32 `json:"code"`} `json:"disconnect"`}
 if err:=json.Unmarshal(body,&reply);err!=nil{return errors.New("auth rejection malformed")}
 if reply.Disconnect.Code!=uint32(centrifuge.DisconnectInvalidToken.Code){return errors.New("expected Centrifuge invalid-token denial (3500)")}
 return nil
}

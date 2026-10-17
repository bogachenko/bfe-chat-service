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
const owner chat.AccountID="11111111-1111-1111-1111-111111111111"
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
 token:=flag.String("token","opaque-session-token","BFE Drive token")
 expect:=flag.String("expect","ok","ok, invalid or unavailable")
 flag.Parse()
 if *socket=="" {exit(errors.New("missing socket"))}
 if *expect!="ok"&&*expect!="invalid"&&*expect!="unavailable"{exit(errors.New("invalid expectation"))}
 v,err:=authentication.NewValidator(*socket,5*time.Minute);if err!=nil{exit(err)}
 node,err:=centrifuge.New(centrifuge.Config{});if err!=nil{exit(err)}
 svc:=realtime.Service{Auth:v,Permissions:policy{},Assignments:assignment{},Publisher:publish{node:node}}
 if err:=svc.Bind(node,endpoint);err!=nil{exit(err)}
 if err:=node.Run();err!=nil{exit(err)}
 defer func(){ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();_ =node.Shutdown(ctx)}()
 srv:=httptest.NewServer(centrifuge.NewWebsocketHandler(node,centrifuge.WebsocketConfig{}))
 defer srv.Close()
 if err:=checkWebsocket(strings.TrimPrefix(srv.URL,"http://"),*token,*expect);err!=nil{exit(err)}
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
 command,err:=json.Marshal(map[string]any{"id":1,"connect":map[string]any{"token":token,"data":data}});if err!=nil{return err}
 if err:=sendFrame(conn,append(command,'\n'));err!=nil{return err}
 raw,err:=receiveFrame(conn,rd)
 if expect=="ok" {
  if err!=nil{return err}
  var reply map[string]json.RawMessage
  if err:=json.Unmarshal(raw,&reply);err!=nil{return err}
  if len(reply["connect"])==0{return errors.New("valid session was not accepted")}
  return nil
 }
 if err==nil {
  var reply map[string]json.RawMessage
  if json.Unmarshal(raw,&reply)==nil&&len(reply["connect"])!=0{return errors.New("revoked/unavailable session was accepted")}
 }
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
func exit(err error){fmt.Fprintln(os.Stderr,"wsprobe:",err);os.Exit(1)}

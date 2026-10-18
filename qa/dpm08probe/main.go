// Command dpm08probe exercises the production HTTP and WebSocket endpoints.
// It prints ONLY fixed milestone identifiers, never tokens, proofs or responses.
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
 "os"
 "strings"
 "time"

 jose "github.com/go-jose/go-jose/v4"
)

const publicOrigin = "https://chat.example.test"
const otherOwner = "99999999-9999-9999-9999-999999999999"

func main(){
 address:=flag.String("address","","loopback executable address host:port")
 owner:=flag.String("owner","","BFE Drive account UUID")
 flag.Parse()
 if err:=run(*address,*owner);err!=nil{
  // Deliberately discard underlying errors: HTTP/client errors may contain secrets.
  fmt.Fprintln(os.Stderr,"DP-M08 scenario failed (details suppressed)")
  os.Exit(1)
 }
}
func stage(name string){fmt.Println(name);_ =os.Stdout.Sync()}
type scenario struct {
 address string
 owner string
 root string
 token string
 conn net.Conn
 reader *bufio.Reader
 commands *bufio.Reader
}

func run(address,owner string)error{
 token:=os.Getenv("CHAT_DPM08_TOKEN")
 if token==""||address==""||owner==""{return errors.New("missing private fixture configuration")}
 sc:=scenario{address:address,owner:owner,root:"/v1/conversations/"+owner,token:token,commands:bufio.NewReader(os.Stdin)}
 if err:=sc.initialHTTP();err!=nil{return err}
 if err:=sc.openConnection();err!=nil{return err}
 defer func(){_ =sc.conn.Close()}()
 if err:=sc.deliverBeforeRevoke();err!=nil{return err}
 if err:=sc.checkRevoked();err!=nil{return err}
 if err:=sc.checkFreshSession();err!=nil{return err}
 return sc.checkUDSOutage()
}
func(sc scenario)initialHTTP()error{
 if err:=expectHTTP(sc.address,http.MethodPut,sc.root+"/assignment",sc.token,
  map[string]string{"recipientAccountId":"22222222-2222-2222-2222-222222222222"},200);err!=nil{return err}
 stage("HTTP_ASSIGNMENT")
 if err:=expectHTTP(sc.address,http.MethodPost,sc.root+"/messages",sc.token,
  map[string]any{"idempotencyKey":"33333333-3333-3333-3333-333333333333","text":"initial"},200);err!=nil{return err}
 stage("HTTP_SEND")
 if err:=expectHTTP(sc.address,http.MethodGet,sc.root+"/history",sc.token,nil,200);err!=nil{return err}
 stage("HTTP_HISTORY")
 if err:=expectHTTP(sc.address,http.MethodPost,"/v1/conversations/"+otherOwner+"/messages",sc.token,
  map[string]any{"idempotencyKey":"44444444-4444-4444-4444-444444444444","text":"must deny"},403);err!=nil{return err}
 stage("HTTP_FORBIDDEN")
 return nil
}
func(sc *scenario)openConnection()error{
 conn,rd,err:=openSocket(sc.address)
 if err!=nil{return err}
 sc.conn=conn
 sc.reader=rd
 if err:=connectCommand(conn,rd,sc.token,1);err!=nil{return err}
 stage("WS_CONNECT")
 if err:=subscribeCommand(conn,rd,sc.owner,2);err!=nil{return err}
 stage("WS_SUBSCRIBE")
 return nil
}
func(sc scenario)deliverBeforeRevoke()error{
 if err:=expectHTTP(sc.address,http.MethodPost,sc.root+"/messages",sc.token,
  map[string]any{"idempotencyKey":"55555555-5555-5555-5555-555555555555","text":"realtime"},200);err!=nil{return err}
 stage("WS_DELIVERY_HTTP_COMMITTED")
 if err:=receivePublication(sc.conn,sc.reader,true);err!=nil{return err}
 stage("WS_DELIVERY")
 return nil
}
func(sc scenario)checkRevoked()error{
 if err:=expectControl(sc.commands,"REVOKED");err!=nil{return err}
 if err:=expectHTTP(sc.address,http.MethodGet,sc.root+"/history",sc.token,nil,401);err!=nil{return err}
 stage("HTTP_REVOKED")
 if err:=expectDeniedSocket(sc.address,sc.token);err!=nil{return err}
 stage("WS_RECONNECT_DENIED")
 return nil
}
func(sc scenario)checkFreshSession()error{
 if err:=expectControl(sc.commands,"NEW");err!=nil{return err}
 fresh,err:=sc.commands.ReadString('\n')
 if err!=nil{return errors.New("fresh token unavailable")}
 fresh=strings.TrimSpace(fresh)
 if fresh==""||fresh==sc.token{return errors.New("invalid second session")}
 if err:=expectHTTP(sc.address,http.MethodGet,sc.root+"/history",fresh,nil,200);err!=nil{return err}
 stage("HTTP_FRESH_SESSION")
 return sc.verifyFreshConnections(fresh)
}
func(sc scenario)verifyFreshConnections(fresh string)error{
 newer,rd,err:=openSocket(sc.address)
 if err!=nil{return err}
 defer func(){_ =newer.Close()}()
 if err:=connectCommand(newer,rd,fresh,3);err!=nil{return err}
 if err:=subscribeCommand(newer,rd,sc.owner,4);err!=nil{return err}
 stage("WS_FRESH_RECONNECT")
 if err:=expectHTTP(sc.address,http.MethodPost,sc.root+"/messages",fresh,
  map[string]any{"idempotencyKey":"66666666-6666-6666-6666-666666666666","text":"after revoke"},200);err!=nil{return err}
 if err:=receivePublication(sc.conn,sc.reader,false);err!=nil{return err}
 stage("OPEN_AFTER_REVOKE")
 if err:=receivePublication(newer,rd,false);err!=nil{return err}
 stage("WS_FRESH_DELIVERY")
 return nil
}
func(sc scenario)checkUDSOutage()error{
 if err:=expectControl(sc.commands,"UDS_DOWN");err!=nil{return err}
 // The fixture revokes the UDS only after the second access token has been
 // established. Obtain it again from the private control channel instead of
 // persisting it in a report or command-line arguments.
 fresh,err:=sc.commands.ReadString('\n')
 if err!=nil{return errors.New("outage token unavailable")}
 fresh=strings.TrimSpace(fresh)
 if fresh==""||fresh==sc.token{return errors.New("invalid outage fixture token")}
 if err:=expectHTTP(sc.address,http.MethodGet,sc.root+"/history",fresh,nil,503);err!=nil{return err}
 stage("HTTP_UDS_UNAVAILABLE")
 if err:=expectDeniedSocket(sc.address,fresh);err!=nil{return err}
 stage("WS_UDS_UNAVAILABLE")
 return nil
}
func expectControl(reader *bufio.Reader,expected string)error{
 command,err:=reader.ReadString('\n')
 if err!=nil||strings.TrimSpace(command)!=expected{return errors.New("unexpected fixture stage")}
 return nil
}
func expectHTTP(addr,method,path,token string,payload any,want int)error{
 var body io.Reader
 if payload!=nil{
  encoded,err:=json.Marshal(payload)
  if err!=nil{return err}
  body=strings.NewReader(string(encoded))
 }
 req,err:=http.NewRequestWithContext(context.Background(),method,"http://"+addr+path,body)
 if err!=nil{return err}
 if payload!=nil{req.Header.Set("Content-Type","application/json")}
 proof,err:=dpop(token,method,publicOrigin+path)
 if err!=nil{return err}
 req.Header.Set("Authorization","DPoP "+token)
 req.Header.Set("DPoP",proof)
 client:=&http.Client{Timeout:5*time.Second}
 response,err:=client.Do(req)
 if err!=nil{return errors.New("http transport failed")}
 defer func(){_ =response.Body.Close()}()
 // Never copy bodies/headers into diagnostics.
 if response.StatusCode!=want{return errors.New("unexpected HTTP authorization or persistence result")}
 return nil
}
func key()ed25519.PrivateKey{
 seed:=sha256.Sum256([]byte("dp-m01-independent-fixture-private-key"))
 return ed25519.NewKeyFromSeed(seed[:])
}
func dpop(token,method,target string)(string,error){
 private:=key()
 signer,err:=jose.NewSigner(jose.SigningKey{Algorithm:jose.EdDSA,Key:jose.JSONWebKey{Key:private,Algorithm:string(jose.EdDSA)}},
  &jose.SignerOptions{EmbedJWK:true,ExtraHeaders:map[jose.HeaderKey]interface{}{jose.HeaderType:"dpop+jwt"}})
 if err!=nil{return "",err}
 nonce:=make([]byte,16)
 if _,err:=rand.Read(nonce);err!=nil{return "",err}
 sum:=sha256.Sum256([]byte(token))
 claims,err:=json.Marshal(map[string]any{
  "jti":base64.RawURLEncoding.EncodeToString(nonce),
  "iat":time.Now().Unix(),
  "htm":method,"htu":target,
  "ath":base64.RawURLEncoding.EncodeToString(sum[:]),
 })
 if err!=nil{return "",err}
 signed,err:=signer.Sign(claims)
 if err!=nil{return "",err}
 _,err=(&jose.JSONWebKey{Key:private.Public()}).Thumbprint(crypto.SHA256)
 if err!=nil{return "",err}
 return signed.CompactSerialize()
}
func openSocket(addr string)(net.Conn,*bufio.Reader,error){
 conn,err:=net.DialTimeout("tcp",addr,5*time.Second)
 if err!=nil{return nil,nil,err}
 secret:=make([]byte,16)
 if _,err=rand.Read(secret);err!=nil{_ =conn.Close();return nil,nil,err}
 handshake:=fmt.Sprintf("GET /ws HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: %s\r\n\r\n",
  addr,base64.StdEncoding.EncodeToString(secret))
 if _,err=io.WriteString(conn,handshake);err!=nil{_ =conn.Close();return nil,nil,err}
 reader:=bufio.NewReader(conn)
 response,err:=http.ReadResponse(reader,&http.Request{Method:http.MethodGet})
 if err!=nil{_ =conn.Close();return nil,nil,err}
 if response.StatusCode!=http.StatusSwitchingProtocols{_ =response.Body.Close();_ =conn.Close();return nil,nil,errors.New("WS upgrade denied")}
 return conn,reader,nil
}
func sendFrame(conn net.Conn,bytes []byte)error{
 if len(bytes)>65535{return errors.New("WS command too large")}
 mask:=make([]byte,4)
 if _,err:=rand.Read(mask);err!=nil{return err}
 header:=[]byte{0x81,0xfe,byte(len(bytes)>>8),byte(len(bytes))}
 if len(bytes)<126{header=[]byte{0x81,0x80|byte(len(bytes))}}
 header=append(header,mask...)
 for i:=range bytes{bytes[i]^=mask[i%4]}
 if err:=conn.SetWriteDeadline(time.Now().Add(5*time.Second));err!=nil{return err}
 _,err:=conn.Write(append(header,bytes...))
 return err
}
func receiveFrame(conn net.Conn,reader *bufio.Reader)(byte,[]byte,error){
 if err:=conn.SetReadDeadline(time.Now().Add(5*time.Second));err!=nil{return 0,nil,err}
 header:=make([]byte,2)
 if _,err:=io.ReadFull(reader,header);err!=nil{return 0,nil,errors.New("WS transport failed")}
 if header[1]&0x80!=0{return 0,nil,errors.New("server returned masked frame")}
 size:=int(header[1]&127)
 if size==126{
  large:=make([]byte,2)
  if _,err:=io.ReadFull(reader,large);err!=nil{return 0,nil,err}
  size=int(large[0])<<8|int(large[1])
 }
 if size==127{return 0,nil,errors.New("oversized WS frame")}
 payload:=make([]byte,size)
 if _,err:=io.ReadFull(reader,payload);err!=nil{return 0,nil,err}
 return header[0]&15,payload,nil
}
func sendCommand(conn net.Conn,id int,key string,data any)error{
 payload,err:=json.Marshal(map[string]any{"id":id,key:data})
 if err!=nil{return err}
 return sendFrame(conn,append(payload,'\n'))
}
func connectCommand(conn net.Conn,reader *bufio.Reader,token string,id int)error{
 proof,err:=dpop(token,"GET",publicOrigin+"/ws")
 if err!=nil{return err}
 if err:=sendCommand(conn,id,"connect",map[string]any{"token":token,"data":map[string]string{"dpop":proof}});err!=nil{return err}
 op,raw,err:=receiveFrame(conn,reader)
 if err!=nil||op!=1{return errors.New("connect response unavailable")}
 var reply struct{Connect json.RawMessage `json:"connect"`}
 if json.Unmarshal(raw,&reply)!=nil||len(reply.Connect)==0{return errors.New("valid WS Connect rejected")}
 return nil
}
func subscribeCommand(conn net.Conn,reader *bufio.Reader,owner string,id int)error{
 channel:="chat:"+owner+":"+owner
 if err:=sendCommand(conn,id,"subscribe",map[string]string{"channel":channel});err!=nil{return err}
 op,raw,err:=receiveFrame(conn,reader)
 if err!=nil||op!=1{return errors.New("subscribe response unavailable")}
 var reply struct{Subscribe json.RawMessage `json:"subscribe"`}
 if json.Unmarshal(raw,&reply)!=nil||len(reply.Subscribe)==0{return errors.New("authorized WS Subscribe rejected")}
 return nil
}
func receivePublication(conn net.Conn,reader *bufio.Reader,diagnostics bool)error{
 op,raw,err:=receiveFrame(conn,reader)
 if err!=nil||op!=1{return errors.New("realtime publication unavailable")}
 if diagnostics{stage("WS_EVENT_FRAME")}
 var push struct{Push struct{Pub struct{Data json.RawMessage `json:"data"`} `json:"pub"`} `json:"push"`}
 if json.Unmarshal(raw,&push)!=nil||len(push.Push.Pub.Data)==0{return errors.New("unexpected realtime frame")}
 if diagnostics{stage("WS_EVENT_PARSED")}
 var event struct{MessageID int64 `json:"messageId"`}
 if json.Unmarshal(push.Push.Pub.Data,&event)!=nil||event.MessageID<=0{return errors.New("invalid committed message event")}
 return nil
}
func expectDeniedSocket(addr,token string)error{
 conn,reader,err:=openSocket(addr)
 if err!=nil{return err}
 defer func(){_ =conn.Close()}()
 proof,err:=dpop(token,"GET",publicOrigin+"/ws")
 if err!=nil{return err}
 if err:=sendCommand(conn,1,"connect",map[string]any{"token":token,"data":map[string]string{"dpop":proof}});err!=nil{return err}
 op,raw,err:=receiveFrame(conn,reader)
 if err!=nil{return errors.New("expected explicit invalid-session close, not timeout")}
 return verifyInvalidTokenDisconnect(op,raw)
}
func verifyInvalidTokenDisconnect(op byte,raw []byte)error{
 const invalidCode=3500
 if op==8 {
  if len(raw)<2||int(raw[0])<<8|int(raw[1])!=invalidCode{return errors.New("unexpected WS close code")}
  return nil
 }
 if op!=1{return errors.New("unexpected auth rejection frame")}
 var reply struct{Disconnect struct{Code uint32 `json:"code"`} `json:"disconnect"`}
 if json.Unmarshal(raw,&reply)!=nil||reply.Disconnect.Code!=invalidCode{return errors.New("expected Centrifuge invalid-token disconnect")}
 return nil
}

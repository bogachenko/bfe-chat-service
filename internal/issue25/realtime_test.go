package issue25
import ("encoding/json";"strings";"testing")
func TestNotificationEnvelopeHasNoIdentifiers(t *testing.T){
 a:=Notification{Version:1,WakeToken:"opaque-123"}
 raw,err:=json.Marshal(a);if err!=nil{t.Fatal(err)}
 var fields map[string]any;if err=json.Unmarshal(raw,&fields);err!=nil{t.Fatal(err)}
 if len(fields)!=2||fields["version"]!=float64(1)||fields["wakeToken"]!="opaque-123"{t.Fatalf("unexpected envelope %s",raw)}
 for _,word:=range []string{"messageId","ordinal","viewer","conversation","epoch","document","sender"}{if strings.Contains(string(raw),word){t.Fatal("protected field")}}
}
func TestOpaqueChannel(t *testing.T){
 a,err:=OpaqueChannel();if err!=nil{t.Fatal(err)}
 b,err:=OpaqueChannel();if err!=nil{t.Fatal(err)}
 if a==b||!strings.HasPrefix(a,"chat:e:")||len(a)<25{t.Fatal("opaque channel collision")}
}

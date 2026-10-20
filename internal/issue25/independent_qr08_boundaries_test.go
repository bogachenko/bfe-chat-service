package issue25

// Independent boundary, input validation, and opaque notification regressions.
import (
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "strings"
 "testing"
)
func TestIndependentQR08CanonicalPayloadBoundary(t *testing.T) {
 id,title:="doc-id","Document title"
 cases:=[]struct{name string;payload Payload;valid bool}{
  {"max text",Payload{Text:strings.Repeat("x",16384)},true},
  {"one byte over",Payload{Text:strings.Repeat("x",16385)},false},
  {"document only",Payload{DocumentID:&id,DocumentTitle:&title},true},
  {"text and document",Payload{Text:"x",DocumentID:&id,DocumentTitle:&title},true},
  {"missing title",Payload{Text:"x",DocumentID:&id},false},
  {"missing id",Payload{Text:"x",DocumentTitle:&title},false},
  {"whitespace only",Payload{Text:"\n \t"},false},
  {"empty",Payload{},false},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){
  raw,err:=CanonicalPayload(tc.payload)
  if tc.valid {qaRejectf(t, (err!=nil||len(raw)==0), "valid payload rejected: %v",err)
   replay,e:=CanonicalPayload(tc.payload);qaRejectf(t, (e!=nil||!bytes.Equal(raw,replay)), "canonical instability: %v",e)
  }else if !errors.Is(err,ErrInvalid){t.Fatalf("invalid payload accepted: %v",err)}
 })}
}
func TestIndependentQR08UUIDInputValidation(t *testing.T){
 id,e:=newUUID();qaRejectf(t, (e!=nil||!validUUID(id)), "new UUID invalid: %q: %v",id,e)
 for _,invalid:=range []string{"","abc","../","00000000-0000-0000-0000-00000000000g",id+"a",strings.ToUpper(id)+"!"}{
  qaRejectf(t, (validUUID(invalid)), "invalid UUID accepted: %q",invalid)
 }
 qaReject(t, (!validUUID(strings.ToUpper(id))), "valid uppercase UUID rejected")
}
func TestIndependentQR08OpaqueEnvelope(t *testing.T){
 a,e:=OpaqueChannel();qaReject(t, (e!=nil), e)
 b,e:=OpaqueChannel();qaReject(t, (e!=nil), e)
 qaRejectf(t, (a==b||!strings.HasPrefix(a,"chat:e:")||len(a)!=len("chat:e:")+36), "channel invariants: %q %q",a,b)
 w,e:=WakeToken();qaRejectf(t, (e!=nil||!validUUID(w)), "invalid wake token: %q %v",w,e)
 raw,e:=json.Marshal(Notification{Version:1,WakeToken:w});qaReject(t, (e!=nil), e)
 var m map[string]any;func() { e=json.Unmarshal(raw,&m); qaReject(t, (e!=nil), e) }()
 qaRejectf(t, (len(m)!=2||m["version"]!=float64(1)||m["wakeToken"]!=w), "notification leaked fields: %s",raw)
}
func TestIndependentQR08NilStoreFailsClosed(t *testing.T){
 s:=Store{}
 if _,err:=s.Send(context.Background(),"","", "",Payload{});!errors.Is(err,ErrInvalid){t.Fatalf("invalid payload on nil store: %v",err)}
}

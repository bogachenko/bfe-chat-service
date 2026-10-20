package issue25

import (
 "bytes"
 "errors"
 "testing"
)

func TestCanonicalPayload(t *testing.T) {
 base,err:=CanonicalPayload(Payload{Text:"hello"})
 if err!=nil||len(base)==0{t.Fatalf("canonical: %v",err)}
 same,_:=CanonicalPayload(Payload{Text:"hello"})
 if !bytes.Equal(base,same){t.Fatal("not stable")}
 other,_:=CanonicalPayload(Payload{Text:"hello "})
 if bytes.Equal(base,other){t.Fatal("changed payload must differ")}
 for _,p:=range []Payload{{Text:""},{Text:string(bytes.Repeat([]byte{'a'},16385))},{Text:"ok",DocumentID:ptr("id")},{Text:"",DocumentID:ptr("id"),DocumentTitle:ptr(" ")}}{
  if _,err:=CanonicalPayload(p);!errors.Is(err,ErrInvalid){t.Fatalf("expected invalid: %+v %v",p,err)}
 }
}
func ptr(s string)*string{return &s}
func TestUniqueUUID(t *testing.T){
 seen:=map[string]bool{}
 for i:=0;i<100;i++{x,e:=newUUID();if e!=nil||seen[x]||len(x)!=36{t.Fatalf("uuid=%s error=%v",x,e)};seen[x]=true}
}

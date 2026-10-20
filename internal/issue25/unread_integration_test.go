package issue25

import (
 "context"
 "testing"
)

func TestPGMS2512ReadMarkerAndUnreadAcrossEpochs(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,viewer:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"read-unread",[]string{viewer});if err!=nil{t.Fatal(err)}
 sender:=func(user,text string)Message{t.Helper();m,e:=s.Send(ctx,group,user,fixtureID(t),Payload{Text:text});if e!=nil{t.Fatal(e)};return m}
 m1:=sender(owner,"other-1")
 _=sender(viewer,"mine")
 m3:=sender(owner,"other-3")
 if n,err:=s.Unread(ctx,group,viewer);err!=nil||n!=2{t.Fatalf("initial unread %d: %v",n,err)}
 if marker,err:=s.MarkRead(ctx,group,viewer,m1.Ordinal);err!=nil||marker!=m1.Ordinal{t.Fatalf("mark1=%d %v",marker,err)}
 if n,err:=s.Unread(ctx,group,viewer);err!=nil||n!=1{t.Fatalf("after read unread %d: %v",n,err)}
 if marker,err:=s.MarkRead(ctx,group,viewer,0);err!=nil||marker!=m1.Ordinal{t.Fatalf("stale marker=%d %v",marker,err)}
 if err:=s.Remove(ctx,group,owner,viewer);err!=nil{t.Fatal(err)}
 _=sender(owner,"absent-4")
 if _,err:=s.Join(ctx,group,owner,viewer,"member");err!=nil{t.Fatal(err)}
 _=sender(owner,"other-5")
 if n,err:=s.Unread(ctx,group,viewer);err!=nil||n!=2{t.Fatalf("after rejoin unread=%d %v",n,err)}
 if marker,err:=s.MarkRead(ctx,group,viewer,m3.Ordinal);err!=nil||marker!=m3.Ordinal{t.Fatalf("mark3=%d %v",marker,err)}
 if n,err:=s.Unread(ctx,group,viewer);err!=nil||n!=1{t.Fatalf("expected one visible after read, unread=%d %v",n,err)}
}

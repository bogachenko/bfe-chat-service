package issue25

import (
 "context"
 "testing"
)

func TestPGMS2512ReadMarkerAndUnreadAcrossEpochs(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,viewer:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"read-unread",[]string{viewer});qaReject(t, (err!=nil), err)
 sender:=func(user,text string)Message{t.Helper();m,e:=s.Send(ctx,group,user,fixtureID(t),Payload{Text:text});qaReject(t, (e!=nil), e);return m}
 m1:=sender(owner,"other-1")
 _=sender(viewer,"mine")
 m3:=sender(owner,"other-3")
 func() { n,err:=s.Unread(ctx,group,viewer); qaRejectf(t, (err!=nil||n!=2), "initial unread %d: %v",n,err) }()
 func() { marker,err:=s.MarkRead(ctx,group,viewer,m1.Ordinal); qaRejectf(t, (err!=nil||marker!=m1.Ordinal), "mark1=%d %v",marker,err) }()
 func() { n,err:=s.Unread(ctx,group,viewer); qaRejectf(t, (err!=nil||n!=1), "after read unread %d: %v",n,err) }()
 func() { marker,err:=s.MarkRead(ctx,group,viewer,0); qaRejectf(t, (err!=nil||marker!=m1.Ordinal), "stale marker=%d %v",marker,err) }()
 func() { err:=s.Remove(ctx,group,owner,viewer); qaReject(t, (err!=nil), err) }()
 _=sender(owner,"absent-4")
 func() { _,err:=s.Join(ctx,group,owner,viewer,"member"); qaReject(t, (err!=nil), err) }()
 _=sender(owner,"other-5")
 func() { n,err:=s.Unread(ctx,group,viewer); qaRejectf(t, (err!=nil||n!=2), "after rejoin unread=%d %v",n,err) }()
 func() { marker,err:=s.MarkRead(ctx,group,viewer,m3.Ordinal); qaRejectf(t, (err!=nil||marker!=m3.Ordinal), "mark3=%d %v",marker,err) }()
 func() { n,err:=s.Unread(ctx,group,viewer); qaRejectf(t, (err!=nil||n!=1), "expected one visible after read, unread=%d %v",n,err) }()
}

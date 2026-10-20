package issue25

import (
 "context"
 "testing"
)

func TestIndependentQR08ConversationListMembershipIsolation(t *testing.T) {
 db:=integrationDB(t)
 s:=Store{DB:db}
 ctx:=context.Background()
 owner,member,outsider:=fixtureID(t),fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"qa visible group",[]string{member})
 qaReject(t, (err!=nil), err)
 for _,tc:=range []struct{user string;present bool;role string}{
  {owner,true,"owner"},{member,true,"member"},{outsider,false,""},
 }{
  items,e:=s.ListConversations(ctx,tc.user)
  qaReject(t, (e!=nil), e)
  found:=false
  for _,item:=range items {
   if item.ID!=group{continue}
   found=true
   qaRejectf(t, (item.Kind!="group"||item.Role!=tc.role||item.Name==nil||*item.Name!="qa visible group"), "unexpected group listing: %+v",item)
  }
  qaRejectf(t, (found!=tc.present), "member %s: listed=%t expected=%t",tc.user,found,tc.present)
 }
 func() { e:=s.Remove(ctx,group,owner,member); qaReject(t, (e!=nil), e) }()
 items,err:=s.ListConversations(ctx,member)
 qaReject(t, (err!=nil), err)
 for _,item:=range items{qaReject(t, (item.ID==group), "removed member can still list conversation")}
}

func TestIndependentQR08NotificationRecipientsRevocation(t *testing.T) {
 db:=integrationDB(t)
 s:=Store{DB:db}
 ctx:=context.Background()
 owner,viewer:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"qa notifications",[]string{viewer})
 qaReject(t, (err!=nil), err)
 before,err:=s.NotificationRecipients(ctx,group)
 qaReject(t, (err!=nil), err)
 qaRejectf(t, (len(before)!=2), "active recipient count=%d, want 2",len(before))
 epoch:=""
 for _,v:=range before{if v.Viewer==viewer{epoch=v.Epoch}}
 qaRejectf(t, (!validUUID(epoch)), "missing opaque membership epoch: %q",epoch)
 func() { err=s.AuthorizeEpochChannel(ctx,"chat:e:"+epoch,viewer); qaReject(t, (err!=nil), err) }()
 func() { err=s.Remove(ctx,group,owner,viewer); qaReject(t, (err!=nil), err) }()
 after,err:=s.NotificationRecipients(ctx,group)
 qaReject(t, (err!=nil), err)
 qaRejectf(t, (len(after)!=1||after[0].Viewer!=owner), "removed viewer still notified: %+v",after)
 func() { err=s.AuthorizeEpochChannel(ctx,"chat:e:"+epoch,viewer); qaRejectf(t, (err!=ErrForbidden), "retired epoch subscription not rejected: %v",err) }()
 newEpoch,err:=s.Join(ctx,group,owner,viewer,"member")
 qaReject(t, (err!=nil), err)
 qaReject(t, (newEpoch==epoch), "rejoined membership reused retired epoch")
 func() { err=s.AuthorizeEpochChannel(ctx,"chat:e:"+newEpoch,viewer); qaReject(t, (err!=nil), err) }()
 func() { err=s.AuthorizeEpochChannel(ctx,"chat:e:"+epoch,viewer); qaRejectf(t, (err!=ErrForbidden), "old epoch revived after rejoin: %v",err) }()
}

func TestIndependentQR08HistoryDocumentAndCursor(t *testing.T){
 db:=integrationDB(t)
 s:=Store{DB:db}
 ctx:=context.Background()
 owner,viewer:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"qa document history",[]string{viewer})
 qaReject(t, (err!=nil), err)
 docID,title:="document-qa-1","QA attachment"
 first,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{DocumentID:&docID,DocumentTitle:&title})
 qaReject(t, (err!=nil), err)
 second,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:"second"})
 qaReject(t, (err!=nil), err)
 items,err:=s.ReadHistoryDetailed(ctx,group,viewer,0,1)
 qaReject(t, (err!=nil), err)
 qaRejectf(t, (len(items)!=1||items[0].Ordinal!=first.Ordinal||items[0].Document==nil), "missing first document: %+v",items)
 qaRejectf(t, (items[0].Document.ID!=docID||items[0].Document.Title!=title), "document metadata changed: %+v",items[0].Document)
 items,err=s.ReadHistoryDetailed(ctx,group,viewer,first.Ordinal,100)
 qaReject(t, (err!=nil), err)
 qaRejectf(t, (len(items)!=1||items[0].Ordinal!=second.Ordinal||items[0].Document!=nil), "invalid cursor or phantom document: %+v",items)
 for _,limit:=range []int{0,-1,101}{
  func() { _,e:=s.ReadHistoryDetailed(ctx,group,viewer,0,limit); qaRejectf(t, (e!=ErrInvalid), "limit=%d accepted: %v",limit,e) }()
 }
 func() { _,e:=s.ReadHistoryDetailed(ctx,group,viewer,-1,1); qaRejectf(t, (e!=ErrInvalid), "negative cursor accepted: %v",e) }()
}

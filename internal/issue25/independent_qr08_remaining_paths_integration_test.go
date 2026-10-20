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
 if err!=nil{t.Fatal(err)}
 for _,tc:=range []struct{user string;present bool;role string}{
  {owner,true,"owner"},{member,true,"member"},{outsider,false,""},
 }{
  items,e:=s.ListConversations(ctx,tc.user)
  if e!=nil{t.Fatal(e)}
  found:=false
  for _,item:=range items {
   if item.ID!=group{continue}
   found=true
   if item.Kind!="group"||item.Role!=tc.role||item.Name==nil||*item.Name!="qa visible group" {
    t.Fatalf("unexpected group listing: %+v",item)
   }
  }
  if found!=tc.present{t.Fatalf("member %s: listed=%t expected=%t",tc.user,found,tc.present)}
 }
 if e:=s.Remove(ctx,group,owner,member);e!=nil{t.Fatal(e)}
 items,err:=s.ListConversations(ctx,member)
 if err!=nil{t.Fatal(err)}
 for _,item:=range items{if item.ID==group{t.Fatal("removed member can still list conversation")}}
}

func TestIndependentQR08NotificationRecipientsRevocation(t *testing.T) {
 db:=integrationDB(t)
 s:=Store{DB:db}
 ctx:=context.Background()
 owner,viewer:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"qa notifications",[]string{viewer})
 if err!=nil{t.Fatal(err)}
 before,err:=s.NotificationRecipients(ctx,group)
 if err!=nil{t.Fatal(err)}
 if len(before)!=2{t.Fatalf("active recipient count=%d, want 2",len(before))}
 epoch:=""
 for _,v:=range before{if v.Viewer==viewer{epoch=v.Epoch}}
 if !validUUID(epoch){t.Fatalf("missing opaque membership epoch: %q",epoch)}
 if err=s.AuthorizeEpochChannel(ctx,"chat:e:"+epoch,viewer);err!=nil{t.Fatal(err)}
 if err=s.Remove(ctx,group,owner,viewer);err!=nil{t.Fatal(err)}
 after,err:=s.NotificationRecipients(ctx,group)
 if err!=nil{t.Fatal(err)}
 if len(after)!=1||after[0].Viewer!=owner{t.Fatalf("removed viewer still notified: %+v",after)}
 if err=s.AuthorizeEpochChannel(ctx,"chat:e:"+epoch,viewer);err!=ErrForbidden{
  t.Fatalf("retired epoch subscription not rejected: %v",err)
 }
 newEpoch,err:=s.Join(ctx,group,owner,viewer,"member")
 if err!=nil{t.Fatal(err)}
 if newEpoch==epoch{t.Fatal("rejoined membership reused retired epoch")}
 if err=s.AuthorizeEpochChannel(ctx,"chat:e:"+newEpoch,viewer);err!=nil{t.Fatal(err)}
 if err=s.AuthorizeEpochChannel(ctx,"chat:e:"+epoch,viewer);err!=ErrForbidden{
  t.Fatalf("old epoch revived after rejoin: %v",err)
 }
}

func TestIndependentQR08HistoryDocumentAndCursor(t *testing.T){
 db:=integrationDB(t)
 s:=Store{DB:db}
 ctx:=context.Background()
 owner,viewer:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"qa document history",[]string{viewer})
 if err!=nil{t.Fatal(err)}
 docID,title:="document-qa-1","QA attachment"
 first,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{DocumentID:&docID,DocumentTitle:&title})
 if err!=nil{t.Fatal(err)}
 second,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:"second"})
 if err!=nil{t.Fatal(err)}
 items,err:=s.ReadHistoryDetailed(ctx,group,viewer,0,1)
 if err!=nil{t.Fatal(err)}
 if len(items)!=1||items[0].Ordinal!=first.Ordinal||items[0].Document==nil{
  t.Fatalf("missing first document: %+v",items)
 }
 if items[0].Document.ID!=docID||items[0].Document.Title!=title{
  t.Fatalf("document metadata changed: %+v",items[0].Document)
 }
 items,err=s.ReadHistoryDetailed(ctx,group,viewer,first.Ordinal,100)
 if err!=nil{t.Fatal(err)}
 if len(items)!=1||items[0].Ordinal!=second.Ordinal||items[0].Document!=nil{
  t.Fatalf("invalid cursor or phantom document: %+v",items)
 }
 for _,limit:=range []int{0,-1,101}{
  if _,e:=s.ReadHistoryDetailed(ctx,group,viewer,0,limit);e!=ErrInvalid{
   t.Fatalf("limit=%d accepted: %v",limit,e)
  }
 }
 if _,e:=s.ReadHistoryDetailed(ctx,group,viewer,-1,1);e!=ErrInvalid{t.Fatalf("negative cursor accepted: %v",e)}
}

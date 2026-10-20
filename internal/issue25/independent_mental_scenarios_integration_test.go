package issue25

// Independent QA checks for Issue #25. PostgreSQL cases require isolated,
// migrated BFE_CHAT_TEST_DATABASE_URL and skip if it is not configured.
import (
 "context"
 "errors"
 "fmt"
 "strings"
 "sync"
 "testing"
)
func qaMessage(t *testing.T,s Store,c,u string,n int64) Message {
 t.Helper();m,e:=s.Send(context.Background(),c,u,fixtureID(t),Payload{Text:fmt.Sprintf("qa-%d",n)})
 qaRequiref(t, !(e!=nil||m.Ordinal!=n), "ordinal want %d got %d: %v",n,m.Ordinal,e);return m
}
func qaGroup(t *testing.T,s Store,o string,members ...string)string{
 t.Helper();c,e:=s.CreateGroup(context.Background(),o,"independent qa",members);qaRequire(t, !(e!=nil), e);return c
}
func TestIndependentMS2501Direct(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();a,b:=fixtureID(t),fixtureID(t)
 var wg sync.WaitGroup;out:=make(chan string,2);errs:=make(chan error,2)
 for _,p:=range [][2]string{{a,b},{b,a}}{wg.Add(1);go func(p [2]string){defer wg.Done();v,e:=s.OpenDirect(ctx,p[0],p[1]);out<-v;errs<-e}(p)}
 wg.Wait();close(out);close(errs);for e:=range errs{qaRequire(t, !(e!=nil), e)}
 x,y:=<-out,<-out;qaRequiref(t, !(x==""||x!=y), "distinct directs %q %q",x,y)
 func() { _,e:=s.OpenDirect(ctx,a,a); qaRequire(t, !(!errors.Is(e,ErrInvalid)), "self-direct accepted") }()
 func() { e:=s.HideDirect(ctx,x,a,true); qaRequire(t, !(e!=nil), e) }()
 qaMessage(t,s,x,b,1)
 var hidden bool;func() { e:=db.QueryRowContext(ctx,"SELECT hidden FROM issue25_user_state WHERE conversation_id=$1 AND account_id=$2",x,a).Scan(&hidden); qaRequiref(t, !(e!=nil||hidden), "incoming must reopen: %v",e) }()
}
func TestIndependentMS2502Roles(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,d,m,n:=fixtureID(t),fixtureID(t),fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o,d,m,n)
 func() { e:=s.ChangeAdmin(ctx,c,m,n,true); qaRequiref(t, !(!errors.Is(e,ErrForbidden)), "member grant=%v",e) }()
 func() { e:=s.ChangeAdmin(ctx,c,o,d,true); qaRequire(t, !(e!=nil), e) }()
 func() { e:=s.ChangeAdmin(ctx,c,d,m,true); qaRequiref(t, !(!errors.Is(e,ErrForbidden)), "admin grant=%v",e) }()
 func() { e:=s.TransferOwner(ctx,c,o,n); qaRequire(t, !(e!=nil), e) }()
 var count int;func() { e:=db.QueryRowContext(ctx,"SELECT count(*) FROM issue25_memberships WHERE conversation_id=$1 AND role='owner' AND leave_boundary IS NULL",c).Scan(&count); qaRequiref(t, !(e!=nil||count!=1), "owner count=%d %v",count,e) }()
}
func TestIndependentMS2503Intervals(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o)
 for i:=int64(1);i<=9;i++{qaMessage(t,s,c,o,i)}
 e1,e:=s.Join(ctx,c,o,v,"member");qaRequire(t, !(e!=nil), e)
 for i:=int64(10);i<=19;i++{qaMessage(t,s,c,o,i)}
 func() { e:=s.Remove(ctx,c,o,v); qaRequire(t, !(e!=nil), e) }()
 for i:=int64(20);i<=29;i++{qaMessage(t,s,c,o,i)}
 e2,e:=s.Join(ctx,c,o,v,"member");qaRequire(t, !(e!=nil||e1==e2), "epoch reused or rejoin failed")
 qaMessage(t,s,c,o,30);qaMessage(t,s,c,o,31)
 rows,e:=s.ReadHistory(ctx,c,v,0,50);qaRequire(t, !(e!=nil), e)
 expected:=[]int64{10,11,12,13,14,15,16,17,18,19,30,31};qaRequiref(t, !(len(rows)!=12), "visible=%d",len(rows))
 for i,n:=range expected{qaRequiref(t, !(rows[i].Ordinal!=n), "history[%d]=%d expected %d",i,rows[i].Ordinal,n)}
}
func TestIndependentMS2504SendRemoveBothOrders(t *testing.T){
 for _,first:=range []bool{true,false}{t.Run(fmt.Sprint("sendFirst=",first),func(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o,v)
 for i:=int64(1);i<=39;i++{qaMessage(t,s,c,o,i)}
 if first{qaMessage(t,s,c,v,40)}
 func() { e:=s.Remove(ctx,c,o,v); qaRequire(t, !(e!=nil), e) }()
 var boundary int64;func() { e:=db.QueryRowContext(ctx,"SELECT leave_boundary FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2",c,v).Scan(&boundary); qaRequire(t, !(e!=nil), e) }()
 want:=int64(40);if first{want=41};qaRequiref(t, !(boundary!=want), "boundary=%d want=%d",boundary,want)
 if _,e:=s.Send(ctx,c,v,fixtureID(t),Payload{Text:"denied"});!errors.Is(e,ErrForbidden){t.Fatalf("revoked send=%v",e)}
 if !first{qaMessage(t,s,c,o,40)}
 var next int64;func() { e:=db.QueryRowContext(ctx,"SELECT next_ordinal FROM issue25_conversations WHERE id=$1",c).Scan(&next); qaRequiref(t, !(e!=nil||next!=41), "next=%d %v",next,e) }()
 })}
}
func TestIndependentMS2505OwnerTransfer(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,n:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o,n)
 func() { e:=s.Remove(ctx,c,o,o); qaRequiref(t, !(!errors.Is(e,ErrForbidden)), "owner leave before transfer=%v",e) }()
 func() { e:=s.TransferOwner(ctx,c,o,n); qaRequire(t, !(e!=nil), e) }()
 func() { e:=s.Remove(ctx,c,o,o); qaRequire(t, !(e!=nil), e) }()
 var owner string;func() { e:=db.QueryRowContext(ctx,"SELECT account_id::text FROM issue25_memberships WHERE conversation_id=$1 AND role='owner' AND leave_boundary IS NULL",c).Scan(&owner); qaRequiref(t, !(e!=nil||owner!=n), "owner=%s %v",owner,e) }()
}
func TestIndependentMS2506Idempotency(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o:=fixtureID(t);c:=qaGroup(t,s,o);k:=fixtureID(t)
 a,e:=s.Send(ctx,c,o,k,Payload{Text:"abc"});qaRequire(t, !(e!=nil), e)
 b,e:=s.Send(ctx,c,o,k,Payload{Text:"abc"});qaRequiref(t, !(e!=nil||!b.Replayed||b.ID!=a.ID), "replay=%+v %v",b,e)
 if _,e:=s.Send(ctx,c,o,k,Payload{Text:"abd"});!errors.Is(e,ErrConflict){t.Fatalf("409=%v",e)}
}
func TestIndependentMS2507Cursor(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o:=fixtureID(t);c:=qaGroup(t,s,o)
 for i:=int64(1);i<=72;i++{qaMessage(t,s,c,o,i)}
 for _,v:=range []struct{after int64;want int64}{{70,71},{71,72},{72,0}}{
 rows,e:=s.ReadHistory(ctx,c,o,v.after,1);qaRequire(t, !(e!=nil), e)
 if v.want==0{qaRequire(t, !(len(rows)!=0), "cursor returned extra row")}else if len(rows)!=1||rows[0].Ordinal!=v.want{t.Fatalf("cursor after %d: %+v",v.after,rows)}
 }
}
func qaAttempt(t *testing.T,s Store,c,o,v string)(Attempt,Message){
 t.Helper();ctx:=context.Background();m:=qaMessage(t,s,c,o,1);var epoch string
 func() { e:=s.DB.QueryRowContext(ctx,"SELECT epoch::text FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL",c,v).Scan(&epoch); qaRequire(t, !(e!=nil), e) }()
 a:=Attempt{ID:fixtureID(t),Conversation:c,Viewer:v,Epoch:epoch,MessageID:m.ID,Ordinal:m.Ordinal,WakeToken:fixtureID(t),Channel:"chat:e:"+epoch}
 func() { e:=s.RegisterAttempt(ctx,a); qaRequire(t, !(e!=nil), e) }();return a,m
}
func TestIndependentMS2508ImmutableAttempt(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o,v);a,_:=qaAttempt(t,s,c,o,v)
 func() { e:=s.Remove(ctx,c,o,v); qaRequire(t, !(e!=nil), e) }();m:=qaMessage(t,s,c,o,2)
 func() { _,e:=db.ExecContext(ctx,"UPDATE issue25_publish_attempts SET message_id=$1 WHERE attempt_id=$2",m.ID,a.ID); qaRequire(t, !(e==nil), "attempt rebinding allowed") }()
 a.ID=fixtureID(t);a.MessageID=m.ID;a.Ordinal=m.Ordinal
 func() { e:=s.RegisterAttempt(ctx,a); qaRequiref(t, !(!errors.Is(e,ErrForbidden)), "retired epoch registration=%v",e) }()
}
func TestIndependentMS2509AtomicClaim(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o,v);a,_:=qaAttempt(t,s,c,o,v)
 var wg sync.WaitGroup;ok:=make(chan bool,8);errs:=make(chan error,8)
 for i:=0;i<8;i++{wg.Add(1);go func(){defer wg.Done();v,e:=s.ClaimAttempt(ctx,a.ID);ok<-v;errs<-e}()}
 wg.Wait();close(ok);close(errs);for e:=range errs{qaRequire(t, !(e!=nil), e)}
 n:=0;for x:=range ok{if x{n++}};qaRequiref(t, !(n!=1), "successful claims=%d",n)
}
func TestIndependentMS2510UnknownNoRetry(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o,v);a,_:=qaAttempt(t,s,c,o,v)
 func() { ok,e:=s.ClaimAttempt(ctx,a.ID); qaRequiref(t, !(e!=nil||!ok), "claim: %v %v",ok,e) }()
 func() { e:=s.SetAttemptOutcome(ctx,a.ID,"UNKNOWN"); qaRequire(t, !(e!=nil), e) }()
 func() { ok,e:=s.ClaimAttempt(ctx,a.ID); qaRequiref(t, !(e!=nil||ok), "UNKNOWN replay allowed: %v %v",ok,e) }()
}
func TestIndependentMS2511RevokedChannel(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o,v)
 var epoch string;func() { e:=db.QueryRowContext(ctx,"SELECT epoch::text FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL",c,v).Scan(&epoch); qaRequire(t, !(e!=nil), e) }()
 old:="chat:e:"+epoch;func() { e:=s.AuthorizeEpochChannel(ctx,old,v); qaRequire(t, !(e!=nil), e) }()
 func() { e:=s.Remove(ctx,c,o,v); qaRequire(t, !(e!=nil), e) }()
 func() { e:=s.AuthorizeEpochChannel(ctx,old,v); qaRequiref(t, !(!errors.Is(e,ErrForbidden)), "retired channel=%v",e) }()
 fresh,e:=s.Join(ctx,c,o,v,"member");qaRequiref(t, !(e!=nil||fresh==epoch), "fresh epoch %s %v",fresh,e)
 func() { e:=s.AuthorizeEpochChannel(ctx,old,v); qaRequiref(t, !(!errors.Is(e,ErrForbidden)), "old channel revived=%v",e) }()
}
func TestIndependentMS2512Unread(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o,v)
 qaMessage(t,s,c,o,1);qaMessage(t,s,c,v,2);qaMessage(t,s,c,o,3)
 func() { n,e:=s.Unread(ctx,c,v); qaRequiref(t, !(e!=nil||n!=2), "unread=%d %v",n,e) }()
 func() { n,e:=s.MarkRead(ctx,c,v,1); qaRequiref(t, !(e!=nil||n!=1), "marker=%d %v",n,e) }()
 func() { n,e:=s.MarkRead(ctx,c,v,0); qaRequiref(t, !(e!=nil||n!=1), "stale marker=%d %v",n,e) }()
}
func TestIndependentMS2513NonMemberDenied(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o);qaMessage(t,s,c,o,1)
 func() { _,e:=s.ReadHistoryDetailed(ctx,c,v,0,50); qaRequiref(t, !(!errors.Is(e,ErrForbidden)), "nonmember history=%v",e) }()
 func() { e:=s.AuthorizeEpochChannel(ctx,"chat:e:"+fixtureID(t),v); qaRequiref(t, !(!errors.Is(e,ErrForbidden)), "nonmember subscribe=%v",e) }()
}
func TestIndependentMS2514LegacyUnchanged(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t)
 func() { _,e:=db.ExecContext(ctx,"INSERT INTO chat_assignments(owner_account_id,recipient_account_id) VALUES($1,$2)",o,v); qaRequire(t, !(e!=nil), e) }()
 func() { _,e:=db.ExecContext(ctx,"INSERT INTO chat_conversations(owner_account_id) VALUES($1)",o); qaRequire(t, !(e!=nil), e) }()
 key:=fixtureID(t);func() { _,e:=db.ExecContext(ctx,"INSERT INTO chat_messages(owner_account_id,sender_account_id,recipient_account_id,idempotency_key,body) VALUES($1,$1,$2,$3,'qa-legacy')",o,v,key); qaRequire(t, !(e!=nil), e) }()
 user:=fixtureID(t);c:=qaGroup(t,s,user);qaMessage(t,s,c,user,1)
 var text string;func() { e:=db.QueryRowContext(ctx,"SELECT body FROM chat_messages WHERE owner_account_id=$1 AND idempotency_key=$2",o,key).Scan(&text); qaRequiref(t, !(e!=nil||text!="qa-legacy"), "legacy mutated %q %v",text,e) }()
}
func TestIndependentMS2515InvalidWritesAtomic(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o:=fixtureID(t);c:=qaGroup(t,s,o)
 for _,p:=range []Payload{{Text:strings.Repeat("a",16385)},{Text:"ok",DocumentID:ptr("doc")},{Text:" "}}{
 func() { _,e:=s.Send(ctx,c,o,fixtureID(t),p); qaRequiref(t, !(!errors.Is(e,ErrInvalid)), "invalid payload accepted: %v",e) }()
 }
 var n int;func() { e:=db.QueryRowContext(ctx,"SELECT count(*) FROM issue25_messages WHERE conversation_id=$1",c).Scan(&n); qaRequiref(t, !(e!=nil||n!=0), "partial message writes=%d %v",n,e) }()
}

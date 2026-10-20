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
 if e!=nil||m.Ordinal!=n{t.Fatalf("ordinal want %d got %d: %v",n,m.Ordinal,e)};return m
}
func qaGroup(t *testing.T,s Store,o string,members ...string)string{
 t.Helper();c,e:=s.CreateGroup(context.Background(),o,"independent qa",members);if e!=nil{t.Fatal(e)};return c
}
func TestIndependentMS2501Direct(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();a,b:=fixtureID(t),fixtureID(t)
 var wg sync.WaitGroup;out:=make(chan string,2);errs:=make(chan error,2)
 for _,p:=range [][2]string{{a,b},{b,a}}{wg.Add(1);go func(p [2]string){defer wg.Done();v,e:=s.OpenDirect(ctx,p[0],p[1]);out<-v;errs<-e}(p)}
 wg.Wait();close(out);close(errs);for e:=range errs{if e!=nil{t.Fatal(e)}}
 x,y:=<-out,<-out;if x==""||x!=y{t.Fatalf("distinct directs %q %q",x,y)}
 if _,e:=s.OpenDirect(ctx,a,a);!errors.Is(e,ErrInvalid){t.Fatal("self-direct accepted")}
 if e:=s.HideDirect(ctx,x,a,true);e!=nil{t.Fatal(e)}
 qaMessage(t,s,x,b,1)
 var hidden bool;if e:=db.QueryRowContext(ctx,"SELECT hidden FROM issue25_user_state WHERE conversation_id=$1 AND account_id=$2",x,a).Scan(&hidden);e!=nil||hidden{t.Fatalf("incoming must reopen: %v",e)}
}
func TestIndependentMS2502Roles(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,d,m,n:=fixtureID(t),fixtureID(t),fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o,d,m,n)
 if e:=s.ChangeAdmin(ctx,c,m,n,true);!errors.Is(e,ErrForbidden){t.Fatalf("member grant=%v",e)}
 if e:=s.ChangeAdmin(ctx,c,o,d,true);e!=nil{t.Fatal(e)}
 if e:=s.ChangeAdmin(ctx,c,d,m,true);!errors.Is(e,ErrForbidden){t.Fatalf("admin grant=%v",e)}
 if e:=s.TransferOwner(ctx,c,o,n);e!=nil{t.Fatal(e)}
 var count int;if e:=db.QueryRowContext(ctx,"SELECT count(*) FROM issue25_memberships WHERE conversation_id=$1 AND role='owner' AND leave_boundary IS NULL",c).Scan(&count);e!=nil||count!=1{t.Fatalf("owner count=%d %v",count,e)}
}
func TestIndependentMS2503Intervals(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o)
 for i:=int64(1);i<=9;i++{qaMessage(t,s,c,o,i)}
 e1,e:=s.Join(ctx,c,o,v,"member");if e!=nil{t.Fatal(e)}
 for i:=int64(10);i<=19;i++{qaMessage(t,s,c,o,i)}
 if e:=s.Remove(ctx,c,o,v);e!=nil{t.Fatal(e)}
 for i:=int64(20);i<=29;i++{qaMessage(t,s,c,o,i)}
 e2,e:=s.Join(ctx,c,o,v,"member");if e!=nil||e1==e2{t.Fatal("epoch reused or rejoin failed")}
 qaMessage(t,s,c,o,30);qaMessage(t,s,c,o,31)
 rows,e:=s.ReadHistory(ctx,c,v,0,50);if e!=nil{t.Fatal(e)}
 expected:=[]int64{10,11,12,13,14,15,16,17,18,19,30,31};if len(rows)!=12{t.Fatalf("visible=%d",len(rows))}
 for i,n:=range expected{if rows[i].Ordinal!=n{t.Fatalf("history[%d]=%d expected %d",i,rows[i].Ordinal,n)}}
}
func TestIndependentMS2504SendRemoveBothOrders(t *testing.T){
 for _,first:=range []bool{true,false}{t.Run(fmt.Sprint("sendFirst=",first),func(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o,v)
 for i:=int64(1);i<=39;i++{qaMessage(t,s,c,o,i)}
 if first{qaMessage(t,s,c,v,40)}
 if e:=s.Remove(ctx,c,o,v);e!=nil{t.Fatal(e)}
 var boundary int64;if e:=db.QueryRowContext(ctx,"SELECT leave_boundary FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2",c,v).Scan(&boundary);e!=nil{t.Fatal(e)}
 want:=int64(40);if first{want=41};if boundary!=want{t.Fatalf("boundary=%d want=%d",boundary,want)}
 if _,e:=s.Send(ctx,c,v,fixtureID(t),Payload{Text:"denied"});!errors.Is(e,ErrForbidden){t.Fatalf("revoked send=%v",e)}
 if !first{qaMessage(t,s,c,o,40)}
 var next int64;if e:=db.QueryRowContext(ctx,"SELECT next_ordinal FROM issue25_conversations WHERE id=$1",c).Scan(&next);e!=nil||next!=41{t.Fatalf("next=%d %v",next,e)}
 })}
}
func TestIndependentMS2505OwnerTransfer(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,n:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o,n)
 if e:=s.Remove(ctx,c,o,o);!errors.Is(e,ErrForbidden){t.Fatalf("owner leave before transfer=%v",e)}
 if e:=s.TransferOwner(ctx,c,o,n);e!=nil{t.Fatal(e)}
 if e:=s.Remove(ctx,c,o,o);e!=nil{t.Fatal(e)}
 var owner string;if e:=db.QueryRowContext(ctx,"SELECT account_id::text FROM issue25_memberships WHERE conversation_id=$1 AND role='owner' AND leave_boundary IS NULL",c).Scan(&owner);e!=nil||owner!=n{t.Fatalf("owner=%s %v",owner,e)}
}
func TestIndependentMS2506Idempotency(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o:=fixtureID(t);c:=qaGroup(t,s,o);k:=fixtureID(t)
 a,e:=s.Send(ctx,c,o,k,Payload{Text:"abc"});if e!=nil{t.Fatal(e)}
 b,e:=s.Send(ctx,c,o,k,Payload{Text:"abc"});if e!=nil||!b.Replayed||b.ID!=a.ID{t.Fatalf("replay=%+v %v",b,e)}
 if _,e:=s.Send(ctx,c,o,k,Payload{Text:"abd"});!errors.Is(e,ErrConflict){t.Fatalf("409=%v",e)}
}
func TestIndependentMS2507Cursor(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o:=fixtureID(t);c:=qaGroup(t,s,o)
 for i:=int64(1);i<=72;i++{qaMessage(t,s,c,o,i)}
 for _,v:=range []struct{after int64;want int64}{{70,71},{71,72},{72,0}}{
 rows,e:=s.ReadHistory(ctx,c,o,v.after,1);if e!=nil{t.Fatal(e)}
 if v.want==0{if len(rows)!=0{t.Fatal("cursor returned extra row")}}else if len(rows)!=1||rows[0].Ordinal!=v.want{t.Fatalf("cursor after %d: %+v",v.after,rows)}
 }
}
func qaAttempt(t *testing.T,s Store,c,o,v string)(Attempt,Message){
 t.Helper();ctx:=context.Background();m:=qaMessage(t,s,c,o,1);var epoch string
 if e:=s.DB.QueryRowContext(ctx,"SELECT epoch::text FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL",c,v).Scan(&epoch);e!=nil{t.Fatal(e)}
 a:=Attempt{ID:fixtureID(t),Conversation:c,Viewer:v,Epoch:epoch,MessageID:m.ID,Ordinal:m.Ordinal,WakeToken:fixtureID(t),Channel:"chat:e:"+epoch}
 if e:=s.RegisterAttempt(ctx,a);e!=nil{t.Fatal(e)};return a,m
}
func TestIndependentMS2508ImmutableAttempt(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o,v);a,_:=qaAttempt(t,s,c,o,v)
 if e:=s.Remove(ctx,c,o,v);e!=nil{t.Fatal(e)};m:=qaMessage(t,s,c,o,2)
 if _,e:=db.ExecContext(ctx,"UPDATE issue25_publish_attempts SET message_id=$1 WHERE attempt_id=$2",m.ID,a.ID);e==nil{t.Fatal("attempt rebinding allowed")}
 a.ID=fixtureID(t);a.MessageID=m.ID;a.Ordinal=m.Ordinal
 if e:=s.RegisterAttempt(ctx,a);!errors.Is(e,ErrForbidden){t.Fatalf("retired epoch registration=%v",e)}
}
func TestIndependentMS2509AtomicClaim(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o,v);a,_:=qaAttempt(t,s,c,o,v)
 var wg sync.WaitGroup;ok:=make(chan bool,8);errs:=make(chan error,8)
 for i:=0;i<8;i++{wg.Add(1);go func(){defer wg.Done();v,e:=s.ClaimAttempt(ctx,a.ID);ok<-v;errs<-e}()}
 wg.Wait();close(ok);close(errs);for e:=range errs{if e!=nil{t.Fatal(e)}}
 n:=0;for x:=range ok{if x{n++}};if n!=1{t.Fatalf("successful claims=%d",n)}
}
func TestIndependentMS2510UnknownNoRetry(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o,v);a,_:=qaAttempt(t,s,c,o,v)
 if ok,e:=s.ClaimAttempt(ctx,a.ID);e!=nil||!ok{t.Fatalf("claim: %v %v",ok,e)}
 if e:=s.SetAttemptOutcome(ctx,a.ID,"UNKNOWN");e!=nil{t.Fatal(e)}
 if ok,e:=s.ClaimAttempt(ctx,a.ID);e!=nil||ok{t.Fatalf("UNKNOWN replay allowed: %v %v",ok,e)}
}
func TestIndependentMS2511RevokedChannel(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o,v)
 var epoch string;if e:=db.QueryRowContext(ctx,"SELECT epoch::text FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL",c,v).Scan(&epoch);e!=nil{t.Fatal(e)}
 old:="chat:e:"+epoch;if e:=s.AuthorizeEpochChannel(ctx,old,v);e!=nil{t.Fatal(e)}
 if e:=s.Remove(ctx,c,o,v);e!=nil{t.Fatal(e)}
 if e:=s.AuthorizeEpochChannel(ctx,old,v);!errors.Is(e,ErrForbidden){t.Fatalf("retired channel=%v",e)}
 fresh,e:=s.Join(ctx,c,o,v,"member");if e!=nil||fresh==epoch{t.Fatalf("fresh epoch %s %v",fresh,e)}
 if e:=s.AuthorizeEpochChannel(ctx,old,v);!errors.Is(e,ErrForbidden){t.Fatalf("old channel revived=%v",e)}
}
func TestIndependentMS2512Unread(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o,v)
 qaMessage(t,s,c,o,1);qaMessage(t,s,c,v,2);qaMessage(t,s,c,o,3)
 if n,e:=s.Unread(ctx,c,v);e!=nil||n!=2{t.Fatalf("unread=%d %v",n,e)}
 if n,e:=s.MarkRead(ctx,c,v,1);e!=nil||n!=1{t.Fatalf("marker=%d %v",n,e)}
 if n,e:=s.MarkRead(ctx,c,v,0);e!=nil||n!=1{t.Fatalf("stale marker=%d %v",n,e)}
}
func TestIndependentMS2513NonMemberDenied(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t);c:=qaGroup(t,s,o);qaMessage(t,s,c,o,1)
 if _,e:=s.ReadHistoryDetailed(ctx,c,v,0,50);!errors.Is(e,ErrForbidden){t.Fatalf("nonmember history=%v",e)}
 if e:=s.AuthorizeEpochChannel(ctx,"chat:e:"+fixtureID(t),v);!errors.Is(e,ErrForbidden){t.Fatalf("nonmember subscribe=%v",e)}
}
func TestIndependentMS2514LegacyUnchanged(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o,v:=fixtureID(t),fixtureID(t)
 if _,e:=db.ExecContext(ctx,"INSERT INTO chat_assignments(owner_account_id,recipient_account_id) VALUES($1,$2)",o,v);e!=nil{t.Fatal(e)}
 if _,e:=db.ExecContext(ctx,"INSERT INTO chat_conversations(owner_account_id) VALUES($1)",o);e!=nil{t.Fatal(e)}
 key:=fixtureID(t);if _,e:=db.ExecContext(ctx,"INSERT INTO chat_messages(owner_account_id,sender_account_id,recipient_account_id,idempotency_key,body) VALUES($1,$1,$2,$3,'qa-legacy')",o,v,key);e!=nil{t.Fatal(e)}
 user:=fixtureID(t);c:=qaGroup(t,s,user);qaMessage(t,s,c,user,1)
 var text string;if e:=db.QueryRowContext(ctx,"SELECT body FROM chat_messages WHERE owner_account_id=$1 AND idempotency_key=$2",o,key).Scan(&text);e!=nil||text!="qa-legacy"{t.Fatalf("legacy mutated %q %v",text,e)}
}
func TestIndependentMS2515InvalidWritesAtomic(t *testing.T){
 db:=integrationDB(t);s:=Store{DB:db};ctx:=context.Background();o:=fixtureID(t);c:=qaGroup(t,s,o)
 for _,p:=range []Payload{{Text:strings.Repeat("a",16385)},{Text:"ok",DocumentID:ptr("doc")},{Text:" "}}{
 if _,e:=s.Send(ctx,c,o,fixtureID(t),p);!errors.Is(e,ErrInvalid){t.Fatalf("invalid payload accepted: %v",e)}
 }
 var n int;if e:=db.QueryRowContext(ctx,"SELECT count(*) FROM issue25_messages WHERE conversation_id=$1",c).Scan(&n);e!=nil||n!=0{t.Fatalf("partial message writes=%d %v",n,e)}
}

package issue25

import (
 "context"
 "database/sql"
 "errors"
 "os"
 "sync"
 "testing"

 _ "github.com/jackc/pgx/v5/stdlib"
)

func integrationDB(t *testing.T)*sql.DB{
 t.Helper()
 dsn:=os.Getenv("BFE_CHAT_TEST_DATABASE_URL")
 if dsn==""{t.Skip("BFE_CHAT_TEST_DATABASE_URL not configured")}
 db,err:=sql.Open("pgx",dsn);if err!=nil{t.Fatal(err)}
 t.Cleanup(func(){_ =db.Close()})
 if err=db.PingContext(context.Background());err!=nil{t.Fatal(err)}
 return db
}
func fixtureID(t *testing.T)string{t.Helper();id,err:=newUUID();if err!=nil{t.Fatal(err)};return id}
func TestPGDirectConcurrentCreate(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 a,b:=fixtureID(t),fixtureID(t)
 results:=make(chan string,2);errs:=make(chan error,2)
 var wg sync.WaitGroup
 for _,pair:=range [][2]string{{a,b},{b,a}}{
  wg.Add(1);go func(p [2]string){defer wg.Done();id,err:=s.OpenDirect(ctx,p[0],p[1]);results<-id;errs<-err}(pair)
 }
 wg.Wait();close(results);close(errs)
 for err:=range errs{if err!=nil{t.Fatalf("concurrent direct: %v",err)}}
 ids:=[]string{};for id:=range results{ids=append(ids,id)}
 if len(ids)!=2||ids[0]!=ids[1]{t.Fatalf("direct ids: %v",ids)}
 var n int
 if err:=db.QueryRowContext(ctx,`SELECT count(*) FROM issue25_conversations WHERE kind='direct' AND id=$1`,ids[0]).Scan(&n);err!=nil||n!=1{t.Fatalf("direct count %d: %v",n,err)}
 if err:=db.QueryRowContext(ctx,`SELECT count(*) FROM issue25_memberships WHERE conversation_id=$1 AND leave_boundary IS NULL`,ids[0]).Scan(&n);err!=nil||n!=2{t.Fatalf("direct members %d: %v",n,err)}
}
func TestPGSendRemoveSerialOrders(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,member:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"fixture",[]string{member});if err!=nil{t.Fatal(err)}
 first,err:=s.Send(ctx,group,member,fixtureID(t),Payload{Text:"before remove"});if err!=nil{t.Fatal(err)}
 if first.Ordinal!=1{t.Fatalf("first ordinal %d",first.Ordinal)}
 if err=s.Remove(ctx,group,owner,member);err!=nil{t.Fatal(err)}
 var leave int64
 if err=db.QueryRowContext(ctx,`SELECT leave_boundary FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2`,group,member).Scan(&leave);err!=nil{t.Fatal(err)}
 if leave!=2{t.Fatalf("leaveBoundary=%d expected 2",leave)}
 _,err=s.Send(ctx,group,member,fixtureID(t),Payload{Text:"after remove"})
 if !errors.Is(err,ErrForbidden){t.Fatalf("removed sender err=%v",err)}
 next,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:"owner after remove"});if err!=nil{t.Fatal(err)}
 if next.Ordinal!=2{t.Fatalf("post-removal ordinal=%d",next.Ordinal)}
 visible,err:=s.ReadHistory(ctx,group,member,0,50);if err!=nil{t.Fatal(err)}
 if len(visible)!=1||visible[0].Ordinal!=1{t.Fatalf("history=%+v",visible)}
 var counter int64
 if err=db.QueryRowContext(ctx,`SELECT next_ordinal FROM issue25_conversations WHERE id=$1`,group).Scan(&counter);err!=nil{t.Fatal(err)}
 if counter!=3{t.Fatalf("counter=%d",counter)}
}
func TestPGImmutableAttemptAndSingleClaim(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,viewer:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"attempt fixture",[]string{viewer});if err!=nil{t.Fatal(err)}
 m1,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:"M1"});if err!=nil{t.Fatal(err)}
 epoch:= "";err=db.QueryRowContext(ctx,`SELECT epoch::text FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,group,viewer).Scan(&epoch)
 if err!=nil{t.Fatal(err)}
 id,channel,token:=fixtureID(t),fixtureID(t),fixtureID(t)
 a:=Attempt{ID:id,Conversation:group,Viewer:viewer,Epoch:epoch,MessageID:m1.ID,Ordinal:m1.Ordinal,Channel:channel,WakeToken:token}
 if err=s.RegisterAttempt(ctx,a);err!=nil{t.Fatalf("register M1: %v",err)}
 if err=s.Remove(ctx,group,owner,viewer);err!=nil{t.Fatal(err)}
 m2,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:"M2"});if err!=nil{t.Fatal(err)}
 if m2.Ordinal<=m1.Ordinal{t.Fatalf("bad ordinal M1=%d M2=%d",m1.Ordinal,m2.Ordinal)}
 // An existing attempt cannot mutate its binding to the post-removal message.
 _,err=db.ExecContext(ctx,`UPDATE issue25_publish_attempts SET message_id=$1,message_ordinal=$2 WHERE attempt_id=$3`,m2.ID,m2.Ordinal,id)
 if err==nil{t.Fatal("mutable attempt allowed M2 rebind")}
 // Nor can the retired epoch register a new attempt for M2.
 a.ID=fixtureID(t);a.MessageID=m2.ID;a.Ordinal=m2.Ordinal
 if err=s.RegisterAttempt(ctx,a);err==nil{t.Fatal("retired epoch admitted M2")}
 const workers=8
 var wg sync.WaitGroup;results:=make(chan bool,workers);errCh:=make(chan error,workers)
 for i:=0;i<workers;i++{wg.Add(1);go func(){defer wg.Done();ok,e:=s.ClaimAttempt(ctx,id);results<-ok;errCh<-e}()}
 wg.Wait();close(results);close(errCh)
 count:=0;for e:=range errCh{if e!=nil{t.Fatalf("claim: %v",e)}};for ok:=range results{if ok{count++}}
 if count!=1{t.Fatalf("claims=%d, expected 1",count)}
 var messageID string
 if err=db.QueryRowContext(ctx,`SELECT message_id::text FROM issue25_publish_attempts WHERE attempt_id=$1`,id).Scan(&messageID);err!=nil{t.Fatal(err)}
 if messageID!=m1.ID{t.Fatalf("attempt changed to %s",messageID)}
}

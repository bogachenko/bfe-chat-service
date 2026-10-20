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
 db,err:=sql.Open("pgx",dsn);qaRequire(t, !(err!=nil), err)
 t.Cleanup(func(){_ =db.Close()})
 func() { err=db.PingContext(context.Background()); qaRequire(t, !(err!=nil), err) }()
 return db
}
func fixtureID(t *testing.T)string{t.Helper();id,err:=newUUID();qaRequire(t, !(err!=nil), err);return id}
func TestPGDirectConcurrentCreate(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 a,b:=fixtureID(t),fixtureID(t)
 results:=make(chan string,2);errs:=make(chan error,2)
 var wg sync.WaitGroup
 for _,pair:=range [][2]string{{a,b},{b,a}}{
  wg.Add(1);go func(p [2]string){defer wg.Done();id,err:=s.OpenDirect(ctx,p[0],p[1]);results<-id;errs<-err}(pair)
 }
 wg.Wait();close(results);close(errs)
 for err:=range errs{qaRequiref(t, !(err!=nil), "concurrent direct: %v",err)}
 ids:=[]string{};for id:=range results{ids=append(ids,id)}
 qaRequiref(t, !(len(ids)!=2||ids[0]!=ids[1]), "direct ids: %v",ids)
 var n int
 func() { err:=db.QueryRowContext(ctx,`SELECT count(*) FROM issue25_conversations WHERE kind='direct' AND id=$1`,ids[0]).Scan(&n); qaRequiref(t, !(err!=nil||n!=1), "direct count %d: %v",n,err) }()
 func() { err:=db.QueryRowContext(ctx,`SELECT count(*) FROM issue25_memberships WHERE conversation_id=$1 AND leave_boundary IS NULL`,ids[0]).Scan(&n); qaRequiref(t, !(err!=nil||n!=2), "direct members %d: %v",n,err) }()
}
func TestPGSendRemoveSerialOrders(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,member:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"fixture",[]string{member});qaRequire(t, !(err!=nil), err)
 first,err:=s.Send(ctx,group,member,fixtureID(t),Payload{Text:"before remove"});qaRequire(t, !(err!=nil), err)
 qaRequiref(t, !(first.Ordinal!=1), "first ordinal %d",first.Ordinal)
 func() { err=s.Remove(ctx,group,owner,member); qaRequire(t, !(err!=nil), err) }()
 var leave int64
 func() { err=db.QueryRowContext(ctx,`SELECT leave_boundary FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2`,group,member).Scan(&leave); qaRequire(t, !(err!=nil), err) }()
 qaRequiref(t, !(leave!=2), "leaveBoundary=%d expected 2",leave)
 _,err=s.Send(ctx,group,member,fixtureID(t),Payload{Text:"after remove"})
 qaRequiref(t, !(!errors.Is(err,ErrForbidden)), "removed sender err=%v",err)
 next,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:"owner after remove"});qaRequire(t, !(err!=nil), err)
 qaRequiref(t, !(next.Ordinal!=2), "post-removal ordinal=%d",next.Ordinal)
 _,err=s.ReadHistory(ctx,group,member,0,50)
 qaRequiref(t, !(!errors.Is(err,ErrForbidden)), "revoked history must deny: %v",err)
 func() { _,err=s.Join(ctx,group,owner,member,"member"); qaRequire(t, !(err!=nil), err) }()
 visible,err:=s.ReadHistory(ctx,group,member,0,50);qaRequire(t, !(err!=nil), err)
 qaRequiref(t, !(len(visible)!=1||visible[0].Ordinal!=1), "history after rejoin=%+v",visible)
 var counter int64
 func() { err=db.QueryRowContext(ctx,`SELECT next_ordinal FROM issue25_conversations WHERE id=$1`,group).Scan(&counter); qaRequire(t, !(err!=nil), err) }()
 qaRequiref(t, !(counter!=3), "counter=%d",counter)
}
func TestPGImmutableAttemptAndSingleClaim(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,viewer:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"attempt fixture",[]string{viewer});qaRequire(t, !(err!=nil), err)
 m1,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:"M1"});qaRequire(t, !(err!=nil), err)
 epoch:= "";err=db.QueryRowContext(ctx,`SELECT epoch::text FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,group,viewer).Scan(&epoch)
 qaRequire(t, !(err!=nil), err)
 id,token:=fixtureID(t),fixtureID(t)
 channel:="chat:e:"+epoch
 a:=Attempt{ID:id,Conversation:group,Viewer:viewer,Epoch:epoch,MessageID:m1.ID,Ordinal:m1.Ordinal,Channel:channel,WakeToken:token}
 func() { err=s.RegisterAttempt(ctx,a); qaRequiref(t, !(err!=nil), "register M1: %v",err) }()
 func() { err=s.Remove(ctx,group,owner,viewer); qaRequire(t, !(err!=nil), err) }()
 m2,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:"M2"});qaRequire(t, !(err!=nil), err)
 qaRequiref(t, !(m2.Ordinal<=m1.Ordinal), "bad ordinal M1=%d M2=%d",m1.Ordinal,m2.Ordinal)
 // An existing attempt cannot mutate its binding to the post-removal message.
 _,err=db.ExecContext(ctx,`UPDATE issue25_publish_attempts SET message_id=$1,message_ordinal=$2 WHERE attempt_id=$3`,m2.ID,m2.Ordinal,id)
 qaRequire(t, !(err==nil), "mutable attempt allowed M2 rebind")
 // Nor can the retired epoch register a new attempt for M2.
 a.ID=fixtureID(t);a.MessageID=m2.ID;a.Ordinal=m2.Ordinal
 func() { err=s.RegisterAttempt(ctx,a); qaRequire(t, !(err==nil), "retired epoch admitted M2") }()
 const workers=8
 var wg sync.WaitGroup;results:=make(chan bool,workers);errCh:=make(chan error,workers)
 for i:=0;i<workers;i++{wg.Add(1);go func(){defer wg.Done();ok,e:=s.ClaimAttempt(ctx,id);results<-ok;errCh<-e}()}
 wg.Wait();close(results);close(errCh)
 count:=0;for e:=range errCh{qaRequiref(t, !(e!=nil), "claim: %v",e)};for ok:=range results{if ok{count++}}
 qaRequiref(t, !(count!=1), "claims=%d, expected 1",count)
 var messageID string
 func() { err=db.QueryRowContext(ctx,`SELECT message_id::text FROM issue25_publish_attempts WHERE attempt_id=$1`,id).Scan(&messageID); qaRequire(t, !(err!=nil), err) }()
 qaRequiref(t, !(messageID!=m1.ID), "attempt changed to %s",messageID)
}

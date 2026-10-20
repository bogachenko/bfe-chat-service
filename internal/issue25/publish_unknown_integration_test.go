package issue25

import (
 "context"
 "encoding/json"
 "errors"
 "testing"
 "time"
 "sync/atomic"

 "github.com/centrifugal/centrifuge"
)

type blockedPublish struct{
 started chan struct{}
 unblock chan struct{}
 count atomic.Int32
 channel string
 payload []byte
}
func (p *blockedPublish) Publish(channel string,payload []byte,_ ...centrifuge.PublishOption)(centrifuge.PublishResult,error){
 p.count.Add(1)
 p.channel=channel;p.payload=append([]byte(nil),payload...)
 close(p.started)
 <-p.unblock
 return centrifuge.PublishResult{},nil
}
func TestPGMS2510PublishTimeoutUnknownWithoutRetry(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,viewer:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"unknown",[]string{viewer});if err!=nil{t.Fatal(err)}
 msg,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:"M1"});if err!=nil{t.Fatal(err)}
 var epoch string
 if err=db.QueryRowContext(ctx,`SELECT epoch::text FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,group,viewer).Scan(&epoch);err!=nil{t.Fatal(err)}
 id:=fixtureID(t)
 a:=Attempt{ID:id,Conversation:group,Viewer:viewer,Epoch:epoch,MessageID:msg.ID,Ordinal:msg.Ordinal,WakeToken:fixtureID(t),Channel:"chat:e:"+epoch}
 if err=s.RegisterAttempt(ctx,a);err!=nil{t.Fatal(err)}
 broker:=&blockedPublish{started:make(chan struct{}),unblock:make(chan struct{})}
 publishCtx,cancel:=context.WithCancel(ctx)
 done:=make(chan Admission,1)
 go func(){outcome,_:=s.PublishAttempt(publishCtx,id,broker);done<-outcome}()
 select{case <-broker.started:case <-time.After(10*time.Second):t.Fatal("broker call not reached")}
 cancel()
 select{case outcome:=<-done:if outcome!=AdmissionUnknown{t.Fatalf("outcome %s",outcome)};case <-time.After(10*time.Second):t.Fatal("timeout result blocked")}
 if err=s.Remove(ctx,group,owner,viewer);err!=nil{t.Fatal(err)}
 close(broker.unblock)
 // The old attempt cannot be claimed again, including after its publish completes.
 if _,err=s.PublishAttempt(ctx,id,broker);!errors.Is(err,ErrAlreadyClaimed){t.Fatalf("replay after UNKNOWN: %v",err)}
 if broker.count.Load()!=1{t.Fatalf("publish invocations=%d",broker.count.Load())}
 var payload map[string]any
 if err=json.Unmarshal(broker.payload,&payload);err!=nil{t.Fatal(err)}
 if len(payload)!=2||payload["version"]!=float64(1)||payload["wakeToken"]!=a.WakeToken{t.Fatalf("payload=%v",payload)}
 var storedState string
 if err=db.QueryRowContext(ctx,`SELECT state FROM issue25_publish_attempts WHERE attempt_id=$1`,id).Scan(&storedState);err!=nil{t.Fatal(err)}
 if storedState!="UNKNOWN"{t.Fatalf("stored state=%s",storedState)}
}

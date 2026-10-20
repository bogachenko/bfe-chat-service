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
 group,err:=s.CreateGroup(ctx,owner,"unknown",[]string{viewer});qaReject(t, (err!=nil), err)
 msg,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:"M1"});qaReject(t, (err!=nil), err)
 var epoch string
 func() { err=db.QueryRowContext(ctx,`SELECT epoch::text FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,group,viewer).Scan(&epoch); qaReject(t, (err!=nil), err) }()
 id:=fixtureID(t)
 a:=Attempt{ID:id,Conversation:group,Viewer:viewer,Epoch:epoch,MessageID:msg.ID,Ordinal:msg.Ordinal,WakeToken:fixtureID(t),Channel:"chat:e:"+epoch}
 func() { err=s.RegisterAttempt(ctx,a); qaReject(t, (err!=nil), err) }()
 broker:=&blockedPublish{started:make(chan struct{}),unblock:make(chan struct{})}
 publishCtx,cancel:=context.WithCancel(ctx)
 done:=make(chan Admission,1)
 go func(){outcome,_:=s.PublishAttempt(publishCtx,id,broker);done<-outcome}()
 select{case <-broker.started:case <-time.After(10*time.Second):t.Fatal("broker call not reached")}
 cancel()
 select{case outcome:=<-done:qaRejectf(t, (outcome!=AdmissionUnknown), "outcome %s",outcome);case <-time.After(10*time.Second):t.Fatal("timeout result blocked")}
 func() { err=s.Remove(ctx,group,owner,viewer); qaReject(t, (err!=nil), err) }()
 close(broker.unblock)
 // The old attempt cannot be claimed again, including after its publish completes.
 func() { _,err=s.PublishAttempt(ctx,id,broker); qaRejectf(t, (!errors.Is(err,ErrAlreadyClaimed)), "replay after UNKNOWN: %v",err) }()
 qaRejectf(t, (broker.count.Load()!=1), "publish invocations=%d",broker.count.Load())
 var payload map[string]any
 func() { err=json.Unmarshal(broker.payload,&payload); qaReject(t, (err!=nil), err) }()
 qaRejectf(t, (len(payload)!=2||payload["version"]!=float64(1)||payload["wakeToken"]!=a.WakeToken), "payload=%v",payload)
 var storedState string
 func() { err=db.QueryRowContext(ctx,`SELECT state FROM issue25_publish_attempts WHERE attempt_id=$1`,id).Scan(&storedState); qaReject(t, (err!=nil), err) }()
 qaRejectf(t, (storedState!="UNKNOWN"), "stored state=%s",storedState)
}

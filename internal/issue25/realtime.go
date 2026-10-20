package issue25

import (
 "context"
 "encoding/json"
 "errors"
 "time"
 "github.com/centrifugal/centrifuge"
)
type Publisher interface{Publish(string,[]byte,...centrifuge.PublishOption)(centrifuge.PublishResult,error)}
type Admission string
const (AdmissionSuccess Admission="SUCCESS";AdmissionUnknown Admission="UNKNOWN";AdmissionDenied Admission="DENIED";AdmissionAlreadyClaimed Admission="ALREADY_CLAIMED")
type Notification struct{Version int `json:"version"`;WakeToken string `json:"wakeToken"`}
// PublishAttempt consumes one immutable attempt. Timeout is UNKNOWN, never absence of admission.
// The broker call may finish after return; its envelope has no protected content.
func (s Store) PublishAttempt(ctx context.Context,id string,p Publisher)(Admission,error){
 if p==nil{return AdmissionDenied,ErrInvalid}
 attempt,ok,err:=s.ClaimAndGet(ctx,id)
 if err!=nil{return AdmissionDenied,err}
 if !ok{return AdmissionAlreadyClaimed,ErrAlreadyClaimed}
 payload,err:=json.Marshal(Notification{Version:1,WakeToken:attempt.WakeToken})
 if err!=nil{return AdmissionUnknown,err}
 outcome:=make(chan error,1)
 go func(){_,e:=p.Publish(attempt.Channel,payload);outcome<-e}()
 select {
 case e:=<-outcome:
  if e!=nil{
   // Broker error is ambiguous: the publish could already have been admitted.
   _=s.SetAttemptOutcome(context.Background(),id,"UNKNOWN")
   return AdmissionUnknown,e
  }
  _=s.SetAttemptOutcome(context.Background(),id,"SUCCESS")
  return AdmissionSuccess,nil
 case <-ctx.Done():
  // No attempt replay; goroutine may still finish after REMOVE.
  save,stop:=context.WithTimeout(context.Background(),2*time.Second)
  defer stop()
  _=s.SetAttemptOutcome(save,id,"UNKNOWN")
  return AdmissionUnknown,ctx.Err()
 }
}
func OpaqueChannel()(string,error){token,err:=newUUID();if err!=nil{return "",err};return "chat:e:"+token,nil}
func WakeToken()(string,error){return newUUID()}
var ErrStaleEpoch=errors.New("STALE_EPOCH")

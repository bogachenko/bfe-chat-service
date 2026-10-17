//go:build integration

package storage_test

import (
 "context"
 "errors"
 "sync"
 "testing"
 "time"

 "github.com/bogachenko/bfe-chat-service/internal/authentication"
 "github.com/bogachenko/bfe-chat-service/internal/authorization"
 "github.com/bogachenko/bfe-chat-service/internal/chat"
)
func TestDPM04MSF02ReassignmentDeliverySerialization(t *testing.T){
 db,s:=database(t)
 _=db
 assign(t,s,recipientA)
 a:=authorization.Authorizer{Assignments:s}
 current:=authentication.Caller{AccountID:recipientA}
 ownerID:=chat.AccountID(owner)
 entered:=make(chan struct{})
 release:=make(chan struct{})
 delivered:=make(chan error,1)
 go func(){
  delivered<-a.AdmitDelivery(context.Background(),current,ownerID,func()error{
   close(entered)
   <-release
   return nil
  })
 }()
 select{case <-entered:case <-time.After(15*time.Second):t.Fatal("initial delivery not admitted")}
 replacement:=make(chan error,1)
 go func(){replacement<-s.CommitAssignment(context.Background(),owner,recipientB)}()
 select{
 case err:=<-replacement:t.Fatalf("reassignment completed while delivery lock held: %v",err)
 case <-time.After(200*time.Millisecond):
 }
 close(release)
 select{case err:=<-delivered:if err!=nil{t.Fatal(err)};case <-time.After(15*time.Second):t.Fatal("delivery deadlocked")}
 select{case err:=<-replacement:if err!=nil{t.Fatal(err)};case <-time.After(15*time.Second):t.Fatal("reassignment deadlocked")}
 called:=false
 err:=a.AdmitDelivery(context.Background(),current,ownerID,func()error{called=true;return nil})
 if !errors.Is(err,authorization.ErrForbidden)||called{t.Fatalf("stale recipient delivered after reassignment: err=%v called=%v",err,called)}
 current.AccountID=recipientB
 if err:=a.AdmitDelivery(context.Background(),current,ownerID,func()error{called=true;return nil});err!=nil||!called{t.Fatalf("new recipient denied: %v",err)}
}
func TestDPM04MSF02FailedDeliveryNoPartialWrites(t *testing.T){
 db,s:=database(t)
 assign(t,s,recipientA)
 a:=authorization.Authorizer{Assignments:s}
 ownerID:=chat.AccountID(owner)
 publishFailure:=errors.New("publication failed")
 err:=a.AdmitDelivery(context.Background(),authentication.Caller{AccountID:recipientA},ownerID,func()error{return publishFailure})
 if !errors.Is(err,publishFailure){t.Fatalf("lost publish error: %v",err)}
 if rows(t,db,"chat_messages")!=0||rows(t,db,"chat_conversations")!=0{t.Fatal("failed admission mutated messages")}
 assign(t,s,recipientB)
 invoked:=false
 err=a.AdmitDelivery(context.Background(),authentication.Caller{AccountID:recipientA},ownerID,func()error{invoked=true;return nil})
 if !errors.Is(err,authorization.ErrForbidden)||invoked{t.Fatal("old recipient admitted after failed publish")}
}
func TestDPM04MSF02ConcurrentReassignmentAndDelivery(t *testing.T){
 _,s:=database(t)
 assign(t,s,recipientA)
 a:=authorization.Authorizer{Assignments:s}
 ownerID:=chat.AccountID(owner)
 var wg sync.WaitGroup
 start:=make(chan struct{})
 const count=12
 errorsCh:=make(chan error,count)
 for i:=0;i<count;i++{
  wg.Add(2)
  go func(i int){defer wg.Done();<-start;to:=recipientA;if i%2!=0{to=recipientB};if err:=s.CommitAssignment(context.Background(),owner,to);err!=nil{errorsCh<-err}}(i)
  go func(i int){defer wg.Done();<-start;id:=recipientA;if i%2!=0{id=recipientB};err:=a.AdmitDelivery(context.Background(),authentication.Caller{AccountID:id},ownerID,func()error{return nil});if err!=nil&&!errors.Is(err,authorization.ErrForbidden){errorsCh<-err}}(i)
 }
 close(start);wg.Wait();close(errorsCh)
 for err:=range errorsCh{t.Fatal(err)}
 current,err:=s.GetAssignment(context.Background(),owner)
 if err!=nil{t.Fatal(err)}
 stale:=recipientA;if current==recipientA{stale=recipientB}
 if err:=a.AdmitDelivery(context.Background(),authentication.Caller{AccountID:stale},ownerID,func()error{t.Error("stale delivery executed");return nil});!errors.Is(err,authorization.ErrForbidden){t.Fatalf("stale recipient accepted: %v",err)}
}

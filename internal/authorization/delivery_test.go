package authorization_test

import (
 "context"
 "errors"
 "testing"

 "github.com/bogachenko/bfe-chat-service/internal/authorization"
 "github.com/bogachenko/bfe-chat-service/internal/chat"
)
type lockedStore struct{
 recipient string
 err error
 called int
}
func(s *lockedStore)GetAssignment(context.Context,string)(string,error){return s.recipient,s.err}
func(s *lockedStore)WithAssignmentLock(_ context.Context,_ string,fn func(string)error)error{
 s.called++
 if s.err!=nil{return s.err}
 return fn(s.recipient)
}
func TestFC07AdmitDeliveryAuthorizedAndReassignment(t *testing.T){
 s:=&lockedStore{recipient:string(first)}
 a:=authorization.Authorizer{Assignments:s}
 calls:=0
 publish:=func()error{calls++;return nil}
 if err:=a.AdmitDelivery(context.Background(),caller(first,""),owner,publish);err!=nil{t.Fatal(err)}
 s.recipient=string(second)
 if err:=a.AdmitDelivery(context.Background(),caller(first,""),owner,publish);!errors.Is(err,authorization.ErrForbidden){t.Fatalf("stale recipient: %v",err)}
 if calls!=1{t.Fatalf("stale publish happened %d",calls)}
 if err:=a.AdmitDelivery(context.Background(),caller(second,""),owner,publish);err!=nil{t.Fatal(err)}
 if err:=a.AdmitDelivery(context.Background(),caller(owner,""),owner,publish);err!=nil{t.Fatal(err)}
 if calls!=3||s.called!=4{t.Fatalf("publish=%d locks=%d",calls,s.called)}
}
func TestFC07AdmitDeliveryFailClosed(t *testing.T){
 for _,tc:=range []struct{name string;id chat.AccountID;owner chat.AccountID;s *lockedStore;want error}{
  {"invalid owner",first,"invalid",&lockedStore{recipient:string(first)},authorization.ErrForbidden},
  {"invalid caller","invalid",owner,&lockedStore{recipient:string(first)},authorization.ErrForbidden},
  {"missing assignment",first,owner,&lockedStore{},authorization.ErrForbidden},
  {"invalid assignment",first,owner,&lockedStore{recipient:"bad"},authorization.ErrForbidden},
  {"wrong recipient",second,owner,&lockedStore{recipient:string(first)},authorization.ErrForbidden},
  {"storage error",first,owner,&lockedStore{recipient:string(first),err:errors.New("database unavailable")},errors.New("database unavailable")},
 }{
  t.Run(tc.name,func(t *testing.T){
   invoked:=false
   err:=(authorization.Authorizer{Assignments:tc.s}).AdmitDelivery(context.Background(),caller(tc.id,""),tc.owner,func()error{invoked=true;return nil})
   if err==nil||invoked{t.Fatalf("fail-open %v invoked=%v",err,invoked)}
  })
 }
}
func TestFC07AdmitDeliveryRequiresLockAndPublication(t *testing.T){
 a:=authorization.Authorizer{Assignments:&store{recipient:string(first)}}
 if err:=a.AdmitDelivery(context.Background(),caller(first,""),owner,func()error{return nil});!errors.Is(err,authorization.ErrForbidden){t.Fatal(err)}
 locked:=authorization.Authorizer{Assignments:&lockedStore{recipient:string(first)}}
 if err:=locked.AdmitDelivery(context.Background(),caller(first,""),owner,nil);!errors.Is(err,authorization.ErrForbidden){t.Fatal(err)}
 fail:=errors.New("publication error")
 err:=locked.AdmitDelivery(context.Background(),caller(first,""),owner,func()error{return fail})
 if !errors.Is(err,fail){t.Fatalf("publish failure swallowed: %v",err)}
}

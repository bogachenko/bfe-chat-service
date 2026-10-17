package authorization_test

import (
 "context"
 "errors"
 "testing"

 "github.com/bogachenko/bfe-chat-service/internal/authentication"
 "github.com/bogachenko/bfe-chat-service/internal/authorization"
 "github.com/bogachenko/bfe-chat-service/internal/chat"
)
const owner=chat.AccountID("11111111-1111-4111-8111-111111111111")
const first=chat.AccountID("22222222-2222-4222-8222-222222222222")
const second=chat.AccountID("33333333-3333-4333-8333-333333333333")
type store struct{recipient string;err error;reads int}
func(s *store)GetAssignment(context.Context,string)(string,error){s.reads++;return s.recipient,s.err}
func caller(id chat.AccountID,role string)authentication.Caller{return authentication.Caller{AccountID:string(id),AdministrativeStatus:role}}
func TestFC07CurrentParticipantsSendRead(t *testing.T){
 for _,action:=range []authorization.Operation{authorization.Send,authorization.Read,authorization.Subscribe,authorization.Deliver}{
  for _,id:=range []chat.AccountID{owner,first}{
   s:=&store{recipient:string(first)}
   err:=(authorization.Authorizer{Assignments:s}).AuthorizeOperation(context.Background(),caller(id,""),owner,action)
   if err!=nil||s.reads!=1{t.Fatalf("action=%s id=%s err=%v reads=%d",action,id,err,s.reads)}
  }
  s:=&store{recipient:string(first)}
  err:=(authorization.Authorizer{Assignments:s}).AuthorizeOperation(context.Background(),caller(second,""),owner,action)
  if !errors.Is(err,authorization.ErrForbidden){t.Fatalf("outsider allowed %s: %v",action,err)}
 }
}
func TestFC07ReassignmentImmediatelyRevokesOldRecipient(t *testing.T){
 s:=&store{recipient:string(first)};a:=authorization.Authorizer{Assignments:s}
 if err:=a.AuthorizeSend(context.Background(),caller(first,""),owner);err!=nil{t.Fatal(err)}
 s.recipient=string(second)
 for _,action:=range []authorization.Operation{authorization.Send,authorization.Read,authorization.Subscribe,authorization.Deliver}{
  if err:=a.AuthorizeOperation(context.Background(),caller(first,""),owner,action);!errors.Is(err,authorization.ErrForbidden){t.Fatalf("old recipient still allowed %s",action)}
  if err:=a.AuthorizeOperation(context.Background(),caller(second,""),owner,action);err!=nil{t.Fatalf("new recipient denied %s: %v",action,err)}
 }
}
func TestFC07AdminAndAdminReadNoSend(t *testing.T){
 s:=&store{recipient:string(first)};a:=authorization.Authorizer{Assignments:s}
 admin:=caller(second,"ADMIN")
 for _,action:=range []authorization.Operation{authorization.Assign,authorization.AdminRead,authorization.Read}{
  if err:=a.AuthorizeOperation(context.Background(),admin,owner,action);err!=nil{t.Fatalf("admin %s: %v",action,err)}
 }
 if err:=a.AuthorizeSend(context.Background(),admin,owner);!errors.Is(err,authorization.ErrForbidden){t.Fatalf("ADMIN unexpectedly SEND: %v",err)}
 for _,action:=range []authorization.Operation{authorization.Assign,authorization.AdminRead}{
  if err:=a.AuthorizeOperation(context.Background(),caller(second,""),owner,action);!errors.Is(err,authorization.ErrForbidden){t.Fatalf("stale ADMIN accepted: %s",action)}
 }
}
func TestFC07FailClosed(t *testing.T){
 for _,tc:=range []struct{label string;db *store;id chat.AccountID;op authorization.Operation}{
  {"missing assignment",&store{},first,authorization.Send},
  {"store error",&store{err:errors.New("down")},owner,authorization.Read},
  {"bad recipient",&store{recipient:"broken"},owner,authorization.Send},
  {"outsider",&store{recipient:string(first)},second,authorization.Read},
  {"invalid caller",&store{recipient:string(first)},"bad",authorization.Read},
  {"nil storage",nil,owner,authorization.Send},
  {"unknown op",&store{recipient:string(first)},owner,authorization.Operation("ADMIN_DELETE")},
 }{
  t.Run(tc.label,func(t *testing.T){
   var a authorization.Authorizer
   if tc.db!=nil{a.Assignments=tc.db}
   err:=a.AuthorizeOperation(context.Background(),caller(tc.id,""),owner,tc.op)
   if !errors.Is(err,authorization.ErrForbidden){t.Fatalf("fail-open: %v",err)}
  })
 }
}

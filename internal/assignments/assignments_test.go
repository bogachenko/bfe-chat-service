package assignments_test

import (
 "context"
 "errors"
 "testing"
 "github.com/bogachenko/bfe-chat-service/internal/assignments"
 "github.com/bogachenko/bfe-chat-service/internal/authentication"
 "github.com/bogachenko/bfe-chat-service/internal/chat"
)

type authStub struct{role string; err error; calls int}
func(a *authStub)ValidateCaller(context.Context,string,string,authentication.RequestBinding)(authentication.Caller,error){a.calls++;return authentication.Caller{AdministrativeStatus:a.role},a.err}
type storeStub struct{ recipient string; writes int }
func(s *storeStub)CommitAssignment(_ context.Context,_,recipient string)error{s.writes++;s.recipient=recipient;return nil}
func(s *storeStub)GetAssignment(context.Context,string)(string,error){return s.recipient,nil}
func TestFC03OnlyCurrentAdmin(t *testing.T){
 owner:=chat.AccountID("11111111-1111-4111-8111-111111111111")
 recipient:=chat.AccountID("22222222-2222-4222-8222-222222222222")
 for _,role:=range []string{"ADMIN","NON_ADMIN",""}{
  a:=&authStub{role:role};db:=&storeStub{}
  got,err:=(assignments.Service{Auth:a,Store:db}).ReplaceRecipient(context.Background(),"token","proof",authentication.RequestBinding{},owner,recipient)
  if role=="ADMIN"&&(err!=nil||got.Recipient!=recipient||db.writes!=1){t.Fatalf("ADMIN: %+v %v",got,err)}
  if role!="ADMIN"&&(!errors.Is(err,assignments.ErrForbidden)||db.writes!=0){t.Fatalf("role %q: %v writes %d",role,err,db.writes)}
  if a.calls!=1{t.Fatalf("auth calls: %d",a.calls)}
 }
}
func TestFC03InvalidUUIDAndNoAutoAssignment(t *testing.T){
 owner:=chat.AccountID("11111111-1111-4111-8111-111111111111")
 a:=&authStub{role:"ADMIN"};db:=&storeStub{};svc:=assignments.Service{Auth:a,Store:db}
 if _,err:=svc.GetAssignment(context.Background(),owner);err!=nil{t.Fatal(err)}
 if db.writes!=0||a.calls!=0{t.Fatal("implicit assignment or auth status lookup")}
 for _,invalid:=range []chat.AccountID{"","bad","zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz"}{
  if _,err:=svc.ReplaceRecipient(context.Background(),"token","proof",authentication.RequestBinding{},owner,invalid);!errors.Is(err,assignments.ErrInvalidRecipient){t.Fatalf("invalid %q: %v",invalid,err)}
 }
 if db.writes!=0{t.Fatal("invalid recipients persisted")}
}

package assignments_test

import (
 "context"
 "database/sql"
 "errors"
 "testing"

 "github.com/bogachenko/bfe-chat-service/internal/assignments"
 "github.com/bogachenko/bfe-chat-service/internal/authentication"
 "github.com/bogachenko/bfe-chat-service/internal/chat"
)
const qaOwner=chat.AccountID("11111111-1111-4111-8111-111111111111")
const qaRecipient=chat.AccountID("22222222-2222-4222-8222-222222222222")

type failingAuth struct{err error}
func(a failingAuth)ValidateCaller(context.Context,string,string,authentication.RequestBinding)(authentication.Caller,error){return authentication.Caller{},a.err}
type stubStorage struct{recipient string;getErr,putErr error}
func(s *stubStorage)CommitAssignment(_ context.Context,_,recipient string)error{if s.putErr!=nil{return s.putErr};s.recipient=recipient;return nil}
func(s *stubStorage)GetAssignment(context.Context,string)(string,error){return s.recipient,s.getErr}
func TestDP02AuthAndStoreFailures(t *testing.T){
 binding:=authentication.RequestBinding{Method:"POST",Target:"https://chat.test/assignment"}
 for _,authErr:=range []error{authentication.ErrAuthInvalid,authentication.ErrProofInvalid,authentication.ErrAuthUnavailable}{
  svc:=assignments.Service{Auth:failingAuth{err:authErr},Store:&stubStorage{}}
  got,err:=svc.ReplaceRecipient(context.Background(),"opaque-token","proof",binding,qaOwner,qaRecipient)
  if !errors.Is(err,authErr)||got!=(chat.Assignment{}){t.Fatalf("auth=%v caller=%+v err=%v",authErr,got,err)}
 }
 svc:=assignments.Service{Auth:admin(),Store:&stubStorage{}}
 for _,tc:=range []struct{owner,recipient chat.AccountID}{
  {"not-a-uuid",qaRecipient},{qaOwner,"broken"},{qaOwner,""},{qaOwner,"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaZ"},
 }{
  got,err:=svc.ReplaceRecipient(context.Background(),"token","proof",binding,tc.owner,tc.recipient)
  if !errors.Is(err,assignments.ErrInvalidRecipient)||got!=(chat.Assignment{}){t.Fatalf("invalid IDs: %+v err=%v",got,err)}
 }
 for _,tc:=range []struct{err,want error}{
  {pgError("40001"),assignments.ErrAssignmentConflict},
  {pgError("40P01"),assignments.ErrAssignmentConflict},
  {pgError("23505"),assignments.ErrAssignmentConflict},
  {pgError("23503"),assignments.ErrStorageUnavailable},
  {errors.New("database failure"),assignments.ErrStorageUnavailable},
 }{
  svc:=assignments.Service{Auth:admin(),Store:&stubStorage{putErr:tc.err}}
  got,err:=svc.ReplaceRecipient(context.Background(),"token","proof",binding,qaOwner,qaRecipient)
  if !errors.Is(err,tc.want)||got!=(chat.Assignment{}){t.Fatalf("state=%v got=%+v err=%v",tc.err,got,err)}
 }
}
func TestDP02FC04ErrorBranches(t *testing.T){
 for _,tc:=range []struct{owner chat.AccountID;store *stubStorage;want error}{
  {"not-a-uuid",&stubStorage{},assignments.ErrInvalidRecipient},
  {qaOwner,nil,assignments.ErrStorageUnavailable},
  {qaOwner,&stubStorage{getErr:sql.ErrNoRows},assignments.ErrNoAssignment},
  {qaOwner,&stubStorage{getErr:errors.New("failed")},assignments.ErrStorageUnavailable},
  {qaOwner,&stubStorage{recipient:"malformed"},assignments.ErrStorageUnavailable},
 }{
  got,err:=(assignments.Service{Store:tc.store}).GetAssignment(context.Background(),tc.owner)
  if !errors.Is(err,tc.want)||got!=(chat.Assignment{}){t.Fatalf("got=%+v err=%v want=%v",got,err,tc.want)}
 }
 valid:=&stubStorage{recipient:string(qaRecipient)}
 got,err:=(assignments.Service{Store:valid}).GetAssignment(context.Background(),qaOwner)
 if err!=nil||got.Owner!=qaOwner||got.Recipient!=qaRecipient{t.Fatalf("valid assignment=%+v err=%v",got,err)}
}

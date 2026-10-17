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
type failingStore struct{current string; failure error; calls int}
func(s *failingStore)CommitAssignment(context.Context,string,string)error{s.calls++;return s.failure}
func(s *failingStore)GetAssignment(context.Context,string)(string,error){if s.failure!=nil{return "",s.failure};return s.current,nil}
type pgError string
func(e pgError)Error()string{return string(e)}
func(e pgError)SQLState()string{return string(e)}
func TestFC12RejectedReplacementKeepsOriginal(t *testing.T){
 owner:=chat.AccountID("11111111-1111-4111-8111-111111111111")
 recipient:=chat.AccountID("22222222-2222-4222-8222-222222222222")
 for _,tc:=range []struct{failure,want error}{
  {pgError("40001"),assignments.ErrAssignmentConflict},
  {pgError("40P01"),assignments.ErrAssignmentConflict},
  {pgError("23505"),assignments.ErrAssignmentConflict},
  {errors.New("database unavailable"),assignments.ErrStorageUnavailable},
 }{
  db:=&failingStore{current:"33333333-3333-4333-8333-333333333333",failure:tc.failure}
  svc:=assignments.Service{Auth:&authStub{role:"ADMIN"},Store:db}
  got,err:=svc.ReplaceRecipient(context.Background(),"token","proof",authentication.RequestBinding{},owner,recipient)
  if !errors.Is(err,tc.want)||got!=(chat.Assignment{})||db.current!="33333333-3333-4333-8333-333333333333"||db.calls!=1{
   t.Fatalf("rollback mapping: %+v %v current=%s",got,err,db.current)
  }
 }
}
func TestFC04NoAssignmentAndReadFailure(t *testing.T){
 owner:=chat.AccountID("11111111-1111-4111-8111-111111111111")
 for _,tc:=range []struct{failure,want error}{
  {sql.ErrNoRows,assignments.ErrNoAssignment},
  {errors.New("read failed"),assignments.ErrStorageUnavailable},
 }{
  got,err:=(assignments.Service{Store:&failingStore{failure:tc.failure}}).GetAssignment(context.Background(),owner)
  if !errors.Is(err,tc.want)||got!=(chat.Assignment{}){t.Fatalf("read: %+v %v",got,err)}
 }
 got,err:=(assignments.Service{Store:&failingStore{current:"bad-uuid"}}).GetAssignment(context.Background(),owner)
 if !errors.Is(err,assignments.ErrStorageUnavailable)||got!=(chat.Assignment{}){t.Fatalf("invalid stored UUID: %+v %v",got,err)}
}

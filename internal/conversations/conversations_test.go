package conversations_test

import (
 "context"
 "database/sql"
 "errors"
 "testing"

 "github.com/bogachenko/bfe-chat-service/internal/authentication"
 "github.com/bogachenko/bfe-chat-service/internal/chat"
 "github.com/bogachenko/bfe-chat-service/internal/conversations"
 "github.com/bogachenko/bfe-chat-service/internal/documentcontext"
 "github.com/bogachenko/bfe-chat-service/internal/storage"
)
const owner=chat.AccountID("11111111-1111-4111-8111-111111111111")
const key="22222222-2222-4222-8222-222222222222"
type authStub struct{err error;calls int}
func(a *authStub)ValidateCaller(context.Context,string,string,authentication.RequestBinding)(authentication.Caller,error){a.calls++;return authentication.Caller{AccountID:string(owner)},a.err}
type permissionStub struct{sendErr,readErr error;send,read int}
func(p *permissionStub)AuthorizeSend(context.Context,authentication.Caller,chat.AccountID)error{p.send++;return p.sendErr}
func(p *permissionStub)AuthorizeRead(context.Context,authentication.Caller,chat.AccountID)error{p.read++;return p.readErr}
type storeStub struct{send,read int;writeErr,readErr error;message storage.Message;history []storage.HistoryMessage;docID,docTitle sql.NullString;text string}
func(s *storeStub)CommitMessage(_ context.Context,_,_,_,body string,id,title sql.NullString)(storage.Message,error){s.send++;s.text=body;s.docID=id;s.docTitle=title;return s.message,s.writeErr}
func(s *storeStub)ReadHistory(context.Context,string,int64,int)([]storage.HistoryMessage,error){s.read++;return s.history,s.readErr}
func fixture()(conversations.Service,*authStub,*permissionStub,*storeStub){
 a:=&authStub{};p:=&permissionStub{};s:=&storeStub{message:storage.Message{ID:7,RecipientID:string(owner)}}
 return conversations.Service{Auth:a,Permissions:p,Store:s},a,p,s
}
func send(s conversations.Service,input conversations.MessageInput)(storage.Message,error){return s.SendMessage(context.Background(),"token","proof",authentication.RequestBinding{},owner,input)}
func read(s conversations.Service,after int64,limit int)([]storage.HistoryMessage,error){return s.ReadHistory(context.Background(),"token","proof",authentication.RequestBinding{},owner,after,limit)}
func TestFC05AuthorizedSendAndDocuments(t *testing.T){
 s,a,p,db:=fixture()
 got,err:=send(s,conversations.MessageInput{IdempotencyKey:key,Text:" hello ",Document:&documentcontext.Document{ID:" file ",Title:" title "}})
 if err!=nil||got.ID!=7||db.send!=1||a.calls!=1||p.send!=1||db.text!="hello"||db.docID.String!="file"||db.docTitle.String!="title"{t.Fatalf("send=%+v err=%v store=%+v",got,err,db)}
}
func TestFC05RejectsInvalidAndUnauthorizedWithoutWrites(t *testing.T){
 for _,tc:=range []struct{name string;input conversations.MessageInput;authErr,policyErr,want error}{
  {"no content",conversations.MessageInput{IdempotencyKey:key},nil,nil,conversations.ErrValidation},
  {"bad idempotency UUID",conversations.MessageInput{IdempotencyKey:"bad",Text:"text"},nil,nil,conversations.ErrValidation},
  {"bad document",conversations.MessageInput{IdempotencyKey:key,Document:&documentcontext.Document{ID:"file"}},nil,nil,conversations.ErrValidation},
  {"forbidden",conversations.MessageInput{IdempotencyKey:key,Text:"text"},nil,errors.New("DENIED"),conversations.ErrForbidden},
  {"revoked",conversations.MessageInput{IdempotencyKey:key,Text:"text"},authentication.ErrAuthInvalid,nil,authentication.ErrAuthInvalid},
  {"unavailable",conversations.MessageInput{IdempotencyKey:key,Text:"text"},authentication.ErrAuthUnavailable,nil,authentication.ErrAuthUnavailable},
 }{t.Run(tc.name,func(t *testing.T){
  s,a,p,db:=fixture();a.err=tc.authErr;p.sendErr=tc.policyErr
  got,err:=send(s,tc.input)
  if !errors.Is(err,tc.want)||got!=(storage.Message{})||db.send!=0{t.Fatalf("send=%+v err=%v writes=%d",got,err,db.send)}
 })}
}
type sqlState string
func(e sqlState)Error()string{return string(e)}
func(e sqlState)SQLState()string{return string(e)}
func TestFC05DatabaseErrorsAndReplay(t *testing.T){
 for _,tc:=range []struct{err,want error}{
  {sqlState("P0001"),conversations.ErrNoAssignment},
  {sqlState("22023"),conversations.ErrValidation},
  {sqlState("22P02"),conversations.ErrValidation},
  {sqlState("23514"),conversations.ErrValidation},
  {errors.New("offline"),conversations.ErrStorageUnavailable},
 }{
  s,_,_,db:=fixture();db.writeErr=tc.err
  got,err:=send(s,conversations.MessageInput{IdempotencyKey:key,Text:"hello"})
  if !errors.Is(err,tc.want)||got!=(storage.Message{}){t.Fatalf("err=%v want=%v",err,tc.want)}
 }
 s,_,_,db:=fixture();db.message.Replayed=true
 got,err:=send(s,conversations.MessageInput{IdempotencyKey:key,Text:"text"})
 if err!=nil||!got.Replayed||got.ID!=7{t.Fatalf("replay %+v %v",got,err)}
}
func TestFC06HistoryReadOnlyAndBoundaries(t *testing.T){
 s,a,p,db:=fixture();db.history=[]storage.HistoryMessage{{ID:3},{ID:5}}
 got,err:=read(s,3,100)
 if err!=nil||len(got)!=2||db.send!=0||db.read!=1||a.calls!=1||p.read!=1{t.Fatalf("history %+v %v store=%+v",got,err,db)}
 for _,tc:=range []struct{after int64;limit int}{{-1,1},{0,0},{0,101}}{
  s,_,_,db:=fixture()
  result,err:=read(s,tc.after,tc.limit)
  if !errors.Is(err,conversations.ErrValidation)||result!=nil||db.read!=0{t.Fatalf("boundary %+v %v",result,err)}
 }
}
func TestFC06UnauthorizedAndStorageErrors(t *testing.T){
 for _,tc:=range []struct{authErr,permissionErr,storeErr,want error}{
  {authentication.ErrAuthInvalid,nil,nil,authentication.ErrAuthInvalid},
  {nil,errors.New("denied"),nil,conversations.ErrForbidden},
  {nil,nil,errors.New("unavailable"),conversations.ErrStorageUnavailable},
 }{
  s,a,p,db:=fixture();a.err=tc.authErr;p.readErr=tc.permissionErr;db.readErr=tc.storeErr
  result,err:=read(s,0,10)
  if !errors.Is(err,tc.want)||result!=nil||db.send!=0{t.Fatalf("history %+v %v",result,err)}
 }
}

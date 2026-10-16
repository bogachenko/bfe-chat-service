package storage_test

import (
 "context"
 "database/sql"
 "errors"
 "testing"

 "github.com/bogachenko/bfe-chat-service/internal/storage"
)

func TestCommitMessageInvalidInputs(t *testing.T) {
 t.Parallel()
 cases:=[]struct{name,owner,sender,key string}{
  {"nil DB","owner","sender","key"},
  {"empty owner","","sender","key"},
  {"empty sender","owner","","key"},
  {"empty key","owner","sender",""},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){t.Parallel();got,err:=(storage.Store{}).CommitMessage(context.Background(),tc.owner,tc.sender,tc.key,"body",sql.NullString{},sql.NullString{});if !errors.Is(err,storage.ErrInvalidInput)||got!=(storage.Message{}){t.Fatalf("message=%+v error=%v",got,err)}})}
}

func TestCommitAssignmentInvalidInputs(t *testing.T) {
 t.Parallel()
 cases:=[]struct{name,owner,recipient string}{
  {"nil DB","owner","recipient"},
  {"empty owner","","recipient"},
  {"empty recipient","owner",""},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){t.Parallel();if err:=(storage.Store{}).CommitAssignment(context.Background(),tc.owner,tc.recipient);!errors.Is(err,storage.ErrInvalidInput){t.Fatalf("error=%v",err)}})}
}

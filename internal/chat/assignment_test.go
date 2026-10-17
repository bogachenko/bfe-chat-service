package chat_test

import (
 "errors"
 "testing"
 "github.com/bogachenko/bfe-chat-service/internal/chat"
)
func TestAccountIDValid(t *testing.T) {
 t.Parallel()
 for _,tt:=range []struct{name,id string;want bool}{
  {"lowercase uuid","11111111-1111-1111-1111-111111111111",true},
  {"uppercase hex","ABCDEF01-ABCD-ABCD-ABCD-ABCDEF012345",true},
  {"empty","",false},
  {"short","123",false},
  {"bad separator","11111111_1111-1111-1111-111111111111",false},
  {"non hex","zzzzzzzz-1111-1111-1111-111111111111",false},
  {"extra characters","11111111-1111-1111-1111-1111111111111",false},
 } {
  t.Run(tt.name,func(t *testing.T){t.Parallel();if got:=chat.AccountID(tt.id).Valid();got!=tt.want{t.Fatalf("Valid(%q)=%t want %t",tt.id,got,tt.want)}})
 }
}
func TestNewAssignment(t *testing.T) {
 t.Parallel()
 const a chat.AccountID="11111111-1111-1111-1111-111111111111"
 const b chat.AccountID="22222222-2222-2222-2222-222222222222"
 for _,tt:=range []struct{name string;owner,recipient chat.AccountID;wantErr bool}{
  {"valid",a,b,false},{"same account",a,a,false},{"invalid owner","bad",b,true},{"invalid recipient",a,"bad",true},{"missing owner","",b,true},{"missing recipient",a,"",true},
 } {
  t.Run(tt.name,func(t *testing.T){t.Parallel();got,err:=chat.NewAssignment(tt.owner,tt.recipient);if tt.wantErr{if !errors.Is(err,chat.ErrInvalidAccountID)||got!=(chat.Assignment{}){t.Fatalf("assignment=%+v err=%v",got,err)};return};if err!=nil||got.Owner!=tt.owner||got.Recipient!=tt.recipient{t.Fatalf("assignment=%+v err=%v",got,err)}})
 }
}

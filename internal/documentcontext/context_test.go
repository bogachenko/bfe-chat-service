package documentcontext_test

import (
 "errors"
 "testing"
 "github.com/bogachenko/bfe-chat-service/internal/documentcontext"
)
func TestValidateDocumentContext(t *testing.T) {
 t.Parallel()
 for _,tt:=range []struct{name,text string;doc *documentcontext.Document;wantText string;wantDoc *documentcontext.Document;invalid bool}{
  {"text only"," hello ",nil,"hello",nil,false},
  {"document only","",&documentcontext.Document{ID:" 123 ",Title:" Title "},"",&documentcontext.Document{ID:"123",Title:"Title"},false},
  {"text and document"," text ",&documentcontext.Document{ID:" doc ",Title:" Title "},"text",&documentcontext.Document{ID:"doc",Title:"Title"},false},
  {"empty content"," \n ",nil,"",nil,true},
  {"empty document id","",&documentcontext.Document{ID:"  ",Title:"Title"},"",nil,true},
  {"empty document title","",&documentcontext.Document{ID:"doc",Title:"  "},"",nil,true},
  {"invalid document rejects otherwise valid text","text",&documentcontext.Document{ID:"",Title:"Title"},"",nil,true},
 }{
  t.Run(tt.name,func(t *testing.T){t.Parallel();got,err:=documentcontext.ValidateDocumentContext(tt.text,tt.doc);if tt.invalid{if !errors.Is(err,documentcontext.ErrValidation)||got!=(documentcontext.MessageContent{}){t.Fatalf("invalid got=%+v err=%v",got,err)};return};if err!=nil||got.Text!=tt.wantText{t.Fatalf("got=%+v err=%v",got,err)};if (got.Document==nil)!=(tt.wantDoc==nil){t.Fatalf("document=%+v want=%+v",got.Document,tt.wantDoc)};if got.Document!=nil&&(*got.Document!=*tt.wantDoc){t.Fatalf("document=%+v want=%+v",got.Document,tt.wantDoc)}})
 }
}

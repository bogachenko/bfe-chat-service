package documentcontext_test

import (
 "errors"
 "strings"
 "testing"

 "github.com/bogachenko/bfe-chat-service/internal/documentcontext"
)

func TestIndependentFC10NormalizationAndValidation(t *testing.T) {
 cases:=[]struct{name,text string;doc *documentcontext.Document;wantText,wantID,wantTitle string;reject bool}{
  {"text only","  hello  ",nil,"hello","","",false},
  {"document only"," ",&documentcontext.Document{ID:" file-id ",Title:" file title "},"","file-id","file title",false},
  {"text and reference","  hello ",&documentcontext.Document{ID:" id ",Title:" title "},"hello","id","title",false},
  {"no content"," \t ",nil,"","","",true},
  {"empty ID","",&documentcontext.Document{ID:" ",Title:"valid"},"","","",true},
  {"empty title","",&documentcontext.Document{ID:"valid",Title:" "},"","","",true},
  {"invalid text UTF8",string([]byte{0xff}),nil,"","","",true},
  {"invalid ID UTF8","text",&documentcontext.Document{ID:string([]byte{0xff}),Title:"title"},"","","",true},
  {"invalid title UTF8","text",&documentcontext.Document{ID:"id",Title:string([]byte{0xff})},"","","",true},
  {"NUL text","a\x00b",nil,"","","",true},
  {"NUL ID","text",&documentcontext.Document{ID:"a\x00b",Title:"title"},"","","",true},
  {"NUL title","text",&documentcontext.Document{ID:"id",Title:"a\x00b"},"","","",true},
  {"unicode"," Привет 🧾 ",&documentcontext.Document{ID:"документ",Title:"Название 🧾"},"Привет 🧾","документ","Название 🧾",false},
  {"long valid title","",&documentcontext.Document{ID:"id",Title:strings.Repeat("x",4096)},"","id",strings.Repeat("x",4096),false},
 }
 for _,tc:=range cases {
  t.Run(tc.name,func(t *testing.T){
   checkIndependentFC10Case(t,tc.text,tc.doc,tc.wantText,tc.wantID,tc.wantTitle,tc.reject)
  })
 }
}

func checkIndependentFC10Case(t *testing.T,text string,doc *documentcontext.Document,wantText,wantID,wantTitle string,reject bool) {
 t.Helper()
 var before documentcontext.Document
 if doc!=nil {before=*doc}
 got,err:=documentcontext.ValidateDocumentContext(text,doc)
 if reject {
  checkRejectedFC10(t,got,err)
 } else {
  checkAcceptedFC10(t,got,err,doc,wantText,wantID,wantTitle)
 }
 if doc!=nil&&*doc!=before {t.Fatal("input document mutated")}
}

func checkRejectedFC10(t *testing.T,got documentcontext.MessageContent,err error) {
 t.Helper()
 if !errors.Is(err,documentcontext.ErrValidation)||got!=(documentcontext.MessageContent{}) {
  t.Fatalf("want validation error, got %+v %v",got,err)
 }
}

func checkAcceptedFC10(t *testing.T,got documentcontext.MessageContent,err error,doc *documentcontext.Document,wantText,wantID,wantTitle string) {
 t.Helper()
 if err!=nil||got.Text!=wantText {t.Fatalf("got %+v %v",got,err)}
 if doc==nil {
  if got.Document!=nil {t.Fatal("unexpected reference")}
  return
 }
 if got.Document==nil||got.Document.ID!=wantID||got.Document.Title!=wantTitle||got.Document==doc {
  t.Fatalf("reference=%+v",got.Document)
 }
}

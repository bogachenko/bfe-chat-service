package main

import (
 "context"
 "database/sql"
 "net/http"
 "net/http/httptest"
 "strings"
 "os"
 "path/filepath"
 "testing"
)

func TestFC13ProductionBootstrapFailsClosed(t *testing.T){
 t.Setenv("BFE_CHAT_TEST_DB_DSN","")
 for _,tc:=range []struct{name string;args []string}{
  {"missing configuration",nil},
  {"unexpected argument",[]string{"unexpected"}},
  {"unsafe public listen",[]string{"-listen=0.0.0.0:8081","-drive-auth-uds=/fake.sock","-migrations=/fake","-public-origin=https://chat.example.test"}},
  {"invalid listener",[]string{"-listen=bad","-drive-auth-uds=/fake.sock","-migrations=/fake","-public-origin=https://chat.example.test"}},
  {"missing DSN",[]string{"-drive-auth-uds=/fake.sock","-migrations=/fake","-public-origin=https://chat.example.test","-dsn-env=BFE_CHAT_TEST_DB_DSN"}},
  {"missing UDS",[]string{"-drive-auth-uds=/nonexistent/bfe-drive.sock","-migrations=/fake","-public-origin=https://chat.example.test","-dsn-env=BFE_CHAT_TEST_DB_DSN"}},
 }{
  t.Run(tc.name,func(t *testing.T){if err:=run(context.Background(),tc.args);err==nil{t.Fatal("accepted invalid configuration")}})
 }
}
func TestFC13RejectsNonSocketAuthenticationEndpoint(t *testing.T){
 file:=filepath.Join(t.TempDir(),"auth.sock")
 if err:=os.WriteFile(file,[]byte("not socket"),0600);err!=nil{t.Fatal(err)}
 t.Setenv("BFE_CHAT_TEST_DB_DSN","postgres://fixture-invalid")
 err:=run(context.Background(),[]string{"-drive-auth-uds="+file,"-migrations=/nonexistent","-public-origin=https://chat.example.test","-dsn-env=BFE_CHAT_TEST_DB_DSN"})
 if err==nil{t.Fatal("accepted regular file instead of protected UDS")}
}

func TestIssue32OuterRouterSeparatesCurrentAndRetiredPaths(t *testing.T) {
 t.Parallel()
 h, _, err := buildHandler(&sql.DB{}, runtimeConfig{uds:"/nonexistent/bfe-drive.sock",origin:"https://chat.example.test"})
 if err!=nil {t.Fatal(err)}
 for _,tc:=range []struct{name,method,path,auth,proof string;want int}{
  {"current root",http.MethodGet,"/conversations","","",http.StatusUnauthorized},
  {"current direct",http.MethodPost,"/conversations/direct","","",http.StatusUnauthorized},
  {"current group",http.MethodPost,"/conversations/group","","",http.StatusUnauthorized},
  {"current history",http.MethodGet,"/conversations/11111111-1111-4111-8111-111111111111/history","","",http.StatusUnauthorized},
  {"current rename",http.MethodPut,"/conversations/11111111-1111-4111-8111-111111111111/name","","",http.StatusUnauthorized},
  {"retired v1",http.MethodGet,"/v1/conversations/11111111-1111-4111-8111-111111111111/history","DPoP opaque","signed",http.StatusNotFound},
  {"retired v2",http.MethodPost,"/v2/conversations/direct","DPoP opaque","signed",http.StatusNotFound},
  {"retired v2 group",http.MethodPut,"/v2/conversations/11111111-1111-4111-8111-111111111111/name","","",http.StatusNotFound},
  {"near prefix",http.MethodGet,"/conversations-extra","","",http.StatusNotFound},
  {"ws wrong method",http.MethodPost,"/ws","","",http.StatusMethodNotAllowed},
 } {
  t.Run(tc.name,func(t *testing.T){
   t.Parallel()
   req:=httptest.NewRequest(tc.method,tc.path,nil)
   if tc.auth!=""{req.Header.Set("Authorization",tc.auth)}
   if tc.proof!=""{req.Header.Set("DPoP",tc.proof)}
   w:=httptest.NewRecorder()
   h.ServeHTTP(w,req)
   if w.Code!=tc.want{t.Fatalf("method=%s path=%s status=%d want=%d body=%s",tc.method,tc.path,w.Code,tc.want,w.Body.String())}
   if strings.Contains(w.Body.String(),"signed"){t.Fatal("proof leaked in response")}
  })
 }
}

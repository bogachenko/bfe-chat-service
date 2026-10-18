package main

import (
 "context"
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

//go:build integration

package main

import (
 "context"
 "io"
 "net"
 "net/http"
 "path/filepath"
 "strconv"
 "testing"
 "time"

 "github.com/testcontainers/testcontainers-go"
 pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
 "github.com/testcontainers/testcontainers-go/wait"
)

func TestDPM08RealPostgreSQLBootstrapHTTPAndShutdown(t *testing.T){
 ctx,cancel:=context.WithTimeout(context.Background(),120*time.Second)
 defer cancel()
 ctr,err:=pgcontainer.Run(ctx,"postgres:18.6",
  pgcontainer.WithDatabase("bfe"),pgcontainer.WithUsername("bfe"),pgcontainer.WithPassword("bfe"),
  testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90*time.Second)))
 if err!=nil{t.Fatal("PostgreSQL integration fixture unavailable")}
 defer func(){_ =testcontainers.TerminateContainer(ctr)}()
 dsn,err:=ctr.ConnectionString(ctx,"sslmode=disable")
 if err!=nil{t.Fatal("PostgreSQL connection string unavailable")}
 t.Setenv("BFE_M08_TEST_PG_DSN",dsn)
 socket:=filepath.Join(t.TempDir(),"drive-auth.sock")
 uds,err:=net.Listen("unix",socket)
 if err!=nil{t.Fatal("start fixture UDS")}
 defer func(){_ =uds.Close()}()
 drive:=&http.Server{Handler:http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.WriteHeader(http.StatusUnauthorized)})}
 go func(){_ =drive.Serve(uds)}()
 defer func(){_ =drive.Close()}()
 probe,err:=net.Listen("tcp","127.0.0.1:0")
 if err!=nil{t.Fatal("allocate loopback")}
 port:=probe.Addr().(*net.TCPAddr).Port
 _=probe.Close()
 args:=[]string{
  "-listen=127.0.0.1:"+strconv.Itoa(port),
  "-public-origin=https://chat.example.test",
  "-drive-auth-uds="+socket,
  "-migrations=../../internal/storage/migrations",
  "-dsn-env=BFE_M08_TEST_PG_DSN",
 }
 runtimeCtx,stop:=context.WithCancel(context.Background())
 defer stop()
 result:=make(chan error,1)
 go func(){result<-run(runtimeCtx,args)}()
 url:="http://127.0.0.1:"+strconv.Itoa(port)+"/v1/conversations/11111111-1111-1111-1111-111111111111/history"
 client:=&http.Client{Timeout:500*time.Millisecond}
 ready:=false
 for deadline:=time.Now().Add(30*time.Second);time.Now().Before(deadline);{
  resp,err:=client.Get(url)
  if err==nil{
   _,_ =io.Copy(io.Discard,resp.Body)
   _=resp.Body.Close()
   if resp.StatusCode!=http.StatusUnauthorized{t.Fatalf("public endpoint did not fail closed: %d",resp.StatusCode)}
   ready=true;break
  }
  select{case err:=<-result:t.Fatalf("bootstrap failed: %v",err);default:}
  time.Sleep(100*time.Millisecond)
 }
 if !ready{t.Fatal("bootstrap HTTP endpoint never started")}
 stop()
 select {
 case err:=<-result:if err!=nil{t.Fatalf("graceful shutdown failed: %v",err)}
 case <-time.After(15*time.Second):t.Fatal("server did not shutdown")
 }
}
func TestDPM08RejectsUnreachablePostgresBeforeListener(t *testing.T){
 socket:=filepath.Join(t.TempDir(),"auth.sock")
 l,err:=net.Listen("unix",socket)
 if err!=nil{t.Fatal(err)}
 defer func(){_ =l.Close()}()
 t.Setenv("BFE_M08_MISSING_DB","postgres://127.0.0.1:1/bfe?connect_timeout=1")
 ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second)
 defer cancel()
 if err:=run(ctx,[]string{"-drive-auth-uds="+socket,"-public-origin=https://chat.example.test","-migrations=../../internal/storage/migrations","-dsn-env=BFE_M08_MISSING_DB"});err==nil{
  t.Fatal("unreachable database accepted")
 }
}

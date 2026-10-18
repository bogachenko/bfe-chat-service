//go:build integration

// This fixture runs inside BFE Drive internal/httpapi so it invokes the real
// NewInternalAuthHandler backed by its own PostgreSQL schema and sessions.
package httpapi

import (
 "bufio"
 "context"
 "crypto/rand"
 "crypto/sha256"
 "database/sql"
 "encoding/base64"
 "encoding/hex"
 "errors"
 "fmt"
 "io"
 "net"
 "net/http"
 "net/url"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "testing"
 "time"

 "github.com/jackc/pgx/v5/pgtype"
 "github.com/jackc/pgx/v5/pgxpool"
 _ "github.com/jackc/pgx/v5/stdlib"
 "github.com/pressly/goose/v3"
 "github.com/testcontainers/testcontainers-go"
 pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
 "github.com/testcontainers/testcontainers-go/wait"
)

func TestQADPM08ProductionExecutableRealDriveUDS(t *testing.T){
 executable:=os.Getenv("CHAT_M08_BINARY")
 probe:=os.Getenv("CHAT_M08_PROBE")
 markerPath:=os.Getenv("CHAT_M08_MARKERS")
 jkt:=os.Getenv("CHAT_M08_JKT")
 if executable==""||probe==""||markerPath==""||jkt==""{t.Fatal("DP-M08 harness configuration missing")}
 ctx,cancel:=context.WithTimeout(context.Background(),4*time.Minute)
 defer cancel()
 ctr,err:=pgcontainer.Run(ctx,"postgres:18.6",
  pgcontainer.WithDatabase("bfe"),pgcontainer.WithUsername("bfe"),pgcontainer.WithPassword("bfe"),
  testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90*time.Second)))
 if err!=nil{t.Fatal("PostgreSQL 18.6 unavailable")}
 t.Cleanup(func(){_ =testcontainers.TerminateContainer(ctr)})
 dsn,err:=ctr.ConnectionString(ctx,"sslmode=disable")
 if err!=nil{t.Fatal("PostgreSQL DSN unavailable")}
 db,err:=sql.Open("pgx",dsn)
 if err!=nil{t.Fatal("PostgreSQL connection unavailable")}
 if err:=goose.SetDialect("postgres");err!=nil{t.Fatal("migration dialect")}
 if err:=goose.Up(db,filepath.Join(searchHTTPRepoRoot(t),"migrations"));err!=nil{t.Fatal("Drive migrations failed")}
 if err:=db.Close();err!=nil{t.Fatal("close initial database connection")}
 pool,err:=pgxpool.New(ctx,dsn)
 if err!=nil{t.Fatal("connect Drive database")}
 t.Cleanup(pool.Close)
 if _,err:=pool.Exec(ctx,"CREATE DATABASE chatqa");err!=nil{t.Fatal("create separate Chat database")}
 chatURL,err:=url.Parse(dsn)
 if err!=nil{t.Fatal("invalid container DSN")}
 chatURL.Path="/chatqa"
 chatDSN:=chatURL.String()
 var accountID pgtype.UUID
 if err:=pool.QueryRow(ctx,`INSERT INTO accounts(login,password_verifier,activation_status,administrative_status)
 VALUES('dpm08-executable-qa','verifier','ACTIVE','ADMIN') RETURNING id`).Scan(&accountID);err!=nil{t.Fatal("insert owner account")}
 ownerHex:=hex.EncodeToString(accountID.Bytes[:])
 owner:=fmt.Sprintf("%s-%s-%s-%s-%s",ownerHex[:8],ownerHex[8:12],ownerHex[12:16],ownerHex[16:20],ownerHex[20:])
 old:=fixtureRandomToken(t)
 insertSession(t,ctx,pool,accountID,old,jkt)
 socket:=filepath.Join(t.TempDir(),"bfe-drive-auth.sock")
 listener,err:=net.Listen("unix",socket)
 if err!=nil{t.Fatal("open real Drive auth UDS")}
 server:=&http.Server{Handler:NewInternalAuthHandler(pool),ReadHeaderTimeout:5*time.Second}
 go func(){_ =server.Serve(listener)}()
 t.Cleanup(func(){_ =server.Close()})
 portListener,err:=net.Listen("tcp","127.0.0.1:0")
 if err!=nil{t.Fatal("allocate executable loopback port")}
 address:=portListener.Addr().String()
 if err:=portListener.Close();err!=nil{t.Fatal("release executable port")}
 app:=exec.CommandContext(ctx,executable,
  "-listen="+address,
  "-public-origin=https://chat.example.test",
  "-drive-auth-uds="+socket,
  "-dsn-env=CHAT_M08_DSN",
  "-migrations="+os.Getenv("CHAT_M08_MIGRATIONS"))
 app.Env=append(os.Environ(),"CHAT_M08_DSN="+chatDSN)
 app.Stdout=io.Discard
 app.Stderr=io.Discard
 if err:=app.Start();err!=nil{t.Fatal("start actual production executable")}
 t.Cleanup(func(){if app.Process!=nil{_ =app.Process.Kill();_ =app.Wait()}})
 if err:=waitForExecutable(ctx,address);err!=nil{t.Fatal("production executable did not start")}
 client:=exec.CommandContext(ctx,probe,"--address="+address,"--owner="+owner)
 client.Env=append(os.Environ(),"CHAT_DPM08_TOKEN="+old)
 childOut,err:=client.StdoutPipe()
 if err!=nil{t.Fatal("connect private probe output")}
 childIn,err:=client.StdinPipe()
 if err!=nil{t.Fatal("connect private probe input")}
 client.Stderr=io.Discard
 if err:=client.Start();err!=nil{t.Fatal("start DP-M08 probe")}
 t.Cleanup(func(){if client.Process!=nil{_ =client.Process.Kill()}})
 scanner:=bufio.NewScanner(childOut)
 expect:=func(stage string){
  t.Helper()
  if !scanner.Scan()||strings.TrimSpace(scanner.Text())!=stage{t.Fatalf("DP-M08 milestone %s did not complete",stage)}
  out,err:=os.OpenFile(markerPath,os.O_APPEND|os.O_WRONLY,0600)
  if err!=nil{t.Fatal("private marker file unavailable")}
  if _,err:=io.WriteString(out,stage+" PASS\n");err!=nil{_ =out.Close();t.Fatal("private marker write failed")}
  if err:=out.Close();err!=nil{t.Fatal("close private marker file")}
 }
 for _,stage:=range []string{"HTTP_ASSIGNMENT","HTTP_SEND","HTTP_HISTORY","HTTP_FORBIDDEN","WS_CONNECT","WS_SUBSCRIBE","WS_DELIVERY_HTTP_COMMITTED","WS_EVENT_FRAME","WS_EVENT_PARSED","WS_DELIVERY"}{expect(stage)}
 oldHash:=sha256.Sum256([]byte(old))
 changed,err:=pool.Exec(ctx,`UPDATE login_sessions SET session_invalidated=true WHERE access_token_hash=$1`,oldHash[:])
 if err!=nil||changed.RowsAffected()!=1{t.Fatal("revoke old real BFE Drive session")}
 if _,err:=io.WriteString(childIn,"REVOKED\n");err!=nil{t.Fatal("send revoke boundary")}
 expect("HTTP_REVOKED")
 expect("WS_RECONNECT_DENIED")
 fresh:=fixtureRandomToken(t)
 insertSession(t,ctx,pool,accountID,fresh,jkt)
 if _,err:=io.WriteString(childIn,"NEW\n"+fresh+"\n");err!=nil{t.Fatal("send fresh-session boundary")}
 for _,stage:=range []string{"HTTP_FRESH_SESSION","WS_FRESH_RECONNECT","OPEN_AFTER_REVOKE","WS_FRESH_DELIVERY"}{expect(stage)}
 if err:=server.Close();err!=nil{t.Fatal("close real BFE Drive UDS")}
 if _,err:=io.WriteString(childIn,"UDS_DOWN\n"+fresh+"\n");err!=nil{t.Fatal("send unavailable boundary")}
 expect("HTTP_UDS_UNAVAILABLE")
 expect("WS_UDS_UNAVAILABLE")
 if err:=childIn.Close();err!=nil{t.Fatal("close private stage pipe")}
 if err:=client.Wait();err!=nil{t.Fatal("production E2E probe did not complete")}
}
func fixtureRandomToken(t *testing.T)string{
 t.Helper()
 data:=make([]byte,32)
 if _,err:=rand.Read(data);err!=nil{t.Fatal("generate private session token")}
 return base64.RawURLEncoding.EncodeToString(data)
}
func insertSession(t *testing.T,ctx context.Context,pool *pgxpool.Pool,owner pgtype.UUID,token,jkt string){
 t.Helper()
 hash:=sha256.Sum256([]byte(token))
 renewal:=sha256.Sum256([]byte(fixtureRandomToken(t)))
 _,err:=pool.Exec(ctx,`INSERT INTO login_sessions(account_id,access_token_hash,session_expires_at,session_invalidated,renewal_token_hash,renewal_invalidated,dpop_jkt)
 VALUES($1,$2,clock_timestamp()+interval '1 hour',false,$3,false,$4)`,owner,hash[:],renewal[:],jkt)
 if err!=nil{t.Fatal("insert real BFE Drive session")}
}
func waitForExecutable(ctx context.Context,address string)error{
 client:=http.Client{Timeout:600*time.Millisecond}
 for{
  select{case <-ctx.Done():return errors.New("production executable readiness timeout");default:}
  request,err:=http.NewRequestWithContext(ctx,http.MethodGet,"http://"+address+"/v1/conversations/11111111-1111-1111-1111-111111111111/history",nil)
  if err!=nil{return err}
  response,err:=client.Do(request)
  if err==nil{
   _=response.Body.Close()
   if response.StatusCode==http.StatusUnauthorized{return nil}
  }
  time.Sleep(100*time.Millisecond)
 }
}

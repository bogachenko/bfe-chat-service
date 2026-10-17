//go:build integration

package httpapi

import (
 "context"
 "crypto/sha256"
 "crypto/rand"
 "encoding/base64"
 "encoding/hex"
 "bufio"
 "fmt"
 "io"
 "strings"
 "database/sql"
 "net"
 "net/http"
 "os"
 "os/exec"
 "path/filepath"
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

func TestQADPM05RealCentrifugeWebSocketUDS(t *testing.T){
 probe:=os.Getenv("CHAT_WS_PROBE")
 if probe==""{t.Fatal("CHAT_WS_PROBE must point at the compiled BFE Chat WebSocket QA binary")}
 ctx,cancel:=context.WithTimeout(context.Background(),120*time.Second);defer cancel()
 ctr,err:=pgcontainer.Run(ctx,"postgres:18.6",
  pgcontainer.WithDatabase("bfe"),pgcontainer.WithUsername("bfe"),pgcontainer.WithPassword("bfe"),
  testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90*time.Second)))
 if err!=nil{t.Fatal(err)}
 t.Cleanup(func(){_ =testcontainers.TerminateContainer(ctr)})
 dsn,err:=ctr.ConnectionString(ctx,"sslmode=disable");if err!=nil{t.Fatal(err)}
 db,err:=sql.Open("pgx",dsn);if err!=nil{t.Fatal(err)}
 if err:=goose.SetDialect("postgres");err!=nil{t.Fatal(err)}
 if err:=goose.Up(db,filepath.Join(searchHTTPRepoRoot(t),"migrations"));err!=nil{t.Fatal(err)}
 if err:=db.Close();err!=nil{t.Fatal(err)}
 pool,err:=pgxpool.New(context.Background(),dsn);if err!=nil{t.Fatal(err)}
 t.Cleanup(pool.Close)
 if err:=pool.Ping(ctx);err!=nil{t.Fatal(err)}
 jkt:=os.Getenv("CHAT_WS_JKT")
 if jkt==""{t.Fatal("CHAT_WS_JKT required from auth probe")}
 var accountID pgtype.UUID
 if err:=pool.QueryRow(ctx,`INSERT INTO accounts(login,password_verifier,activation_status,administrative_status)
 VALUES('chat-qa-reality-check','verifier','ACTIVE','NON_ADMIN') RETURNING id`).Scan(&accountID);err!=nil{t.Fatal(err)}
 token:=newWSFixtureToken(t)
 hash:=sha256.Sum256([]byte(token))
 renewalHash:=sha256.Sum256([]byte(newWSFixtureToken(t)))
 _,err=pool.Exec(ctx,`INSERT INTO login_sessions(account_id,access_token_hash,session_expires_at,session_invalidated,renewal_token_hash,renewal_invalidated,dpop_jkt)
 VALUES($1,$2,clock_timestamp()+interval '1 hour',false,$3,false,$4)`,accountID,hash[:],renewalHash[:],jkt)
 if err!=nil{t.Fatal(err)}
 socket:=filepath.Join(t.TempDir(),"drive-auth.sock")
 listener,err:=net.Listen("unix",socket);if err!=nil{t.Fatal(err)}
 server:=&http.Server{Handler:NewInternalAuthHandler(pool)}
 go func(){_ =server.Serve(listener)}()
 t.Cleanup(func(){_ =server.Close()})
 accountHex:=hex.EncodeToString(accountID.Bytes[:])
 account:=fmt.Sprintf("%s-%s-%s-%s-%s",accountHex[:8],accountHex[8:12],accountHex[12:16],accountHex[16:20],accountHex[20:])
 childCtx,stop:=context.WithTimeout(context.Background(),90*time.Second)
 defer stop()
 child:=exec.CommandContext(childCtx,probe,"--socket="+socket,"--account="+account,"--expect=lifecycle")
 child.Env=append(os.Environ(),"CHAT_WS_TOKEN="+token)
 // No child stdout/stderr is copied into CI logs on failure.
 output,err:=child.StdoutPipe();if err!=nil{t.Fatal("open probe output")}
 input,err:=child.StdinPipe();if err!=nil{t.Fatal("open probe input")}
 child.Stderr=io.Discard
 if err:=child.Start();err!=nil{t.Fatal("start Centrifuge probe")}
 defer stop()
 scanner:=bufio.NewScanner(output)
 record:=func(name string){
  t.Helper()
  file:=os.Getenv("CHAT_WS_MARKERS")
  if file==""{t.Fatal("CHAT_WS_MARKERS required")}
  out,err:=os.OpenFile(file,os.O_WRONLY|os.O_APPEND,0600)
  if err!=nil{t.Fatal("open private scenario markers")}
  if _,err:=io.WriteString(out,name+" PASS\n");err!=nil{_ =out.Close();t.Fatal("write scenario marker")}
  if err:=out.Close();err!=nil{t.Fatal("close scenario marker")}
 }
 expectStage:=func(want string){
  t.Helper()
  if !scanner.Scan()||strings.TrimSpace(scanner.Text())!=want{
   t.Fatalf("WebSocket probe did not complete expected stage %s",want)
  }
 }
 expectStage("READY")
 record("VALID_CONNECT")
 if _,err:=pool.Exec(ctx,`UPDATE login_sessions SET session_invalidated=true WHERE access_token_hash=$1`,hash[:]);err!=nil{t.Fatal(err)}
 if _,err:=io.WriteString(input,"REVOKED\n");err!=nil{t.Fatal("send revoke event")}
 expectStage("OPEN_AFTER_REVOKE")
 record("OPEN_AFTER_REVOKE")
 expectStage("OLD_TOKEN_DENIED")
 record("OLD_TOKEN_DENIED")
 fresh:=newWSFixtureToken(t)
 freshHash:=sha256.Sum256([]byte(fresh))
 freshRenewal:=sha256.Sum256([]byte(newWSFixtureToken(t)))
 if _,err:=pool.Exec(ctx,`INSERT INTO login_sessions(account_id,access_token_hash,session_expires_at,session_invalidated,renewal_token_hash,renewal_invalidated,dpop_jkt)
 VALUES($1,$2,clock_timestamp()+interval '1 hour',false,$3,false,$4)`,accountID,freshHash[:],freshRenewal[:],jkt);err!=nil{t.Fatal(err)}
 if _,err:=io.WriteString(input,"NEW\n"+fresh+"\n");err!=nil{t.Fatal("send fresh login")}
 expectStage("NEW_SESSION_CONNECTED")
 record("NEW_SESSION_CONNECTED")
 expectStage("NEW_SESSION_DELIVERED")
 record("NEW_SESSION_DELIVERED")
 expectStage("real Centrifuge WebSocket + BFE Drive UDS: lifecycle PASS")
 if err:=input.Close();err!=nil{t.Fatal("close probe stdin")}
 if err:=child.Wait();err!=nil{t.Fatal("WebSocket lifecycle probe failed")}
 stop()
 if err:=server.Close();err!=nil{t.Fatal(err)}
 unavailable:=exec.CommandContext(context.Background(),probe,"--socket="+socket,"--account="+account,"--expect=unavailable")
 unavailable.Env=append(os.Environ(),"CHAT_WS_TOKEN="+fresh)
 unavailable.Stdout=io.Discard
 unavailable.Stderr=io.Discard
 if err:=unavailable.Run();err!=nil{t.Fatal("UDS outage was not explicitly denied")}
 record("UDS_UNAVAILABLE_DENIED")
}

func newWSFixtureToken(t *testing.T) string {
 t.Helper()
 b:=make([]byte,32)
 if _,err:=rand.Read(b);err!=nil{t.Fatal("generate fixture token")}
 return base64.RawURLEncoding.EncodeToString(b)
}

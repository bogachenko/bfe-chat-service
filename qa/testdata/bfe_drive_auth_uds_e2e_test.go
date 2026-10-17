//go:build integration

package httpapi

import (
 "context"
 "crypto/sha256"
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

func TestQADPM01RealDriveUDS(t *testing.T){
 probe:=os.Getenv("CHAT_AUTH_PROBE")
 if probe==""{t.Fatal("CHAT_AUTH_PROBE must point at the compiled BFE Chat QA binary")}
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
 invoke:=func(args ...string)string{
  t.Helper()
  c,cancel:=context.WithTimeout(context.Background(),10*time.Second);defer cancel()
  out,err:=exec.CommandContext(c,probe,args...).CombinedOutput()
  if err!=nil{t.Fatalf("Chat QA child failed: %v; output=%s",err,out)}
  return string(out)
 }
 jkt:=invoke("--expect=jkt")
 var accountID pgtype.UUID
 if err:=pool.QueryRow(ctx,`INSERT INTO accounts(login,password_verifier,activation_status,administrative_status)
 VALUES('chat-qa-reality-check','verifier','ACTIVE','NON_ADMIN') RETURNING id`).Scan(&accountID);err!=nil{t.Fatal(err)}
 token:="opaque-session-token"
 hash:=sha256.Sum256([]byte(token))
 renewalHash:=sha256.Sum256([]byte("qa-renewal-token"))
 _,err=pool.Exec(ctx,`INSERT INTO login_sessions(account_id,access_token_hash,session_expires_at,session_invalidated,renewal_token_hash,renewal_invalidated,dpop_jkt)
 VALUES($1,$2,clock_timestamp()+interval '1 hour',false,$3,false,$4)`,accountID,hash[:],renewalHash[:],jkt)
 if err!=nil{t.Fatal(err)}
 socket:=filepath.Join(t.TempDir(),"drive-auth.sock")
 listener,err:=net.Listen("unix",socket);if err!=nil{t.Fatal(err)}
 server:=&http.Server{Handler:NewInternalAuthHandler(pool)}
 go func(){_ =server.Serve(listener)}()
 t.Cleanup(func(){_ =server.Close()})
 invoke("--socket="+socket,"--token="+token,"--expect=ok")
 if _,err:=pool.Exec(ctx,`UPDATE login_sessions SET session_invalidated=true WHERE access_token_hash=$1`,hash[:]);err!=nil{t.Fatal(err)}
 invoke("--socket="+socket,"--token="+token,"--expect=invalid")
 if err:=server.Close();err!=nil{t.Fatal(err)}
 invoke("--socket="+socket,"--token="+token,"--expect=unavailable")
}

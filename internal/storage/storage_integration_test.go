//go:build integration

package storage_test

import (
 "context"
 "database/sql"
 "fmt"
 "path/filepath"
 "sync"
 "sync/atomic"
 "testing"
 "time"

 "github.com/bogachenko/bfe-chat-service/internal/storage"
 _ "github.com/jackc/pgx/v5/stdlib"
 "github.com/pressly/goose/v3"
 "github.com/testcontainers/testcontainers-go/modules/postgres"
 "pgregory.net/rapid"
)

const (
 owner="11111111-1111-4111-8111-111111111111"
 sender="22222222-2222-4222-8222-222222222222"
 recipientA="aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
 recipientB="bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
)
func k(n int) string{return fmt.Sprintf("00000000-0000-4000-8000-%012x",n)}
func database(t *testing.T)(*sql.DB,storage.Store){
 t.Helper()
 ctx,cancel:=context.WithTimeout(context.Background(),2*time.Minute)
 defer cancel()
 p,err:=postgres.Run(ctx,"postgres:18.6",postgres.WithDatabase("chat_test"),postgres.WithUsername("test"),postgres.WithPassword("test"))
 if err!=nil{t.Fatal(err)}
 t.Cleanup(func(){c,end:=context.WithTimeout(context.Background(),30*time.Second);defer end();if err:=p.Terminate(c);err!=nil{t.Error(err)}})
 dsn,err:=p.ConnectionString(ctx,"sslmode=disable");if err!=nil{t.Fatal(err)}
 db,err:=sql.Open("pgx",dsn);if err!=nil{t.Fatal(err)}
 db.SetMaxOpenConns(16)
 t.Cleanup(func(){_ =db.Close()})
 if err:=db.PingContext(ctx);err!=nil{t.Fatal(err)}
 if err:=goose.SetDialect("postgres");err!=nil{t.Fatal(err)}
 if err:=goose.Up(db,filepath.Join("migrations"));err!=nil{t.Fatal(err)}
 return db,storage.Store{DB:db}
}
func assign(t *testing.T,s storage.Store,to string){t.Helper();if err:=s.CommitAssignment(context.Background(),owner,to);err!=nil{t.Fatal(err)}}
func send(t *testing.T,s storage.Store,n int)storage.Message{t.Helper();m,e:=s.CommitMessage(context.Background(),owner,sender,k(n),"original",sql.NullString{},sql.NullString{});if e!=nil{t.Fatal(e)};return m}
func rows(t *testing.T,db *sql.DB,table string)int{
 t.Helper()
 if table!="chat_messages"&&table!="chat_conversations"{t.Fatal("invalid table")}
 var n int
 if err:=db.QueryRow("SELECT count(*) FROM "+table).Scan(&n);err!=nil{t.Fatal(err)}
 return n
}
func TestGooseRoundTrip(t *testing.T){
 db,s:=database(t);assign(t,s,recipientA);send(t,s,1)
 if err:=goose.Down(db,"migrations");err!=nil{t.Fatal(err)}
 if err:=goose.Down(db,"migrations");err!=nil{t.Fatal(err)}
 if err:=goose.Up(db,"migrations");err!=nil{t.Fatal(err)}
 if rows(t,db,"chat_messages")!=0{t.Fatal("down/up left messages")}
 var ver string;if err:=db.QueryRow("SHOW server_version").Scan(&ver);err!=nil{t.Fatal(err)}
 if ver!="18.6"{t.Fatalf("server_version=%s",ver)}
}
func TestMS07IdempotencyReassignment(t *testing.T){
 db,s:=database(t);assign(t,s,recipientA);first:=send(t,s,2)
 if first.Replayed||first.RecipientID!=recipientA{t.Fatalf("initial=%+v",first)}
 assign(t,s,recipientB)
 replay,err:=s.CommitMessage(context.Background(),owner,sender,k(2),"changed",sql.NullString{},sql.NullString{})
 if err!=nil{t.Fatal(err)}
 if !replay.Replayed||replay.ID!=first.ID||replay.RecipientID!=recipientA{t.Fatalf("first=%+v replay=%+v",first,replay)}
 var body string
 if err:=db.QueryRow("SELECT body FROM chat_messages WHERE id=$1",first.ID).Scan(&body);err!=nil{t.Fatal(err)}
 if body!="original"||rows(t,db,"chat_messages")!=1{t.Fatalf("replay mutated body=%q or duplicated",body)}
}
func TestMS07ConcurrentDuplicate(t *testing.T){
 db,s:=database(t);assign(t,s,recipientA)
 const n=12
 var wg sync.WaitGroup
 start:=make(chan struct{})
 results:=make([]storage.Message,n);errs:=make([]error,n)
 for i:=0;i<n;i++{wg.Add(1);go func(i int){defer wg.Done();<-start;results[i],errs[i]=s.CommitMessage(context.Background(),owner,sender,k(3),"body",sql.NullString{},sql.NullString{})}(i)}
 close(start);wg.Wait()
 created:=0
 for i,m:=range results{if errs[i]!=nil{t.Fatal(errs[i])};if m.ID!=results[0].ID||m.RecipientID!=recipientA{t.Fatalf("result[%d]=%+v",i,m)};if !m.Replayed{created++}}
 if created!=1||rows(t,db,"chat_messages")!=1{t.Fatalf("created=%d",created)}
}
func TestMS05ConcurrentAssignment(t *testing.T){
 db,s:=database(t);assign(t,s,recipientA)
 for i:=0;i<24;i++{
  start:=make(chan struct{});var wg sync.WaitGroup
  var msg storage.Message;var sendErr,assignmentErr error
  wg.Add(2)
  go func(n int){defer wg.Done();<-start;msg,sendErr=s.CommitMessage(context.Background(),owner,sender,k(n+100),"text",sql.NullString{},sql.NullString{})}(i)
  go func(){defer wg.Done();<-start;assignmentErr=s.CommitAssignment(context.Background(),owner,recipientB)}()
  close(start);wg.Wait()
  if sendErr!=nil||assignmentErr!=nil{t.Fatalf("send=%v assignment=%v",sendErr,assignmentErr)}
  if msg.Replayed||(msg.RecipientID!=recipientA&&msg.RecipientID!=recipientB){t.Fatalf("invalid serialization: %+v",msg)}
  assign(t,s,recipientA)
 }
 if rows(t,db,"chat_messages")!=24{t.Fatal("lost or duplicate messages")}
}
func TestMS11RollbacksAndRecovery(t *testing.T){
 db,s:=database(t)
 if _,err:=s.CommitMessage(context.Background(),owner,sender,k(201),"body",sql.NullString{},sql.NullString{});err==nil{t.Fatal("missing assignment accepted")}
 if rows(t,db,"chat_conversations")!=0{t.Fatal("failed send created conversation")}
 assign(t,s,recipientA)
 if err:=s.CommitAssignment(context.Background(),owner,"invalid-uuid");err==nil{t.Fatal("invalid recipient accepted")}
 var current string
 if err:=db.QueryRow("SELECT recipient_account_id::text FROM chat_assignments WHERE owner_account_id=$1",owner).Scan(&current);err!=nil{t.Fatal(err)}
 if current!=recipientA{t.Fatalf("failed assignment changed recipient: %s",current)}
 if _,err:=s.CommitMessage(context.Background(),owner,sender,k(202),"   ",sql.NullString{},sql.NullString{});err==nil{t.Fatal("blank message accepted")}
 if rows(t,db,"chat_conversations")!=0{t.Fatal("failed validation created conversation")}
 tx,err:=db.BeginTx(context.Background(),nil);if err!=nil{t.Fatal(err)}
 var updated string
 if err:=tx.QueryRow("SELECT recipient_account_id::text FROM chat_commit_assignment($1::uuid,$2::uuid)",owner,recipientB).Scan(&updated);err!=nil{_ =tx.Rollback();t.Fatal(err)}
 if err:=tx.Rollback();err!=nil{t.Fatal(err)}
 if err:=db.QueryRow("SELECT recipient_account_id::text FROM chat_assignments WHERE owner_account_id=$1",owner).Scan(&current);err!=nil{t.Fatal(err)}
 if current!=recipientA{t.Fatal("rollback did not restore assignment")}
 msg:=send(t,s,203);if msg.Replayed||msg.RecipientID!=recipientA{t.Fatalf("recovery=%+v",msg)}
}
func TestFC11DocumentPairAndScope(t *testing.T){
 db,s:=database(t);assign(t,s,recipientA)
 bad:=[]struct{name string;id,title sql.NullString}{
  {"title only",sql.NullString{},sql.NullString{String:"title",Valid:true}},
  {"id only",sql.NullString{String:"id",Valid:true},sql.NullString{}},
  {"blank id",sql.NullString{String:" ",Valid:true},sql.NullString{String:"title",Valid:true}},
 }
 for i,tc:=range bad{t.Run(tc.name,func(t *testing.T){_,err:=s.CommitMessage(context.Background(),owner,sender,k(300+i),"ok",tc.id,tc.title);if err==nil{t.Fatal("invalid pair accepted")}})}
 if rows(t,db,"chat_messages")!=0{t.Fatal("invalid writes")}
 doc:=sql.NullString{String:"doc",Valid:true};title:=sql.NullString{String:"title",Valid:true}
 first,err:=s.CommitMessage(context.Background(),owner,sender,k(310),"",doc,title);if err!=nil{t.Fatal(err)}
 other:="33333333-3333-4333-8333-333333333333"
 second,err:=s.CommitMessage(context.Background(),owner,other,k(310),"body",sql.NullString{},sql.NullString{});if err!=nil{t.Fatal(err)}
 if first.ID==second.ID||second.Replayed{t.Fatal("idempotency scope does not include sender")}
}
func TestFC11PropertyIdempotency(t *testing.T){
 db,s:=database(t);assign(t,s,recipientA)
 var sequence atomic.Int64
 rapid.Check(t,func(rt *rapid.T){
  n:=int(sequence.Add(1))+1000
  body:=rapid.StringMatching("[a-zA-Z0-9]{1,20}").Draw(rt,"body")
  assigned:=rapid.SampledFrom([]string{recipientA,recipientB}).Draw(rt,"recipient")
  if err:=s.CommitAssignment(context.Background(),owner,assigned);err!=nil{rt.Fatal(err)}
  first,err:=s.CommitMessage(context.Background(),owner,sender,k(n),body,sql.NullString{},sql.NullString{});if err!=nil{rt.Fatal(err)}
  next:=recipientB;if assigned==recipientB{next=recipientA}
  if err:=s.CommitAssignment(context.Background(),owner,next);err!=nil{rt.Fatal(err)}
  replay,err:=s.CommitMessage(context.Background(),owner,sender,k(n),"different",sql.NullString{},sql.NullString{});if err!=nil{rt.Fatal(err)}
  if first.Replayed||!replay.Replayed||replay.ID!=first.ID||replay.RecipientID!=assigned{rt.Fatalf("first=%+v replay=%+v",first,replay)}
  var stored string
  if err:=db.QueryRow("SELECT body FROM chat_messages WHERE id=$1",first.ID).Scan(&stored);err!=nil{rt.Fatal(err)}
  if stored!=body{rt.Fatalf("body changed: %q",stored)}
 })
}

//go:build integration

package storage_test

import (
 "context"
 "database/sql"
 "fmt"
 "path/filepath"
 "sync"
 "strings"
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
 // PostgreSQL 18 initializes with a temporary server before final startup.
 // A mapped port can be available while that bootstrap server is restarting.
 readyCtx,stop:=context.WithTimeout(context.Background(),45*time.Second)
 defer stop()
 var pingErr error
 for {
  pingErr=db.PingContext(readyCtx)
  if pingErr==nil {break}
  select {case <-readyCtx.Done():t.Fatalf("PostgreSQL readiness: %v",pingErr);case <-time.After(250*time.Millisecond):}
 }
 // Guard against transient bootstrap readiness: run a real SQL query.
 var version string
 if err:=db.QueryRowContext(readyCtx,"SHOW server_version").Scan(&version);err!=nil{t.Fatal(err)}
 if !strings.HasPrefix(version,"18.6"){t.Fatalf("wrong PostgreSQL version: %s",version)}
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
 if rows(t,db,"chat_messages")!=1{t.Fatal("Issue25 down/up must preserve legacy messages")}
 var ver string;if err:=db.QueryRow("SHOW server_version").Scan(&ver);err!=nil{t.Fatal(err)}
 if !strings.HasPrefix(ver,"18.6"){t.Fatalf("server_version=%s",ver)}
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

func TestMS05AssignmentCommitsBeforeWaitingSend(t *testing.T) {
 db,s:=database(t);assign(t,s,recipientA)
 tx,err:=db.BeginTx(context.Background(),nil);if err!=nil{t.Fatal(err)}
 committed:=false
 t.Cleanup(func(){if !committed{_ =tx.Rollback()}})
 var locked string
 if err:=tx.QueryRow("SELECT recipient_account_id::text FROM chat_commit_assignment($1::uuid,$2::uuid)",owner,recipientB).Scan(&locked);err!=nil{t.Fatal(err)}
 if locked!=recipientB{t.Fatalf("inside tx: %s",locked)}
 result:=make(chan struct{m storage.Message;err error},1)
 go func(){m,e:=s.CommitMessage(context.Background(),owner,sender,k(401),"after B",sql.NullString{},sql.NullString{});result<-struct{m storage.Message;err error}{m,e}}()
 // The owner lock remains held until commit; whichever point the sender starts,
 // it cannot resolve the old recipient after this transaction commits.
 if err:=tx.Commit();err!=nil{t.Fatal(err)}
 committed=true
 select {
 case got:=<-result:
  if got.err!=nil||got.m.Replayed||got.m.RecipientID!=recipientB{t.Fatalf("after assignment commit: %+v err=%v",got.m,got.err)}
 case <-time.After(15*time.Second):t.Fatal("blocked send did not recover")
 }
}
func TestMS11CancelledWaitingSendLeavesNoPartialState(t *testing.T){
 db,s:=database(t);assign(t,s,recipientA)
 tx,err:=db.BeginTx(context.Background(),nil);if err!=nil{t.Fatal(err)}
 defer func(){_ =tx.Rollback()}()
 var held string
 if err:=tx.QueryRow("SELECT recipient_account_id::text FROM chat_commit_assignment($1::uuid,$2::uuid)",owner,recipientB).Scan(&held);err!=nil{t.Fatal(err)}
 ctx,cancel:=context.WithTimeout(context.Background(),150*time.Millisecond)
 defer cancel()
 if _,err:=s.CommitMessage(ctx,owner,sender,k(402),"will cancel",sql.NullString{},sql.NullString{});err==nil{t.Fatal("blocked write ignored canceled context")}
 if rows(t,db,"chat_conversations")!=0||rows(t,db,"chat_messages")!=0{t.Fatal("canceled write partially committed")}
 if err:=tx.Rollback();err!=nil{t.Fatal(err)}
 var current string
 if err:=db.QueryRow("SELECT recipient_account_id::text FROM chat_assignments WHERE owner_account_id=$1",owner).Scan(&current);err!=nil{t.Fatal(err)}
 if current!=recipientA{t.Fatalf("rollback changed recipient: %s",current)}
 msg:=send(t,s,402)
 if msg.Replayed||msg.RecipientID!=recipientA{t.Fatalf("retry=%+v",msg)}
}

func TestMS11BackendDisconnectRollsBackAndRetries(t *testing.T) {
 db,s:=database(t)
 assign(t,s,recipientA)
 tx,err:=db.BeginTx(context.Background(),nil)
 if err!=nil{t.Fatal(err)}
 defer func(){_ =tx.Rollback()}()
 var pid int
 if err:=tx.QueryRow("SELECT pg_backend_pid()").Scan(&pid);err!=nil{t.Fatal(err)}
 var recipient string
 if err:=tx.QueryRow("SELECT recipient_account_id::text FROM chat_commit_assignment($1::uuid,$2::uuid)",owner,recipientB).Scan(&recipient);err!=nil{t.Fatal(err)}
 if recipient!=recipientB{t.Fatalf("within transaction: %s",recipient)}
 var killed bool
 if err:=db.QueryRow("SELECT pg_terminate_backend($1)",pid).Scan(&killed);err!=nil{t.Fatal(err)}
 if !killed{t.Fatal("backend was not terminated")}
 if err:=tx.Commit();err==nil{t.Fatal("terminated transaction unexpectedly committed")}
 var saved string
 if err:=db.QueryRow("SELECT recipient_account_id::text FROM chat_assignments WHERE owner_account_id=$1",owner).Scan(&saved);err!=nil{t.Fatal(err)}
 if saved!=recipientA{t.Fatalf("terminated transaction changed assignment: %s",saved)}
 first:=send(t,s,700)
 replay,err:=s.CommitMessage(context.Background(),owner,sender,k(700),"network retry",sql.NullString{},sql.NullString{})
 if err!=nil{t.Fatal(err)}
 if first.Replayed||!replay.Replayed||replay.ID!=first.ID||replay.RecipientID!=recipientA||rows(t,db,"chat_messages")!=1 {
  t.Fatalf("retry after backend loss: first=%+v replay=%+v",first,replay)
 }
}
func TestFC11FC12InvalidUUIDsAndFailedDB(t *testing.T){
 db,s:=database(t)
 tests:=[]struct{name string;run func()error}{
  {"bad owner message",func()error{_,e:=s.CommitMessage(context.Background(),"invalid",sender,k(711),"text",sql.NullString{},sql.NullString{});return e}},
  {"bad sender message",func()error{_,e:=s.CommitMessage(context.Background(),owner,"invalid",k(712),"text",sql.NullString{},sql.NullString{});return e}},
  {"bad key message",func()error{_,e:=s.CommitMessage(context.Background(),owner,sender,"invalid","text",sql.NullString{},sql.NullString{});return e}},
  {"bad owner assignment",func()error{return s.CommitAssignment(context.Background(),"invalid",recipientA)}},
  {"bad recipient assignment",func()error{return s.CommitAssignment(context.Background(),owner,"invalid")}},
 }
 for _,tc:=range tests{t.Run(tc.name,func(t *testing.T){if err:=tc.run();err==nil{t.Fatal("invalid UUID accepted")}})}
 if rows(t,db,"chat_messages")!=0||rows(t,db,"chat_conversations")!=0{t.Fatal("invalid UUIDs wrote rows")}
 assign(t,s,recipientA)
 if err:=db.Close();err!=nil{t.Fatal(err)}
 if _,err:=s.CommitMessage(context.Background(),owner,sender,k(713),"message",sql.NullString{},sql.NullString{});err==nil{t.Fatal("closed DB allowed send")}
 if err:=s.CommitAssignment(context.Background(),owner,recipientB);err==nil{t.Fatal("closed DB allowed assignment")}
}

func TestDPM02MS05SimultaneousReassignmentsAndSends(t *testing.T){
 db,s:=database(t)
 assign(t,s,recipientA)
 const count=32
 start:=make(chan struct{})
 var wg sync.WaitGroup
 errs:=make([]error,count*2)
 messages:=make([]storage.Message,count)
 for i:=0;i<count;i++{
  wg.Add(2)
  go func(i int){defer wg.Done();<-start;to:=recipientA;if i%2!=0{to=recipientB};errs[2*i]=s.CommitAssignment(context.Background(),owner,to)}(i)
  go func(i int){defer wg.Done();<-start;messages[i],errs[2*i+1]=s.CommitMessage(context.Background(),owner,sender,k(2000+i),"concurrent",sql.NullString{},sql.NullString{})}(i)
 }
 close(start)
 wg.Wait()
 for i,err:=range errs{if err!=nil{t.Fatalf("operation %d: %v",i,err)}}
 seen:=make(map[int64]bool)
 for i,m:=range messages{
  if m.Replayed||m.ID==0||(m.RecipientID!=recipientA&&m.RecipientID!=recipientB)||seen[m.ID]{
   t.Fatalf("lost, duplicate or invalid recipient for message %d: %+v",i,m)
  }
  seen[m.ID]=true
  var persisted string
  if err:=db.QueryRow("SELECT recipient_account_id::text FROM chat_messages WHERE id=$1",m.ID).Scan(&persisted);err!=nil{t.Fatal(err)}
  if persisted!=m.RecipientID{t.Fatalf("message %d shifted recipient %s -> %s",i,m.RecipientID,persisted)}
 }
 if rows(t,db,"chat_messages")!=count{t.Fatal("concurrent sends lost or duplicated")}
 var assignments int
 if err:=db.QueryRow("SELECT count(*) FROM chat_assignments WHERE owner_account_id=$1",owner).Scan(&assignments);err!=nil{t.Fatal(err)}
 if assignments!=1{t.Fatalf("multiple final assignments: %d",assignments)}
}
func TestDPM02MS11AssignmentRollbackVisibleToNextSend(t *testing.T){
 db,s:=database(t)
 assign(t,s,recipientA)
 tx,err:=db.BeginTx(context.Background(),nil)
 if err!=nil{t.Fatal(err)}
 defer func(){_ =tx.Rollback()}()
 var changed string
 if err:=tx.QueryRow("SELECT recipient_account_id::text FROM chat_commit_assignment($1::uuid,$2::uuid)",owner,recipientB).Scan(&changed);err!=nil{t.Fatal(err)}
 if changed!=recipientB{t.Fatalf("transaction did not stage B: %s",changed)}
 if err:=tx.Rollback();err!=nil{t.Fatal(err)}
 var current string
 if err:=db.QueryRow("SELECT recipient_account_id::text FROM chat_assignments WHERE owner_account_id=$1",owner).Scan(&current);err!=nil{t.Fatal(err)}
 if current!=recipientA{t.Fatalf("rollback changed persisted assignment: %s",current)}
 m:=send(t,s,2700)
 if m.Replayed||m.RecipientID!=recipientA{t.Fatalf("post-rollback send used invalid recipient: %+v",m)}
 if rows(t,db,"chat_messages")!=1{t.Fatal("lost or duplicate message after rollback")}
}

func TestDPM02FC04ReadCurrentAssignmentFromPostgres(t *testing.T){
 db,s:=database(t)
 got,err:=s.GetAssignment(context.Background(),owner)
 if err!=sql.ErrNoRows||got!=""{t.Fatalf("expected NO_ASSIGNMENT storage sentinel: %q %v",got,err)}
 for _,to:=range []string{recipientA,recipientB,recipientA}{
  assign(t,s,to)
  got,err=s.GetAssignment(context.Background(),owner)
  if err!=nil||got!=to{t.Fatalf("stale recipient: got=%s want=%s err=%v",got,to,err)}
 }
 if rows(t,db,"chat_conversations")!=0||rows(t,db,"chat_messages")!=0{
  t.Fatal("FC-04/FC-12 created messages or auto-created conversation")
 }
}

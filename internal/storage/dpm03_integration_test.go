//go:build integration

package storage_test

import (
 "context"
 "database/sql"
 "sync"
 "testing"
 "time"

)

func TestDPM03FC06HistoryCursorIsolationAndReadOnly(t *testing.T) {
 db,s:=database(t)
 history,err:=s.ReadHistory(context.Background(),owner,0,100)
 if err!=nil||len(history)!=0{t.Fatalf("empty history=%v err=%v",history,err)}
 if rows(t,db,"chat_conversations")!=0{t.Fatal("read created conversation")}
 assign(t,s,recipientA)
 first:=send(t,s,8101)
 second:=send(t,s,8102)
 third:=send(t,s,8103)
 one,err:=s.ReadHistory(context.Background(),owner,0,2)
 if err!=nil||len(one)!=2||one[0].ID!=first.ID||one[1].ID!=second.ID{t.Fatalf("keyset first page=%+v err=%v",one,err)}
 rest,err:=s.ReadHistory(context.Background(),owner,second.ID,2)
 if err!=nil||len(rest)!=1||rest[0].ID!=third.ID{t.Fatalf("keyset second page=%+v err=%v",rest,err)}
 for _,m:=range append(one,rest...){
  if m.OwnerID!=owner||m.SenderID!=sender||m.RecipientID!=recipientA||m.Body!="original"{
   t.Fatalf("history mismatch: %+v",m)
  }
 }
 outsider:="99999999-9999-4999-8999-999999999999"
 other,err:=s.ReadHistory(context.Background(),outsider,0,100)
 if err!=nil||len(other)!=0{t.Fatalf("cross-owner leak: %+v %v",other,err)}
 if rows(t,db,"chat_messages")!=3{t.Fatal("read mutated storage")}
}
func TestDPM03FC05ReplayReassignmentAndRollback(t *testing.T){
 db,s:=database(t)
 assign(t,s,recipientA)
 original:=send(t,s,8201)
 assign(t,s,recipientB)
 replay,err:=s.CommitMessage(context.Background(),owner,sender,k(8201),"modified",sql.NullString{},sql.NullString{})
 if err!=nil||!replay.Replayed||replay.ID!=original.ID||replay.RecipientID!=recipientA{t.Fatalf("replay=%+v original=%+v err=%v",replay,original,err)}
 tx,err:=db.BeginTx(context.Background(),nil);if err!=nil{t.Fatal(err)}
 var changed string
 if err:=tx.QueryRow("SELECT recipient_account_id::text FROM chat_commit_assignment($1::uuid,$2::uuid)",owner,recipientA).Scan(&changed);err!=nil{_ =tx.Rollback();t.Fatal(err)}
 if err:=tx.Rollback();err!=nil{t.Fatal(err)}
 current,err:=s.GetAssignment(context.Background(),owner)
 if err!=nil||current!=recipientB{t.Fatalf("rollback changed assignment %s %v",current,err)}
 fresh:=send(t,s,8202)
 if fresh.Replayed||fresh.RecipientID!=recipientB{t.Fatalf("fresh message after rollback=%+v",fresh)}
 history,err:=s.ReadHistory(context.Background(),owner,0,100)
 if err!=nil||len(history)!=2||history[0].RecipientID!=recipientA||history[1].RecipientID!=recipientB||history[0].Body!="original"{t.Fatalf("history=%+v err=%v",history,err)}
}
func TestDPM03ConcurrentHistoryReadsAndSends(t *testing.T){
 db,s:=database(t)
 assign(t,s,recipientA)
 const count=16
 start:=make(chan struct{})
 var wg sync.WaitGroup
 failures:=make(chan error,count*2)
 for i:=0;i<count;i++{
  wg.Add(2)
  go func(i int){
   defer wg.Done();<-start
   _,err:=s.CommitMessage(context.Background(),owner,sender,k(8300+i),"concurrent",sql.NullString{},sql.NullString{})
   if err!=nil{failures<-err}
  }(i)
  go func(){
   defer wg.Done();<-start
   ctx,cancel:=context.WithTimeout(context.Background(),10*time.Second);defer cancel()
   history,err:=s.ReadHistory(ctx,owner,0,100)
   if err!=nil{failures<-err;return}
   for _,m:=range history{if m.OwnerID!=owner||m.RecipientID!=recipientA{failures<-sql.ErrNoRows;return}}
  }()
 }
 close(start);wg.Wait();close(failures)
 for err:=range failures{t.Fatalf("concurrent read/send: %v",err)}
 history,err:=s.ReadHistory(context.Background(),owner,0,100)
 if err!=nil||len(history)!=count||rows(t,db,"chat_messages")!=count{t.Fatalf("missing messages=%d err=%v",len(history),err)}
}
func TestDPM03InvalidHistoryBoundariesAndRecovery(t *testing.T){
 db,s:=database(t)
 for _,tc:=range []struct{owner string;after int64;limit int}{{"",0,10},{owner,-1,10},{owner,0,0},{owner,0,101}}{
  if _,err:=s.ReadHistory(context.Background(),tc.owner,tc.after,tc.limit);err==nil{t.Fatalf("accepted invalid %+v",tc)}
 }
 if _,err:=s.CommitMessage(context.Background(),owner,sender,k(8401),"no recipient",sql.NullString{},sql.NullString{});err==nil{t.Fatal("unassigned send succeeded")}
 if rows(t,db,"chat_messages")!=0||rows(t,db,"chat_conversations")!=0{t.Fatal("failed send persisted partial state")}
 assign(t,s,recipientA)
 m:=send(t,s,8401)
 if m.Replayed||m.RecipientID!=recipientA{t.Fatalf("retry=%+v",m)}
}

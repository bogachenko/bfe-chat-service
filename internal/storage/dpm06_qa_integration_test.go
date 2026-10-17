//go:build integration

package storage_test

import (
 "context"
 "database/sql"
 "testing"
)

func TestIndependentDPM06DocumentHistoryReplayAndReassignment(t *testing.T) {
 db,s:=database(t)
 assign(t,s,recipientA)
 firstID:=sql.NullString{String:"file-A",Valid:true}
 firstTitle:=sql.NullString{String:"snapshot A",Valid:true}
 first,err:=s.CommitMessage(context.Background(),owner,sender,k(9901),"first",firstID,firstTitle)
 if err!=nil||first.Replayed{t.Fatalf("first commit=%+v %v",first,err)}
 assign(t,s,recipientB)
 second,err:=s.CommitMessage(context.Background(),owner,sender,k(9902),"",sql.NullString{String:"file-B",Valid:true},sql.NullString{String:"snapshot B",Valid:true})
 if err!=nil||second.Replayed{t.Fatalf("second commit=%+v %v",second,err)}
 plain,err:=s.CommitMessage(context.Background(),owner,sender,k(9903),"ordinary chat",sql.NullString{},sql.NullString{})
 if err!=nil||plain.Replayed{t.Fatalf("plain commit=%+v %v",plain,err)}
 replay,err:=s.CommitMessage(context.Background(),owner,sender,k(9901),"changed on retry",sql.NullString{String:"replacement",Valid:true},sql.NullString{String:"replacement",Valid:true})
 if err!=nil||!replay.Replayed||replay.ID!=first.ID||replay.RecipientID!=first.RecipientID{t.Fatalf("replay=%+v %v",replay,err)}
 history,err:=s.ReadHistory(context.Background(),owner,0,100)
 if err!=nil||len(history)!=3{t.Fatalf("history=%+v %v",history,err)}
 if history[0].DocumentID.String!="file-A"||history[0].DocumentTitle.String!="snapshot A"||history[0].Body!="first"||history[0].RecipientID!=recipientA{t.Fatalf("replay changed saved first reference: %+v",history[0])}
 if history[1].DocumentID.String!="file-B"||history[1].DocumentTitle.String!="snapshot B"||history[1].RecipientID!=recipientB{t.Fatalf("second reference changed: %+v",history[1])}
 if history[2].DocumentID.Valid||history[2].DocumentTitle.Valid{t.Fatalf("ordinary message acquired document: %+v",history[2])}
 if rows(t,db,"chat_messages")!=3{t.Fatal("idempotent retry inserted additional message")}
}

func TestIndependentDPM06RejectsPartialReferenceInDatabase(t *testing.T) {
 db,s:=database(t)
 assign(t,s,recipientA)
 _,err:=s.CommitMessage(context.Background(),owner,sender,k(9911),"body",sql.NullString{String:"id",Valid:true},sql.NullString{})
 if err==nil{t.Fatal("accepted reference with missing title")}
 if rows(t,db,"chat_messages")!=0{t.Fatal("invalid reference persisted")}
 _,err=s.CommitMessage(context.Background(),owner,sender,k(9912),"body",sql.NullString{String:"id",Valid:true},sql.NullString{String:"",Valid:true})
 if err==nil{t.Fatal("accepted blank document title")}
 if rows(t,db,"chat_messages")!=0{t.Fatal("blank title persisted")}
}

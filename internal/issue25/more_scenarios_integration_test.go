package issue25

import (
 "context"
 "errors"
 "testing"
)

func TestPGMS2504RemoveFirstRejectsSendWithoutOrdinalGap(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,member:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"remove-first",[]string{member});if err!=nil{t.Fatal(err)}
 if err=s.Remove(ctx,group,owner,member);err!=nil{t.Fatal(err)}
 _,err=s.Send(ctx,group,member,fixtureID(t),Payload{Text:"denied"})
 if !errors.Is(err,ErrForbidden){t.Fatalf("expected FORBIDDEN got %v",err)}
 var next int64
 if err=db.QueryRowContext(ctx,`SELECT next_ordinal FROM issue25_conversations WHERE id=$1`,group).Scan(&next);err!=nil{t.Fatal(err)}
 if next!=1{t.Fatalf("counter advanced after forbidden SEND: %d",next)}
 m,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:"owner-1"});if err!=nil{t.Fatal(err)}
 if m.Ordinal!=1{t.Fatalf("owner ordinal=%d",m.Ordinal)}
 if _,err=s.ReadHistory(ctx,group,member,0,50);!errors.Is(err,ErrForbidden){t.Fatalf("removed history=%v",err)}
 var n int
 if err=db.QueryRowContext(ctx,`SELECT count(*) FROM issue25_messages WHERE conversation_id=$1`,group).Scan(&n);err!=nil{t.Fatal(err)}
 if n!=1{t.Fatalf("messages=%d",n)}
}
func TestPGMS2506CanonicalReplayAndConflict(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 sender:=fixtureID(t)
 group,err:=s.CreateGroup(ctx,sender,"idempotency",nil);if err!=nil{t.Fatal(err)}
 key:=fixtureID(t)
 first,err:=s.Send(ctx,group,sender,key,Payload{Text:"abc"});if err!=nil{t.Fatal(err)}
 same,err:=s.Send(ctx,group,sender,key,Payload{Text:"abc"});if err!=nil{t.Fatal(err)}
 if !same.Replayed||first.ID!=same.ID||first.Ordinal!=same.Ordinal{t.Fatalf("replay first=%+v same=%+v",first,same)}
 if _,err=s.Send(ctx,group,sender,key,Payload{Text:"abd"});!errors.Is(err,ErrConflict){t.Fatalf("expected CONFLICT_409, got %v",err)}
 var rows int
 if err=db.QueryRowContext(ctx,`SELECT count(*) FROM issue25_messages WHERE conversation_id=$1`,group).Scan(&rows);err!=nil{t.Fatal(err)}
 if rows!=1{t.Fatalf("idempotent message rows=%d",rows)}
 var next int64
 if err=db.QueryRowContext(ctx,`SELECT next_ordinal FROM issue25_conversations WHERE id=$1`,group).Scan(&next);err!=nil{t.Fatal(err)}
 if next!=2{t.Fatalf("unexpected next ordinal %d",next)}
}

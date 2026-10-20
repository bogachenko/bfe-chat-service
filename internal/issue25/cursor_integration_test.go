package issue25

import (
 "context"
 "errors"
 "testing"
 "database/sql"
)

func TestPGMS2507RollbackDoesNotAdvanceCommittedOrdinal(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner:=fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"cursor-order",nil);if err!=nil{t.Fatal(err)}
 forced:=errors.New("deliberate rollback")
 err=s.tx(ctx,group,func(tx *sql.Tx)error{
  var ordinal int64
  if e:=tx.QueryRowContext(ctx,`UPDATE issue25_conversations SET next_ordinal=next_ordinal+1
  WHERE id=$1 RETURNING next_ordinal-1`,group).Scan(&ordinal);e!=nil{return e}
  if ordinal!=1{t.Fatalf("uncommitted ordinal=%d",ordinal)}
  return forced
 })
 if !errors.Is(err,forced){t.Fatalf("rollback err=%v",err)}
 a,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:"first committed"})
 if err!=nil||a.Ordinal!=1{t.Fatalf("after rollback %+v %v",a,err)}
 b,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:"second committed"})
 if err!=nil||b.Ordinal!=2{t.Fatalf("second %+v %v",b,err)}
 first,err:=s.ReadHistoryDetailed(ctx,group,owner,0,1)
 if err!=nil||len(first)!=1||first[0].Ordinal!=1{t.Fatalf("page1 %+v %v",first,err)}
 second,err:=s.ReadHistoryDetailed(ctx,group,owner,first[0].Ordinal,1)
 if err!=nil||len(second)!=1||second[0].Ordinal!=2{t.Fatalf("page2 %+v %v",second,err)}
 end,err:=s.ReadHistoryDetailed(ctx,group,owner,second[0].Ordinal,1)
 if err!=nil||len(end)!=0{t.Fatalf("page3 %+v %v",end,err)}
 var counter int64
 if err=db.QueryRowContext(ctx,`SELECT next_ordinal FROM issue25_conversations WHERE id=$1`,group).Scan(&counter);err!=nil{t.Fatal(err)}
 if counter!=3{t.Fatalf("next committed ordinal=%d",counter)}
}

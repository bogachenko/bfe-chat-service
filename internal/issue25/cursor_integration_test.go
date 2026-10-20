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
 group,err:=s.CreateGroup(ctx,owner,"cursor-order",nil);qaRequire(t, !(err!=nil), err)
 forced:=errors.New("deliberate rollback")
 err=s.tx(ctx,group,func(tx *sql.Tx)error{
  var ordinal int64
  if e:=tx.QueryRowContext(ctx,`UPDATE issue25_conversations SET next_ordinal=next_ordinal+1
  WHERE id=$1 RETURNING next_ordinal-1`,group).Scan(&ordinal);e!=nil{return e}
  qaRequiref(t, !(ordinal!=1), "uncommitted ordinal=%d",ordinal)
  return forced
 })
 qaRequiref(t, !(!errors.Is(err,forced)), "rollback err=%v",err)
 a,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:"first committed"})
 qaRequiref(t, !(err!=nil||a.Ordinal!=1), "after rollback %+v %v",a,err)
 b,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:"second committed"})
 qaRequiref(t, !(err!=nil||b.Ordinal!=2), "second %+v %v",b,err)
 first,err:=s.ReadHistoryDetailed(ctx,group,owner,0,1)
 qaRequiref(t, !(err!=nil||len(first)!=1||first[0].Ordinal!=1), "page1 %+v %v",first,err)
 second,err:=s.ReadHistoryDetailed(ctx,group,owner,first[0].Ordinal,1)
 qaRequiref(t, !(err!=nil||len(second)!=1||second[0].Ordinal!=2), "page2 %+v %v",second,err)
 end,err:=s.ReadHistoryDetailed(ctx,group,owner,second[0].Ordinal,1)
 qaRequiref(t, !(err!=nil||len(end)!=0), "page3 %+v %v",end,err)
 var counter int64
 func() { err=db.QueryRowContext(ctx,`SELECT next_ordinal FROM issue25_conversations WHERE id=$1`,group).Scan(&counter); qaRequire(t, !(err!=nil), err) }()
 qaRequiref(t, !(counter!=3), "next committed ordinal=%d",counter)
}

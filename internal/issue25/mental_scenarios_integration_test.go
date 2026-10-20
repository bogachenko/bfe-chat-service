package issue25

import (
 "context"
 "fmt"
 "testing"
)

func TestPGMS2503MembershipHistory(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,viewer:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"MS25-03 fixture",nil);qaRequire(t, !(err!=nil), err)
 send:=func(expected int64){
  t.Helper()
  m,e:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:fmt.Sprintf("m-%d",expected)})
  qaRequiref(t, !(e!=nil||m.Ordinal!=expected), "SEND expected %d got %+v %v",expected,m,e)
 }
 for i:=int64(1);i<=9;i++{send(i)}
 e1,err:=s.Join(ctx,group,owner,viewer,"member");qaRequire(t, !(err!=nil), err)
 for i:=int64(10);i<=19;i++{send(i)}
 func() { err=s.Remove(ctx,group,owner,viewer); qaRequire(t, !(err!=nil), err) }()
 for i:=int64(20);i<=29;i++{send(i)}
 e2,err:=s.Join(ctx,group,owner,viewer,"member");qaRequire(t, !(err!=nil), err)
 qaRequire(t, !(e1==e2), "reused epoch")
 send(30);send(31)
 items,err:=s.ReadHistoryDetailed(ctx,group,viewer,0,50);qaRequire(t, !(err!=nil), err)
 want:=[]int64{10,11,12,13,14,15,16,17,18,19,30,31}
 qaRequiref(t, !(len(items)!=len(want)), "visible count=%d, expected12",len(items))
 for i,ord:=range want{qaRequiref(t, !(items[i].Ordinal!=ord), "at %d ordinal=%d expected %d",i,items[i].Ordinal,ord)}
 rows,err:=db.QueryContext(ctx,`SELECT epoch::text,join_boundary,leave_boundary FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 ORDER BY join_boundary`,group,viewer)
 qaRequire(t, !(err!=nil), err);defer func() { if err := rows.Close(); err != nil { t.Errorf("close membership rows: %v", err) } }()
 var epoch string;var join int64;var leave *int64
 qaRequire(t, !(!rows.Next()), "missing E1")
 func() { err=rows.Scan(&epoch,&join,&leave); qaRequire(t, !(err!=nil), err) }()
 qaRequiref(t, !(epoch!=e1||join!=10||leave==nil||*leave!=20), "E1 %s [%d,%v)",epoch,join,leave)
 qaRequire(t, !(!rows.Next()), "missing E2")
 func() { err=rows.Scan(&epoch,&join,&leave); qaRequire(t, !(err!=nil), err) }()
 qaRequiref(t, !(epoch!=e2||join!=30||leave!=nil), "E2 %s [%d,%v)",epoch,join,leave)
}
func TestPGMS2505TransferOwnerAndLeave(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 old,next:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,old,"MS25-05",[]string{next});qaRequire(t, !(err!=nil), err)
 func() { err=s.Remove(ctx,group,old,old); qaRequire(t, !(err==nil), "owner left before transfer") }()
 func() { err=s.TransferOwner(ctx,group,old,next); qaRequire(t, !(err!=nil), err) }()
 func() { err=s.Remove(ctx,group,old,old); qaRequire(t, !(err!=nil), err) }()
 var activeOwners int
 func() { err=db.QueryRowContext(ctx,`SELECT count(*) FROM issue25_memberships WHERE conversation_id=$1 AND role='owner' AND leave_boundary IS NULL`,group).Scan(&activeOwners); qaRequire(t, !(err!=nil), err) }()
 qaRequiref(t, !(activeOwners!=1), "active owners=%d",activeOwners)
 var role string
 func() { err=db.QueryRowContext(ctx,`SELECT role FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,group,next).Scan(&role); qaRequiref(t, !(err!=nil||role!="owner"), "successor role %q: %v",role,err) }()
}

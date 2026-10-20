package issue25

import (
 "context"
 "database/sql"
 "fmt"
 "testing"
)

func TestPGMS2503MembershipHistory(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,viewer:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"MS25-03 fixture",nil);qaReject(t, (err!=nil), err)
 send:=func(expected int64){
  t.Helper()
  m,e:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:fmt.Sprintf("m-%d",expected)})
  qaRejectf(t, (e!=nil||m.Ordinal!=expected), "SEND expected %d got %+v %v",expected,m,e)
 }
 for i:=int64(1);i<=9;i++{send(i)}
 e1,err:=s.Join(ctx,group,owner,viewer,"member");qaReject(t, (err!=nil), err)
 for i:=int64(10);i<=19;i++{send(i)}
 func() { err=s.Remove(ctx,group,owner,viewer); qaReject(t, (err!=nil), err) }()
 for i:=int64(20);i<=29;i++{send(i)}
 e2,err:=s.Join(ctx,group,owner,viewer,"member");qaReject(t, (err!=nil), err)
 qaReject(t, (e1==e2), "reused epoch")
 send(30);send(31)
 items,err:=s.ReadHistoryDetailed(ctx,group,viewer,0,50);qaReject(t, (err!=nil), err)
 want:=[]int64{10,11,12,13,14,15,16,17,18,19,30,31}
 qaRejectf(t, (len(items)!=len(want)), "visible count=%d, expected12",len(items))
 for i,ord:=range want{qaRejectf(t, (items[i].Ordinal!=ord), "at %d ordinal=%d expected %d",i,items[i].Ordinal,ord)}
 qaVerifyMembershipEpochRows(t, db, ctx, group, viewer, e1, e2)
}
func TestPGMS2505TransferOwnerAndLeave(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 old,next:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,old,"MS25-05",[]string{next});qaReject(t, (err!=nil), err)
 func() { err=s.Remove(ctx,group,old,old); qaReject(t, (err==nil), "owner left before transfer") }()
 func() { err=s.TransferOwner(ctx,group,old,next); qaReject(t, (err!=nil), err) }()
 func() { err=s.Remove(ctx,group,old,old); qaReject(t, (err!=nil), err) }()
 var activeOwners int
 func() { err=db.QueryRowContext(ctx,`SELECT count(*) FROM issue25_memberships WHERE conversation_id=$1 AND role='owner' AND leave_boundary IS NULL`,group).Scan(&activeOwners); qaReject(t, (err!=nil), err) }()
 qaRejectf(t, (activeOwners!=1), "active owners=%d",activeOwners)
 var role string
 func() { err=db.QueryRowContext(ctx,`SELECT role FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,group,next).Scan(&role); qaRejectf(t, (err!=nil||role!="owner"), "successor role %q: %v",role,err) }()
}

func qaVerifyMembershipEpochRows(t *testing.T, db *sql.DB, ctx context.Context, group, viewer, e1, e2 string) {
 rows,err:=db.QueryContext(ctx,`SELECT epoch::text,join_boundary,leave_boundary FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 ORDER BY join_boundary`,group,viewer)
 qaReject(t, (err!=nil), err);defer func() { if err := rows.Close(); err != nil { t.Errorf("close membership rows: %v", err) } }()
 var epoch string;var join int64;var leave *int64
 qaReject(t, (!rows.Next()), "missing E1")
 func() { err=rows.Scan(&epoch,&join,&leave); qaReject(t, (err!=nil), err) }()
 qaRejectf(t, (epoch!=e1||join!=10||leave==nil||*leave!=20), "E1 %s [%d,%v)",epoch,join,leave)
 qaReject(t, (!rows.Next()), "missing E2")
 func() { err=rows.Scan(&epoch,&join,&leave); qaReject(t, (err!=nil), err) }()
 qaRejectf(t, (epoch!=e2||join!=30||leave!=nil), "E2 %s [%d,%v)",epoch,join,leave)
}

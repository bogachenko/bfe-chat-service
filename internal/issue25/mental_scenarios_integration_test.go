package issue25

import (
 "context"
 "fmt"
 "testing"
)

func TestPGMS2503MembershipHistory(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,viewer:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"MS25-03 fixture",nil);if err!=nil{t.Fatal(err)}
 send:=func(expected int64){
  t.Helper()
  m,e:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:fmt.Sprintf("m-%d",expected)})
  if e!=nil||m.Ordinal!=expected{t.Fatalf("SEND expected %d got %+v %v",expected,m,e)}
 }
 for i:=int64(1);i<=9;i++{send(i)}
 e1,err:=s.Join(ctx,group,owner,viewer,"member");if err!=nil{t.Fatal(err)}
 for i:=int64(10);i<=19;i++{send(i)}
 if err=s.Remove(ctx,group,owner,viewer);err!=nil{t.Fatal(err)}
 for i:=int64(20);i<=29;i++{send(i)}
 e2,err:=s.Join(ctx,group,owner,viewer,"member");if err!=nil{t.Fatal(err)}
 if e1==e2{t.Fatal("reused epoch")}
 send(30);send(31)
 items,err:=s.ReadHistoryDetailed(ctx,group,viewer,0,50);if err!=nil{t.Fatal(err)}
 want:=[]int64{10,11,12,13,14,15,16,17,18,19,30,31}
 if len(items)!=len(want){t.Fatalf("visible count=%d, expected12",len(items))}
 for i,ord:=range want{if items[i].Ordinal!=ord{t.Fatalf("at %d ordinal=%d expected %d",i,items[i].Ordinal,ord)}}
 rows,err:=db.QueryContext(ctx,`SELECT epoch::text,join_boundary,leave_boundary FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 ORDER BY join_boundary`,group,viewer)
 if err!=nil{t.Fatal(err)};defer rows.Close()
 var epoch string;var join int64;var leave *int64
 if !rows.Next(){t.Fatal("missing E1")}
 if err=rows.Scan(&epoch,&join,&leave);err!=nil{t.Fatal(err)}
 if epoch!=e1||join!=10||leave==nil||*leave!=20{t.Fatalf("E1 %s [%d,%v)",epoch,join,leave)}
 if !rows.Next(){t.Fatal("missing E2")}
 if err=rows.Scan(&epoch,&join,&leave);err!=nil{t.Fatal(err)}
 if epoch!=e2||join!=30||leave!=nil{t.Fatalf("E2 %s [%d,%v)",epoch,join,leave)}
}
func TestPGMS2505TransferOwnerAndLeave(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 old,next:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,old,"MS25-05",[]string{next});if err!=nil{t.Fatal(err)}
 if err=s.Remove(ctx,group,old,old);err==nil{t.Fatal("owner left before transfer")}
 if err=s.TransferOwner(ctx,group,old,next);err!=nil{t.Fatal(err)}
 if err=s.Remove(ctx,group,old,old);err!=nil{t.Fatal(err)}
 var activeOwners int
 if err=db.QueryRowContext(ctx,`SELECT count(*) FROM issue25_memberships WHERE conversation_id=$1 AND role='owner' AND leave_boundary IS NULL`,group).Scan(&activeOwners);err!=nil{t.Fatal(err)}
 if activeOwners!=1{t.Fatalf("active owners=%d",activeOwners)}
 var role string
 if err=db.QueryRowContext(ctx,`SELECT role FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,group,next).Scan(&role);err!=nil||role!="owner"{t.Fatalf("successor role %q: %v",role,err)}
}

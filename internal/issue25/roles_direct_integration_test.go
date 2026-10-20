package issue25

import (
 "context"
 "errors"
 "testing"
)

func TestPGMS2502OwnerAdminMemberPermissions(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,admin,member,guest:=fixtureID(t),fixtureID(t),fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"roles",[]string{admin,member});if err!=nil{t.Fatal(err)}
 if err:=s.ChangeAdmin(ctx,group,member,admin,true);!errors.Is(err,ErrForbidden){t.Fatalf("member promoted admin: %v",err)}
 if err:=s.ChangeAdmin(ctx,group,owner,admin,true);err!=nil{t.Fatal(err)}
 if _,err:=s.Join(ctx,group,admin,guest,"admin");!errors.Is(err,ErrForbidden){t.Fatalf("admin created another admin: %v",err)}
 if _,err:=s.Join(ctx,group,member,guest,"member");!errors.Is(err,ErrForbidden){t.Fatalf("member added user: %v",err)}
 if _,err:=s.Join(ctx,group,admin,guest,"member");err!=nil{t.Fatalf("admin failed to add ordinary member: %v",err)}
 if err:=s.Remove(ctx,group,admin,owner);!errors.Is(err,ErrForbidden){t.Fatalf("admin removed owner: %v",err)}
 if err:=s.Remove(ctx,group,admin,guest);err!=nil{t.Fatalf("admin cannot remove ordinary member: %v",err)}
 if err:=s.Remove(ctx,group,admin,admin);err!=nil{t.Fatalf("admin cannot leave itself: %v",err)}
 var remaining int
 if err:=db.QueryRowContext(ctx,`SELECT count(*) FROM issue25_memberships WHERE conversation_id=$1 AND leave_boundary IS NULL`,group).Scan(&remaining);err!=nil{t.Fatal(err)}
 if remaining!=2{t.Fatalf("remaining=%d, expected owner+member",remaining)}
 var owners int
 if err:=db.QueryRowContext(ctx,`SELECT count(*) FROM issue25_memberships WHERE conversation_id=$1 AND role='owner' AND leave_boundary IS NULL`,group).Scan(&owners);err!=nil{t.Fatal(err)}
 if owners!=1{t.Fatalf("owner count %d",owners)}
}
func TestPGMS2501DirectHideUnhideAndIncoming(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 a,b:=fixtureID(t),fixtureID(t)
 direct,err:=s.OpenDirect(ctx,a,b);if err!=nil{t.Fatal(err)}
 if _,err=s.OpenDirect(ctx,a,a);!errors.Is(err,ErrInvalid){t.Fatalf("self direct: %v",err)}
 if err:=s.HideDirect(ctx,direct,a,true);err!=nil{t.Fatal(err)}
 var hidden bool
 if err:=db.QueryRowContext(ctx,`SELECT hidden FROM issue25_user_state WHERE conversation_id=$1 AND account_id=$2`,direct,a).Scan(&hidden);err!=nil||!hidden{t.Fatalf("hidden true: %v %v",hidden,err)}
 if _,err=s.Send(ctx,direct,b,fixtureID(t),Payload{Text:"incoming"});err!=nil{t.Fatal(err)}
 if err:=db.QueryRowContext(ctx,`SELECT hidden FROM issue25_user_state WHERE conversation_id=$1 AND account_id=$2`,direct,a).Scan(&hidden);err!=nil||hidden{t.Fatalf("incoming should reopen: %v %v",hidden,err)}
 if err:=s.HideDirect(ctx,direct,a,true);err!=nil{t.Fatal(err)}
 if err:=s.HideDirect(ctx,direct,a,false);err!=nil{t.Fatal(err)}
 if err:=db.QueryRowContext(ctx,`SELECT hidden FROM issue25_user_state WHERE conversation_id=$1 AND account_id=$2`,direct,a).Scan(&hidden);err!=nil||hidden{t.Fatalf("explicit reopen: %v %v",hidden,err)}
}

package issue25

import (
 "context"
 "errors"
 "testing"
)

func TestPGMS2502OwnerAdminMemberPermissions(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,admin,member,guest:=fixtureID(t),fixtureID(t),fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"roles",[]string{admin,member});qaRequire(t, !(err!=nil), err)
 func() { err:=s.ChangeAdmin(ctx,group,member,admin,true); qaRequiref(t, !(!errors.Is(err,ErrForbidden)), "member promoted admin: %v",err) }()
 func() { err:=s.ChangeAdmin(ctx,group,owner,admin,true); qaRequire(t, !(err!=nil), err) }()
 func() { _,err:=s.Join(ctx,group,admin,guest,"admin"); qaRequiref(t, !(!errors.Is(err,ErrForbidden)), "admin created another admin: %v",err) }()
 func() { _,err:=s.Join(ctx,group,member,guest,"member"); qaRequiref(t, !(!errors.Is(err,ErrForbidden)), "member added user: %v",err) }()
 func() { _,err:=s.Join(ctx,group,admin,guest,"member"); qaRequiref(t, !(err!=nil), "admin failed to add ordinary member: %v",err) }()
 func() { err:=s.Remove(ctx,group,admin,owner); qaRequiref(t, !(!errors.Is(err,ErrForbidden)), "admin removed owner: %v",err) }()
 func() { err:=s.Remove(ctx,group,admin,guest); qaRequiref(t, !(err!=nil), "admin cannot remove ordinary member: %v",err) }()
 func() { err:=s.Remove(ctx,group,admin,admin); qaRequiref(t, !(err!=nil), "admin cannot leave itself: %v",err) }()
 var remaining int
 func() { err:=db.QueryRowContext(ctx,`SELECT count(*) FROM issue25_memberships WHERE conversation_id=$1 AND leave_boundary IS NULL`,group).Scan(&remaining); qaRequire(t, !(err!=nil), err) }()
 qaRequiref(t, !(remaining!=2), "remaining=%d, expected owner+member",remaining)
 var owners int
 func() { err:=db.QueryRowContext(ctx,`SELECT count(*) FROM issue25_memberships WHERE conversation_id=$1 AND role='owner' AND leave_boundary IS NULL`,group).Scan(&owners); qaRequire(t, !(err!=nil), err) }()
 qaRequiref(t, !(owners!=1), "owner count %d",owners)
}
func TestPGMS2501DirectHideUnhideAndIncoming(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 a,b:=fixtureID(t),fixtureID(t)
 direct,err:=s.OpenDirect(ctx,a,b);qaRequire(t, !(err!=nil), err)
 func() { _,err=s.OpenDirect(ctx,a,a); qaRequiref(t, !(!errors.Is(err,ErrInvalid)), "self direct: %v",err) }()
 func() { err:=s.HideDirect(ctx,direct,a,true); qaRequire(t, !(err!=nil), err) }()
 var hidden bool
 func() { err:=db.QueryRowContext(ctx,`SELECT hidden FROM issue25_user_state WHERE conversation_id=$1 AND account_id=$2`,direct,a).Scan(&hidden); qaRequiref(t, !(err!=nil||!hidden), "hidden true: %v %v",hidden,err) }()
 if _,err=s.Send(ctx,direct,b,fixtureID(t),Payload{Text:"incoming"});err!=nil{t.Fatal(err)}
 func() { err:=db.QueryRowContext(ctx,`SELECT hidden FROM issue25_user_state WHERE conversation_id=$1 AND account_id=$2`,direct,a).Scan(&hidden); qaRequiref(t, !(err!=nil||hidden), "incoming should reopen: %v %v",hidden,err) }()
 func() { err:=s.HideDirect(ctx,direct,a,true); qaRequire(t, !(err!=nil), err) }()
 func() { err:=s.HideDirect(ctx,direct,a,false); qaRequire(t, !(err!=nil), err) }()
 func() { err:=db.QueryRowContext(ctx,`SELECT hidden FROM issue25_user_state WHERE conversation_id=$1 AND account_id=$2`,direct,a).Scan(&hidden); qaRequiref(t, !(err!=nil||hidden), "explicit reopen: %v %v",hidden,err) }()
}

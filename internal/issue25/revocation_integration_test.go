package issue25

import (
 "context"
 "errors"
 "testing"
)

func TestPGMS2511EpochChannelRevokedAfterRemoval(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,viewer,stranger:=fixtureID(t),fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"epoch revocation",[]string{viewer});qaRequire(t, !(err!=nil), err)
 var epoch string
 func() { err=db.QueryRowContext(ctx,`SELECT epoch::text FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,group,viewer).Scan(&epoch); qaRequire(t, !(err!=nil), err) }()
 channel:="chat:e:"+epoch
 func() { err=s.AuthorizeEpochChannel(ctx,channel,viewer); qaRequiref(t, !(err!=nil), "authorized member denied: %v",err) }()
 func() { err=s.AuthorizeEpochChannel(ctx,channel,stranger); qaRequiref(t, !(!errors.Is(err,ErrForbidden)), "stranger subscribed: %v",err) }()
 func() { err=s.Remove(ctx,group,owner,viewer); qaRequire(t, !(err!=nil), err) }()
 func() { err=s.AuthorizeEpochChannel(ctx,channel,viewer); qaRequiref(t, !(!errors.Is(err,ErrForbidden)), "retired epoch subscribed: %v",err) }()
 fresh,err:=s.Join(ctx,group,owner,viewer,"member");qaRequire(t, !(err!=nil), err)
 qaRequire(t, !(fresh==epoch), "membership epoch reused")
 func() { err=s.AuthorizeEpochChannel(ctx,channel,viewer); qaRequiref(t, !(!errors.Is(err,ErrForbidden)), "old channel regained rights: %v",err) }()
 func() { err=s.AuthorizeEpochChannel(ctx,"chat:e:"+fresh,viewer); qaRequiref(t, !(err!=nil), "fresh epoch denied: %v",err) }()
}

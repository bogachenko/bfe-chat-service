package issue25

import (
 "context"
 "errors"
 "testing"
)

func TestPGMS2511EpochChannelRevokedAfterRemoval(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,viewer,stranger:=fixtureID(t),fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"epoch revocation",[]string{viewer});qaReject(t, (err!=nil), err)
 var epoch string
 func() { err=db.QueryRowContext(ctx,`SELECT epoch::text FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,group,viewer).Scan(&epoch); qaReject(t, (err!=nil), err) }()
 channel:="chat:e:"+epoch
 func() { err=s.AuthorizeEpochChannel(ctx,channel,viewer); qaRejectf(t, (err!=nil), "authorized member denied: %v",err) }()
 func() { err=s.AuthorizeEpochChannel(ctx,channel,stranger); qaRejectf(t, (!errors.Is(err,ErrForbidden)), "stranger subscribed: %v",err) }()
 func() { err=s.Remove(ctx,group,owner,viewer); qaReject(t, (err!=nil), err) }()
 func() { err=s.AuthorizeEpochChannel(ctx,channel,viewer); qaRejectf(t, (!errors.Is(err,ErrForbidden)), "retired epoch subscribed: %v",err) }()
 fresh,err:=s.Join(ctx,group,owner,viewer,"member");qaReject(t, (err!=nil), err)
 qaReject(t, (fresh==epoch), "membership epoch reused")
 func() { err=s.AuthorizeEpochChannel(ctx,channel,viewer); qaRejectf(t, (!errors.Is(err,ErrForbidden)), "old channel regained rights: %v",err) }()
 func() { err=s.AuthorizeEpochChannel(ctx,"chat:e:"+fresh,viewer); qaRejectf(t, (err!=nil), "fresh epoch denied: %v",err) }()
}

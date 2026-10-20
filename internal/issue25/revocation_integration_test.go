package issue25

import (
 "context"
 "errors"
 "testing"
)

func TestPGMS2511EpochChannelRevokedAfterRemoval(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,viewer,stranger:=fixtureID(t),fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"epoch revocation",[]string{viewer});if err!=nil{t.Fatal(err)}
 var epoch string
 if err=db.QueryRowContext(ctx,`SELECT epoch::text FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,group,viewer).Scan(&epoch);err!=nil{t.Fatal(err)}
 channel:="chat:e:"+epoch
 if err=s.AuthorizeEpochChannel(ctx,channel,viewer);err!=nil{t.Fatalf("authorized member denied: %v",err)}
 if err=s.AuthorizeEpochChannel(ctx,channel,stranger);!errors.Is(err,ErrForbidden){t.Fatalf("stranger subscribed: %v",err)}
 if err=s.Remove(ctx,group,owner,viewer);err!=nil{t.Fatal(err)}
 if err=s.AuthorizeEpochChannel(ctx,channel,viewer);!errors.Is(err,ErrForbidden){t.Fatalf("retired epoch subscribed: %v",err)}
 fresh,err:=s.Join(ctx,group,owner,viewer,"member");if err!=nil{t.Fatal(err)}
 if fresh==epoch{t.Fatal("membership epoch reused")}
 if err=s.AuthorizeEpochChannel(ctx,channel,viewer);!errors.Is(err,ErrForbidden){t.Fatalf("old channel regained rights: %v",err)}
 if err=s.AuthorizeEpochChannel(ctx,"chat:e:"+fresh,viewer);err!=nil{t.Fatalf("fresh epoch denied: %v",err)}
}

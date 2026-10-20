package issue25

import (
 "context"
 "errors"
 "testing"
)

func TestPGMS2504RemoveFirstRejectsSendWithoutOrdinalGap(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 owner,member:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"remove-first",[]string{member});qaReject(t, (err!=nil), err)
 func() { err=s.Remove(ctx,group,owner,member); qaReject(t, (err!=nil), err) }()
 _,err=s.Send(ctx,group,member,fixtureID(t),Payload{Text:"denied"})
 qaRejectf(t, (!errors.Is(err,ErrForbidden)), "expected FORBIDDEN got %v",err)
 var next int64
 func() { err=db.QueryRowContext(ctx,`SELECT next_ordinal FROM issue25_conversations WHERE id=$1`,group).Scan(&next); qaReject(t, (err!=nil), err) }()
 qaRejectf(t, (next!=1), "counter advanced after forbidden SEND: %d",next)
 m,err:=s.Send(ctx,group,owner,fixtureID(t),Payload{Text:"owner-1"});qaReject(t, (err!=nil), err)
 qaRejectf(t, (m.Ordinal!=1), "owner ordinal=%d",m.Ordinal)
 func() { _,err=s.ReadHistory(ctx,group,member,0,50); qaRejectf(t, (!errors.Is(err,ErrForbidden)), "removed history=%v",err) }()
 var n int
 func() { err=db.QueryRowContext(ctx,`SELECT count(*) FROM issue25_messages WHERE conversation_id=$1`,group).Scan(&n); qaReject(t, (err!=nil), err) }()
 qaRejectf(t, (n!=1), "messages=%d",n)
}
func TestPGMS2506CanonicalReplayAndConflict(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 sender:=fixtureID(t)
 group,err:=s.CreateGroup(ctx,sender,"idempotency",nil);qaReject(t, (err!=nil), err)
 key:=fixtureID(t)
 first,err:=s.Send(ctx,group,sender,key,Payload{Text:"abc"});qaReject(t, (err!=nil), err)
 same,err:=s.Send(ctx,group,sender,key,Payload{Text:"abc"});qaReject(t, (err!=nil), err)
 qaRejectf(t, (!same.Replayed||first.ID!=same.ID||first.Ordinal!=same.Ordinal), "replay first=%+v same=%+v",first,same)
 if _,err=s.Send(ctx,group,sender,key,Payload{Text:"abd"});!errors.Is(err,ErrConflict){t.Fatalf("expected CONFLICT_409, got %v",err)}
 var rows int
 func() { err=db.QueryRowContext(ctx,`SELECT count(*) FROM issue25_messages WHERE conversation_id=$1`,group).Scan(&rows); qaReject(t, (err!=nil), err) }()
 qaRejectf(t, (rows!=1), "idempotent message rows=%d",rows)
 var next int64
 func() { err=db.QueryRowContext(ctx,`SELECT next_ordinal FROM issue25_conversations WHERE id=$1`,group).Scan(&next); qaReject(t, (err!=nil), err) }()
 qaRejectf(t, (next!=2), "unexpected next ordinal %d",next)
}

package issue25

import (
 "context"
 "database/sql"
 "errors"
 "strings"
)

func (s Store) CreateGroup(ctx context.Context, owner,name string,members []string)(string,error){
 if owner==""||len(strings.TrimSpace(name))==0||len(name)>256||len(members)+1>100{return "",ErrInvalid}
 id,err:=newUUID();if err!=nil{return "",err}
 epoch,err:=newUUID();if err!=nil{return "",err}
 tx,err:=s.DB.BeginTx(ctx,nil);if err!=nil{return "",err};defer tx.Rollback()
 _,err=tx.ExecContext(ctx,`INSERT INTO issue25_conversations(id,kind,name) VALUES($1,'group',$2)`,id,name);if err!=nil{return "",err}
 _,err=tx.ExecContext(ctx,`INSERT INTO issue25_memberships(epoch,conversation_id,account_id,role,join_boundary) VALUES($1,$2,$3,'owner',1)`,epoch,id,owner);if err!=nil{return "",err}
 seen:=map[string]bool{owner:true}
 for _,member:=range members{
  if member==""||seen[member]{return "",ErrInvalid};seen[member]=true
  e,er:=newUUID();if er!=nil{return "",er}
  if _,er=tx.ExecContext(ctx,`INSERT INTO issue25_memberships(epoch,conversation_id,account_id,role,join_boundary) VALUES($1,$2,$3,'member',1)`,e,id,member);er!=nil{return "",er}
 }
 if err=tx.Commit();err!=nil{return "",err};return id,nil
}
func (s Store) TransferOwner(ctx context.Context,conversation,owner,successor string)error{
 if owner==successor{return ErrInvalid}
 return s.tx(ctx,conversation,func(tx *sql.Tx)error{
  var role string
  if err:=tx.QueryRowContext(ctx,`SELECT role FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,conversation,owner).Scan(&role);err!=nil||role!="owner"{return ErrForbidden}
  if err:=tx.QueryRowContext(ctx,`SELECT role FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,conversation,successor).Scan(&role);err!=nil{return ErrForbidden}
  if _,err:=tx.ExecContext(ctx,`UPDATE issue25_memberships SET role='member' WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,conversation,owner);err!=nil{return err}
  _,err:=tx.ExecContext(ctx,`UPDATE issue25_memberships SET role='owner' WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,conversation,successor)
  return err
 })
}
func (s Store) ChangeAdmin(ctx context.Context,conversation,owner,target string,promote bool)error{
 return s.tx(ctx,conversation,func(tx *sql.Tx)error{
  var role string
  if err:=tx.QueryRowContext(ctx,`SELECT role FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,conversation,owner).Scan(&role);err!=nil||role!="owner"{return ErrForbidden}
  if err:=tx.QueryRowContext(ctx,`SELECT role FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,conversation,target).Scan(&role);err!=nil||role=="owner"{return ErrForbidden}
  next:="member";if promote{next="admin"}
  _,err:=tx.ExecContext(ctx,`UPDATE issue25_memberships SET role=$3 WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,conversation,target,next)
  return err
 })
}
func (s Store) HideDirect(ctx context.Context,conversation,viewer string,hidden bool)error{
 var kind string
 if err:=s.DB.QueryRowContext(ctx,`SELECT kind FROM issue25_conversations WHERE id=$1`,conversation).Scan(&kind);err!=nil{return err}
 if kind!="direct"{return ErrInvalid}
 var present bool
 err:=s.DB.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL)`,conversation,viewer).Scan(&present)
 if err!=nil{return err};if !present{return ErrForbidden}
 _,err=s.DB.ExecContext(ctx,`INSERT INTO issue25_user_state(conversation_id,account_id,hidden) VALUES($1,$2,$3)
 ON CONFLICT(conversation_id,account_id) DO UPDATE SET hidden=excluded.hidden`,conversation,viewer,hidden)
 return err
}
func (s Store) MarkRead(ctx context.Context,conversation,viewer string,ordinal int64)(int64,error){
 if ordinal<0{return 0,ErrInvalid}
 var result int64
 err:=s.tx(ctx,conversation,func(tx *sql.Tx)error{
  if _,_,e:=active(tx,ctx,conversation,viewer);e!=nil{return e}
  if ordinal>0{
   var found bool
   e:=tx.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM issue25_messages m
    JOIN issue25_memberships h ON h.conversation_id=m.conversation_id
    WHERE m.conversation_id=$1 AND m.ordinal=$2 AND h.account_id=$3
    AND m.ordinal>=h.join_boundary AND (h.leave_boundary IS NULL OR m.ordinal<h.leave_boundary))`,conversation,ordinal,viewer).Scan(&found)
   if e!=nil{return e};if !found{return ErrInvalid}
  }
  return tx.QueryRowContext(ctx,`INSERT INTO issue25_user_state(conversation_id,account_id,last_read_ordinal)
    VALUES($1,$2,$3) ON CONFLICT(conversation_id,account_id)
    DO UPDATE SET last_read_ordinal=greatest(issue25_user_state.last_read_ordinal,excluded.last_read_ordinal)
    RETURNING last_read_ordinal`,conversation,viewer,ordinal).Scan(&result)
 })
 return result,err
}
func (s Store) Unread(ctx context.Context,conversation,viewer string)(int64,error){
 var count int64
 err:=s.DB.QueryRowContext(ctx,`SELECT count(*) FROM issue25_messages m WHERE m.conversation_id=$1 AND m.sender_id<>$2
 AND m.ordinal>coalesce((SELECT last_read_ordinal FROM issue25_user_state WHERE conversation_id=$1 AND account_id=$2),0)
 AND EXISTS(SELECT 1 FROM issue25_memberships h WHERE h.conversation_id=m.conversation_id
 AND h.account_id=$2 AND m.ordinal>=h.join_boundary
 AND (h.leave_boundary IS NULL OR m.ordinal<h.leave_boundary))`,conversation,viewer).Scan(&count)
 return count,err
}
func (s Store) ClaimAndGet(ctx context.Context,id string)(Attempt,bool,error){
 claimed,err:=s.ClaimAttempt(ctx,id);if err!=nil||!claimed{return Attempt{},claimed,err}
 var a Attempt
 err=s.DB.QueryRowContext(ctx,`SELECT attempt_id::text,conversation_id::text,viewer_id::text,
 membership_epoch::text,message_id::text,message_ordinal,wake_token,opaque_channel
 FROM issue25_publish_attempts WHERE attempt_id=$1 AND state='CLAIMED'`,id).Scan(
 &a.ID,&a.Conversation,&a.Viewer,&a.Epoch,&a.MessageID,&a.Ordinal,&a.WakeToken,&a.Channel)
 return a,true,err
}
func (s Store) SetAttemptOutcome(ctx context.Context,id,state string)error{
 if state!="SUCCESS"&&state!="UNKNOWN"&&state!="FAILED"{return ErrInvalid}
 res,err:=s.DB.ExecContext(ctx,`UPDATE issue25_publish_attempts SET state=$2 WHERE attempt_id=$1 AND state='CLAIMED'`,id,state)
 if err!=nil{return err};n,err:=res.RowsAffected();if err!=nil{return err};if n!=1{return errors.New("ATTEMPT_NOT_CLAIMED")};return nil
}

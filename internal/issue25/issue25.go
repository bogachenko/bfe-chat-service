// Package issue25 implements the additive chat domain without changing legacy routes.
package issue25

import (
 "bytes"
 "context"
 "crypto/rand"
 "database/sql"
 "encoding/json"
 "errors"
 "fmt"
 "strings"
)

var (
 ErrForbidden=errors.New("FORBIDDEN")
 ErrConflict=errors.New("CONFLICT")
 ErrInvalid=errors.New("VALIDATION_ERROR")
 ErrAlreadyClaimed=errors.New("ALREADY_CLAIMED")
)
type Store struct{ DB *sql.DB }
type Message struct { ID string; Ordinal int64; Replayed bool }
type Payload struct { Text string `json:"text"`; DocumentID *string `json:"documentId"`; DocumentTitle *string `json:"documentTitle"` }
type Attempt struct { ID,Conversation,Viewer,Epoch,MessageID,WakeToken,Channel string; Ordinal int64 }
func canonical(p Payload)([]byte,error){
 if len(p.Text)>16384 || (p.DocumentID==nil)!=(p.DocumentTitle==nil) {return nil,ErrInvalid}
 if p.DocumentID!=nil && (strings.TrimSpace(*p.DocumentID)==""||strings.TrimSpace(*p.DocumentTitle)==""){return nil,ErrInvalid}
 if strings.TrimSpace(p.Text)==""&&p.DocumentID==nil{return nil,ErrInvalid}
 return json.Marshal(p)
}
func CanonicalPayload(p Payload)([]byte,error){return canonical(p)}
func newUUID()(string,error){var b [16]byte;if _,err:=rand.Read(b[:]);err!=nil{return "",err};b[6]=(b[6]&15)|64;b[8]=(b[8]&63)|128;return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",b[:4],b[4:6],b[6:8],b[8:10],b[10:]),nil}
func (s Store) tx(ctx context.Context,conversation string,fn func(*sql.Tx)error)error{
 if s.DB==nil{return ErrInvalid}
 tx,err:=s.DB.BeginTx(ctx,nil);if err!=nil{return err};defer tx.Rollback()
 // Lock by conversation row, not a hash collision-prone advisory key.
 var id string
 if err=tx.QueryRowContext(ctx,`SELECT id::text FROM issue25_conversations WHERE id=$1::uuid FOR UPDATE`,conversation).Scan(&id);err!=nil{return err}
 if err=fn(tx);err!=nil{return err}
 return tx.Commit()
}
func (s Store) OpenDirect(ctx context.Context,a,b string)(string,error){
 if a==b||a==""||b==""{return "",ErrInvalid}
 if a>b{a,b=b,a}
 id,err:=newUUID();if err!=nil{return "",err}
 err=s.DB.QueryRowContext(ctx,`INSERT INTO issue25_conversations(id,kind,direct_a,direct_b) VALUES($1,'direct',$2,$3)
 ON CONFLICT (direct_a,direct_b) WHERE kind='direct' DO UPDATE SET direct_a=excluded.direct_a
 RETURNING id::text`,id,a,b).Scan(&id)
 return id,err
}
func active(tx *sql.Tx,ctx context.Context,conversation,user string)(string,int64,error){
 var epoch string;var join int64
 err:=tx.QueryRowContext(ctx,`SELECT epoch::text,join_boundary FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,conversation,user).Scan(&epoch,&join)
 if errors.Is(err,sql.ErrNoRows){return "",0,ErrForbidden};return epoch,join,err
}
func (s Store) Send(ctx context.Context,conversation,sender,key string,p Payload)(Message,error){
 raw,err:=canonical(p);if err!=nil{return Message{},err}
 var result Message
 err=s.tx(ctx,conversation,func(tx *sql.Tx)error{
  if _,_,e:=active(tx,ctx,conversation,sender);e!=nil{return e}
  var previous []byte
  e:=tx.QueryRowContext(ctx,`SELECT id::text,ordinal,canonical_payload FROM issue25_messages WHERE conversation_id=$1 AND sender_id=$2 AND idempotency_key=$3`,conversation,sender,key).Scan(&result.ID,&result.Ordinal,&previous)
  if e==nil {if !bytes.Equal(previous,raw){return ErrConflict};result.Replayed=true;return nil}
  if !errors.Is(e,sql.ErrNoRows){return e}
  var ordinal int64
  if e=tx.QueryRowContext(ctx,`UPDATE issue25_conversations SET next_ordinal=next_ordinal+1 WHERE id=$1 RETURNING next_ordinal-1`,conversation).Scan(&ordinal);e!=nil{return e}
  id,e:=newUUID();if e!=nil{return e}
  _,e=tx.ExecContext(ctx,`INSERT INTO issue25_messages(id,conversation_id,ordinal,sender_id,idempotency_key,body,document_id,document_title,canonical_payload)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`,id,conversation,ordinal,sender,key,p.Text,p.DocumentID,p.DocumentTitle,raw)
  if e==nil{result=Message{ID:id,Ordinal:ordinal}};return e
 })
 return result,err
}
func (s Store) Join(ctx context.Context,conversation,actor,target,role string)(string,error){
 if role!="member"&&role!="admin"{return "",ErrInvalid}
 epoch,err:=newUUID();if err!=nil{return "",err}
 err=s.tx(ctx,conversation,func(tx *sql.Tx)error{
  var actorRole string
  if e:=tx.QueryRowContext(ctx,`SELECT role FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,conversation,actor).Scan(&actorRole);e!=nil{return ErrForbidden}
  if actorRole!="owner"&&actorRole!="admin"{return ErrForbidden};if role=="admin"&&actorRole!="owner"{return ErrForbidden}
  var boundary int64
  if e:=tx.QueryRowContext(ctx,`SELECT next_ordinal FROM issue25_conversations WHERE id=$1`,conversation).Scan(&boundary);e!=nil{return e}
  var count int
  if e:=tx.QueryRowContext(ctx,`SELECT count(*) FROM issue25_memberships WHERE conversation_id=$1 AND leave_boundary IS NULL`,conversation).Scan(&count);e!=nil{return e}
  if count>=100{return ErrInvalid}
  _,e:=tx.ExecContext(ctx,`INSERT INTO issue25_memberships(epoch,conversation_id,account_id,role,join_boundary) VALUES($1,$2,$3,$4,$5)`,epoch,conversation,target,role,boundary)
  return e
 })
 return epoch,err
}
func (s Store) Remove(ctx context.Context,conversation,actor,target string)error{
 return s.tx(ctx,conversation,func(tx *sql.Tx)error{
  var actorRole,targetRole string
  if e:=tx.QueryRowContext(ctx,`SELECT role FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,conversation,actor).Scan(&actorRole);e!=nil{return ErrForbidden}
  if e:=tx.QueryRowContext(ctx,`SELECT role FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,conversation,target).Scan(&targetRole);e!=nil{return ErrForbidden}
  if targetRole=="owner"|| (actor!=target && actorRole!="owner"&&!(actorRole=="admin"&&targetRole=="member")){return ErrForbidden}
  var n int64;if e:=tx.QueryRowContext(ctx,`SELECT next_ordinal FROM issue25_conversations WHERE id=$1`,conversation).Scan(&n);e!=nil{return e}
  _,e:=tx.ExecContext(ctx,`UPDATE issue25_memberships SET leave_boundary=$3 WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL`,conversation,target,n)
  return e
 })
}
func (s Store) ReadHistory(ctx context.Context,conversation,viewer string,after int64,limit int)([]Message,error){
 if limit<1||limit>100{return nil,ErrInvalid}
 // Historical membership enables interval-only reads; current session must be validated by caller.
 rows,err:=s.DB.QueryContext(ctx,`SELECT m.id::text,m.ordinal FROM issue25_messages m
 WHERE m.conversation_id=$1 AND m.ordinal>$3 AND EXISTS (
 SELECT 1 FROM issue25_memberships h WHERE h.conversation_id=m.conversation_id AND h.account_id=$2
 AND m.ordinal>=h.join_boundary AND (h.leave_boundary IS NULL OR m.ordinal<h.leave_boundary))
 ORDER BY m.ordinal LIMIT $4`,conversation,viewer,after,limit)
 if err!=nil{return nil,err};defer rows.Close();var result []Message
 for rows.Next(){var m Message;if err=rows.Scan(&m.ID,&m.Ordinal);err!=nil{return nil,err};result=append(result,m)}
 return result,rows.Err()
}
func (s Store) RegisterAttempt(ctx context.Context,a Attempt)error{
 return s.tx(ctx,a.Conversation,func(tx *sql.Tx)error{
  var id string
  e:=tx.QueryRowContext(ctx,`SELECT m.id::text FROM issue25_messages m JOIN issue25_memberships h ON h.conversation_id=m.conversation_id
 WHERE m.id=$1 AND m.conversation_id=$2 AND m.ordinal=$3 AND h.account_id=$4 AND h.epoch=$5 AND h.leave_boundary IS NULL
 AND m.ordinal>=h.join_boundary`,a.MessageID,a.Conversation,a.Ordinal,a.Viewer,a.Epoch).Scan(&id)
  if errors.Is(e,sql.ErrNoRows){return ErrForbidden};if e!=nil{return e}
  _,e=tx.ExecContext(ctx,`INSERT INTO issue25_publish_attempts(attempt_id,conversation_id,viewer_id,membership_epoch,message_id,message_ordinal,wake_token,opaque_channel)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,a.ID,a.Conversation,a.Viewer,a.Epoch,a.MessageID,a.Ordinal,a.WakeToken,a.Channel)
  return e
 })
}
func (s Store) ClaimAttempt(ctx context.Context,id string)(bool,error){
 res,err:=s.DB.ExecContext(ctx,`UPDATE issue25_publish_attempts SET state='CLAIMED', claimed_at=now() WHERE attempt_id=$1 AND state='REGISTERED'`,id)
 if err!=nil{return false,err};n,err:=res.RowsAffected();return n==1,err
}

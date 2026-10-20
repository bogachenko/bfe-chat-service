package issue25

import (
 "context"
 "testing"
)

func TestPGMS2514LegacyRowsRemainUntouchedByAdditiveV2(t *testing.T){
 db:=integrationDB(t);ctx:=context.Background();s:=Store{DB:db}
 legacyOwner,legacyPeer:=fixtureID(t),fixtureID(t)
 _,err:=db.ExecContext(ctx,`INSERT INTO chat_assignments(owner_account_id,recipient_account_id) VALUES($1,$2)`,legacyOwner,legacyPeer)
 qaRequire(t, !(err!=nil), err)
 _,err=db.ExecContext(ctx,`INSERT INTO chat_conversations(owner_account_id) VALUES($1)`,legacyOwner)
 qaRequire(t, !(err!=nil), err)
 key:=fixtureID(t)
 _,err=db.ExecContext(ctx,`INSERT INTO chat_messages(owner_account_id,sender_account_id,recipient_account_id,idempotency_key,body,document_id,document_title)
 VALUES($1,$1,$2,$3,'immutable legacy fixture','legacy-doc','Legacy title')`,legacyOwner,legacyPeer,key)
 qaRequire(t, !(err!=nil), err)
 var prior int
 func() { err=db.QueryRowContext(ctx,`SELECT count(*) FROM chat_messages WHERE owner_account_id=$1`,legacyOwner).Scan(&prior); qaRequire(t, !(err!=nil), err) }()
 qaRequiref(t, !(prior!=1), "unexpected legacy baseline=%d",prior)
 user:=fixtureID(t)
 group,err:=s.CreateGroup(ctx,user,"new Issue25 independent conversation",nil);qaRequire(t, !(err!=nil), err)
 if _,err=s.Send(ctx,group,user,fixtureID(t),Payload{Text:"Issue25 independent message"});err!=nil{t.Fatal(err)}
 var count int;var body,title string
 err=db.QueryRowContext(ctx,`SELECT count(*),max(body),max(document_title) FROM chat_messages WHERE owner_account_id=$1`,legacyOwner).Scan(&count,&body,&title)
 qaRequire(t, !(err!=nil), err)
 qaRequiref(t, !(count!=1||body!="immutable legacy fixture"||title!="Legacy title"), "legacy data changed rows=%d body=%q title=%q",count,body,title)
 var assignment string
 func() { err=db.QueryRowContext(ctx,`SELECT recipient_account_id::text FROM chat_assignments WHERE owner_account_id=$1`,legacyOwner).Scan(&assignment); qaRequire(t, !(err!=nil), err) }()
 qaRequiref(t, !(assignment!=legacyPeer), "legacy assignment altered %q",assignment)
}

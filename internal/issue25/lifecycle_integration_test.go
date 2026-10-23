//go:build integration

package issue25

import (
 "context"
 "database/sql"
 "errors"
 "net/http"
 "net/http/httptest"
 "strings"
 "sync"
 "testing"
)

func TestIssue32RenameGroupRolesValidationAndPersistence(t *testing.T){
 db:=integrationDB(t)
 ctx:=context.Background()
 s:=Store{DB:db}
 owner,admin,member,outsider:=fixtureID(t),fixtureID(t),fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"original",[]string{admin,member})
 if err!=nil{t.Fatal(err)}
 if err=s.ChangeAdmin(ctx,group,owner,admin,true);err!=nil{t.Fatal(err)}
 read:=func()string{
  t.Helper()
  var n string
  if err:=db.QueryRowContext(ctx,"SELECT name FROM issue25_conversations WHERE id=$1",group).Scan(&n);err!=nil{t.Fatal(err)}
  return n
 }
 for _,tc:=range []struct{name,actor,title string;want error;after string}{
  {"member denied",member,"forbidden",ErrForbidden,"original"},
  {"outsider denied",outsider,"forbidden",ErrForbidden,"original"},
  {"owner rename",owner,"owner title",nil,"owner title"},
  {"admin rename",admin,"admin title",nil,"admin title"},
  {"empty",owner,"",ErrInvalid,"admin title"},
  {"whitespace",owner," \t ",ErrInvalid,"admin title"},
  {"257 byte",owner,strings.Repeat("x",257),ErrInvalid,"admin title"},
  {"256 byte",owner,strings.Repeat("x",256),nil,strings.Repeat("x",256)},
 }{
  t.Run(tc.name,func(t *testing.T){
   e:=s.RenameGroup(ctx,group,tc.actor,tc.title)
   if !errors.Is(e,tc.want){t.Fatalf("RenameGroup=%v want %v",e,tc.want)}
   if got:=read();got!=tc.after{t.Fatalf("title=%q want %q",got,tc.after)}
  })
 }
 direct,err:=s.OpenDirect(ctx,owner,member)
 if err!=nil{t.Fatal(err)}
 if err=s.RenameGroup(ctx,direct,owner,"forbidden");!errors.Is(err,ErrInvalid){t.Fatalf("direct renamed: %v",err)}
 if err=s.Remove(ctx,group,owner,admin);err!=nil{t.Fatal(err)}
 if err=s.RenameGroup(ctx,group,admin,"revoked");!errors.Is(err,ErrForbidden){t.Fatalf("removed admin renamed: %v",err)}
 if read()!=strings.Repeat("x",256){t.Fatal("denied update changed persisted title")}
}

func TestIssue32RenameGroupHTTP(t *testing.T){
 db:=integrationDB(t)
 s:=Store{DB:db}
 owner,member:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(context.Background(),owner,"start",[]string{member})
 if err!=nil{t.Fatal(err)}
 call:=func(actor,method,body,auth string)*httptest.ResponseRecorder{
  t.Helper()
  h:=HTTPHandler{Store:s,Auth:fakeCaller{id:actor},PublicOrigin:"https://chat.example.test"}
  r:=httptest.NewRequest(method,"/conversations/"+group+"/name",strings.NewReader(body))
  if auth!=""{r.Header.Set("Authorization",auth)}
  r.Header.Set("DPoP","test-proof")
  w:=httptest.NewRecorder();h.ServeHTTP(w,r);return w
 }
 cases:=[]struct{name,actor,method,body,auth string;want int}{
  {"owner rename",owner,http.MethodPut,`{"name":"new title"}`,"DPoP test-token",http.StatusOK},
  {"member forbidden",member,http.MethodPut,`{"name":"denied"}`,"DPoP test-token",http.StatusForbidden},
  {"Bearer forbidden",owner,http.MethodPut,`{"name":"denied"}`,"Bearer test-token",http.StatusUnauthorized},
  {"invalid JSON",owner,http.MethodPut,"{","DPoP test-token",http.StatusBadRequest},
  {"unknown JSON field",owner,http.MethodPut,`{"name":"valid","other":1}`,"DPoP test-token",http.StatusBadRequest},
  {"wrong method",owner,http.MethodPost,`{"name":"no"}`,"DPoP test-token",http.StatusNotFound},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){
  w:=call(tc.actor,tc.method,tc.body,tc.auth)
  if w.Code!=tc.want{t.Fatalf("status=%d want=%d body=%s",w.Code,tc.want,w.Body.String())}
  if w.Header().Get("Cache-Control")!="no-store"{t.Fatal("missing no-store")}
 })}
 var name string
 if err:=db.QueryRowContext(context.Background(),"SELECT name FROM issue25_conversations WHERE id=$1",group).Scan(&name);err!=nil{t.Fatal(err)}
 if name!="new title"{t.Fatalf("failed requests mutated title %q",name)}
}

func TestIssue32RenameConcurrentWithRevocation(t *testing.T){
 db:=integrationDB(t)
 s:=Store{DB:db};ctx:=context.Background()
 owner,admin:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"before",[]string{admin})
 if err!=nil{t.Fatal(err)}
 if err=s.ChangeAdmin(ctx,group,owner,admin,true);err!=nil{t.Fatal(err)}
 var wg sync.WaitGroup
 errs:=make(chan error,2)
 wg.Add(2)
 go func(){defer wg.Done();errs<-s.RenameGroup(ctx,group,admin,"concurrent")}()
 go func(){defer wg.Done();errs<-s.Remove(ctx,group,owner,admin)}()
 wg.Wait();close(errs)
 for e:=range errs{if e!=nil&&!errors.Is(e,ErrForbidden){t.Fatalf("unexpected concurrent error: %v",e)}}
 if err=s.RenameGroup(ctx,group,admin,"after revoke");!errors.Is(err,ErrForbidden){t.Fatalf("revocation bypass: %v",err)}
 var role sql.NullString
 err=db.QueryRowContext(ctx,"SELECT role FROM issue25_memberships WHERE conversation_id=$1 AND account_id=$2 AND leave_boundary IS NULL",group,admin).Scan(&role)
 if !errors.Is(err,sql.ErrNoRows){t.Fatalf("membership still active: role=%v err=%v",role,err)}
 var count int
 if err=db.QueryRowContext(ctx,"SELECT count(*) FROM issue25_memberships WHERE conversation_id=$1 AND role='owner' AND leave_boundary IS NULL",group).Scan(&count);err!=nil||count!=1{t.Fatalf("owner invariant=%d %v",count,err)}
}

func TestIssue32RenameGroupFailurePaths(t *testing.T){
 db:=integrationDB(t)
 ctx:=context.Background()
 s:=Store{DB:db}
 owner,member:=fixtureID(t),fixtureID(t)
 group,err:=s.CreateGroup(ctx,owner,"stable",[]string{member})
 if err!=nil{t.Fatal(err)}
 if err:=s.RenameGroup(ctx,fixtureID(t),owner,"new");err==nil{t.Fatal("nonexistent conversation accepted")}
 stopped,cancel:=context.WithCancel(ctx)
 cancel()
 if err:=s.RenameGroup(stopped,group,owner,"new");err==nil{t.Fatal("cancelled transaction accepted")}
 if err:=s.ChangeAdmin(ctx,group,owner,member,true);err!=nil{t.Fatal(err)}
 if err:=s.ChangeAdmin(ctx,group,owner,member,false);err!=nil{t.Fatal(err)}
 if err:=s.RenameGroup(ctx,group,member,"unauthorized");!errors.Is(err,ErrForbidden){t.Fatalf("demoted admin accepted: %v",err)}
 var name string
 if err:=db.QueryRowContext(ctx,"SELECT name FROM issue25_conversations WHERE id=$1",group).Scan(&name);err!=nil{t.Fatal(err)}
 if name!="stable"{t.Fatalf("rejected rename changed name=%q",name)}
}

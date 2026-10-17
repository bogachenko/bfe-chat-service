package realtime_test

import (
 "context"
 "errors"
 "reflect"
 "sync"
 "testing"

 "github.com/bogachenko/bfe-chat-service/internal/chat"
 "github.com/bogachenko/bfe-chat-service/internal/realtime"
 "github.com/centrifugal/centrifuge"
)

const (
 owner chat.AccountID = "11111111-1111-1111-1111-111111111111"
 first chat.AccountID = "22222222-2222-2222-2222-222222222222"
 second chat.AccountID = "33333333-3333-3333-3333-333333333333"
)
var payload = []byte(`{"id":"m1"}`)

type assignmentStore struct {
 mu sync.Mutex
 recipient string
 err error
 calls int
}
func (s *assignmentStore) WithAssignmentLock(_ context.Context, _ string, fn func(string) error) error {
 s.mu.Lock()
 defer s.mu.Unlock()
 s.calls++
 if s.err != nil { return s.err }
 return fn(s.recipient)
}
type publication struct {
 channel string
 data string
}
type publisher struct {
 mu sync.Mutex
 sent []publication
 failAt int
 onPublish func(string)
}
func (p *publisher) Publish(channel string, data []byte, _ ...centrifuge.PublishOption) (centrifuge.PublishResult,error) {
 p.mu.Lock()
 defer p.mu.Unlock()
 if p.onPublish != nil { p.onPublish(channel) }
 if p.failAt == len(p.sent)+1 { return centrifuge.PublishResult{}, errors.New("broker down") }
 p.sent=append(p.sent,publication{channel,string(data)})
 return centrifuge.PublishResult{},nil
}
func (p *publisher) snapshot() []publication {
 p.mu.Lock()
 defer p.mu.Unlock()
 return append([]publication(nil),p.sent...)
}
func service(s *assignmentStore,p *publisher) realtime.Service {
 return realtime.Service{Assignments:s,Publisher:p}
}
func TestChannel(t *testing.T) {
 t.Parallel()
 got:=realtime.Channel(owner,first)
 want:="chat:"+string(owner)+":"+string(first)
 if got!=want {t.Fatalf("Channel=%q want %q",got,want)}
 if got==realtime.Channel(owner,second) {t.Fatal("different viewers share channel")}
 if got==realtime.Channel(first,first) {t.Fatal("different owners share channel")}
}
func TestBindRejectsMissingDependencies(t *testing.T) {
 t.Parallel()
 tests:=[]struct{name string;svc realtime.Service}{
  {"all missing",realtime.Service{}},
  {"only publisher",realtime.Service{Publisher:&publisher{}}},
  {"only assignments",realtime.Service{Assignments:&assignmentStore{}}},
 }
 for _,tt:=range tests {
  t.Run(tt.name,func(t *testing.T){
   t.Parallel()
   if err:=tt.svc.Bind(nil,"wss://example.org/chat");!errors.Is(err,realtime.ErrDeliveryFailed){t.Fatalf("Bind error=%v",err)}
  })
 }
}
func TestDeliverCommittedMessage(t *testing.T) {
 t.Parallel()
 tests:=[]struct{
  name string
  current string
  committed chat.AccountID
  want []publication
 }{
  {"same recipient",string(first),first,[]publication{{realtime.Channel(owner,owner),string(payload)},{realtime.Channel(owner,first),string(payload)}}},
  {"reassigned before delivery",string(second),first,[]publication{{realtime.Channel(owner,owner),string(payload)}}},
  {"owner assigned to self",string(owner),owner,[]publication{{realtime.Channel(owner,owner),string(payload)}}},
 }
 for _,tt:=range tests {
  t.Run(tt.name,func(t *testing.T){
   t.Parallel()
   s,p:=&assignmentStore{recipient:tt.current},&publisher{}
   err:=service(s,p).DeliverCommittedMessage(context.Background(),owner,tt.committed,payload)
   if err!=nil {t.Fatalf("delivery error: %v",err)}
   if !reflect.DeepEqual(p.snapshot(),tt.want){t.Fatalf("publications=%v want=%v",p.snapshot(),tt.want)}
   if s.calls!=1 {t.Fatalf("lock count=%d want 1",s.calls)}
  })
 }
}
func TestDeliverCommittedMessageSkipsInvalidInputs(t *testing.T) {
 t.Parallel()
 tests:=[]struct{name string;owner,recipient chat.AccountID;data []byte;current string}{
  {"invalid owner","invalid",first,payload,string(first)},
  {"invalid historical recipient",owner,"invalid",payload,string(first)},
  {"invalid JSON",owner,first,[]byte("not JSON"),string(first)},
  {"empty payload",owner,first,nil,string(first)},
  {"missing assignment",owner,first,payload,""},
  {"malformed current assignment",owner,first,payload,"invalid"},
 }
 for _,tt:=range tests {
  t.Run(tt.name,func(t *testing.T){
   t.Parallel()
   s,p:=&assignmentStore{recipient:tt.current},&publisher{}
   err:=service(s,p).DeliverCommittedMessage(context.Background(),tt.owner,tt.recipient,tt.data)
   if !errors.Is(err,realtime.ErrDeliverySkipped){t.Fatalf("error=%v want skipped",err)}
   if len(p.snapshot())!=0{t.Fatalf("unexpected publication: %v",p.snapshot())}
  })
 }
}
func TestDeliverCommittedMessageMissingDependencies(t *testing.T) {
 t.Parallel()
 tests:=[]struct{name string;svc realtime.Service}{
  {"no assignments",realtime.Service{Publisher:&publisher{}}},
  {"no publisher",realtime.Service{Assignments:&assignmentStore{recipient:string(first)}}},
 }
 for _,tt:=range tests {
  t.Run(tt.name,func(t *testing.T){
   t.Parallel()
   if err:=tt.svc.DeliverCommittedMessage(context.Background(),owner,first,payload);!errors.Is(err,realtime.ErrDeliverySkipped){t.Fatalf("error=%v",err)}
  })
 }
}
func TestDeliverCommittedMessageFailurePaths(t *testing.T) {
 t.Parallel()
 tests:=[]struct{name string;storeError error;failAt int;wantPublications int}{
  {"assignment storage failure",errors.New("db unavailable"),0,0},
  {"owner publication failure",nil,1,0},
  {"recipient publication failure",nil,2,1},
 }
 for _,tt:=range tests {
  t.Run(tt.name,func(t *testing.T){
   t.Parallel()
   s,p:=&assignmentStore{recipient:string(first),err:tt.storeError},&publisher{failAt:tt.failAt}
   err:=service(s,p).DeliverCommittedMessage(context.Background(),owner,first,payload)
   if !errors.Is(err,realtime.ErrDeliveryFailed){t.Fatalf("error=%v want delivery failed",err)}
   if len(p.snapshot())!=tt.wantPublications{t.Fatalf("publications=%v want count %d",p.snapshot(),tt.wantPublications)}
  })
 }
}
func TestDeliverySerializesReassignment(t *testing.T) {
 s:=&assignmentStore{recipient:string(first)}
 entered:=make(chan struct{})
 var signalOnce sync.Once
 release:=make(chan struct{})
 p:=&publisher{onPublish:func(channel string){
  if channel==realtime.Channel(owner,owner){signalOnce.Do(func(){close(entered);<-release})}
 }}
 delivered:=make(chan error,1)
 go func(){delivered<-service(s,p).DeliverCommittedMessage(context.Background(),owner,first,payload)}()
 <-entered
 reassigned:=make(chan struct{})
 go func(){
  s.mu.Lock()
  s.recipient=string(second)
  s.mu.Unlock()
  close(reassigned)
 }()
 select {
 case <-reassigned:t.Fatal("reassignment completed before synchronous publication released its assignment lock")
 default:
 }
 close(release)
 if err:=<-delivered;err!=nil{t.Fatalf("delivery: %v",err)}
 <-reassigned
 want:=[]publication{{realtime.Channel(owner,owner),string(payload)},{realtime.Channel(owner,first),string(payload)}}
 if !reflect.DeepEqual(p.snapshot(),want){t.Fatalf("publication=%v want=%v",p.snapshot(),want)}
 before:=len(p.snapshot())
 if err:=service(s,p).DeliverCommittedMessage(context.Background(),owner,first,payload);err!=nil{t.Fatal(err)}
 sent:=p.snapshot()
 if len(sent)!=before+1||sent[len(sent)-1].channel!=realtime.Channel(owner,owner){t.Fatalf("stale recipient received publication: %v",sent)}
}
func TestDeliverCommittedMessageConcurrentIndependentCalls(t *testing.T) {
 s,p:=&assignmentStore{recipient:string(first)},&publisher{}
 const n=20
 var wg sync.WaitGroup
 errs:=make(chan error,n)
 for range n {
  wg.Add(1)
  go func(){defer wg.Done();errs<-service(s,p).DeliverCommittedMessage(context.Background(),owner,first,payload)}()
 }
 wg.Wait()
 close(errs)
 for err:=range errs {if err!=nil{t.Fatalf("delivery: %v",err)}}
 if got:=len(p.snapshot());got!=2*n{t.Fatalf("publications=%d want %d",got,2*n)}
 if s.calls!=n {t.Fatalf("locked deliveries=%d want %d",s.calls,n)}
}

package main

import "testing"

func TestIsCriticalPackage(t *testing.T) {
 t.Parallel()
 cases:=[]struct{name,path string;want bool}{
  {"authentication","github.com/bogachenko/bfe-chat-service/internal/authentication/authentication.go:1.2,2.5",true},
  {"authorization","github.com/bogachenko/bfe-chat-service/internal/authorization/authorization.go:1.2,2.5",true},
  {"realtime","github.com/bogachenko/bfe-chat-service/internal/realtime/realtime.go:1.2,2.5",true},
  {"unrelated package","github.com/bogachenko/bfe-chat-service/internal/documentcontext/context.go:1.2,2.5",false},
  {"legacy path is not current package","github.com/bogachenko/bfe-chat-service/internal/auth/foo.go:1.2,2.5",false},
 }
 for _,tc:=range cases {
  t.Run(tc.name,func(t *testing.T){t.Parallel();if got:=isCriticalPackage(tc.path);got!=tc.want {t.Fatalf("critical=%t want=%t for %q",got,tc.want,tc.path)}})
 }
}
func TestCoverageCounts(t *testing.T) {
 t.Parallel()
 var counts coverageCounts
 counts.addRow("github.com/bogachenko/bfe-chat-service/internal/realtime/realtime.go:1.2,2.5 5 1")
 counts.addRow("github.com/bogachenko/bfe-chat-service/internal/realtime/realtime.go:3.2,4.5 3 0")
 counts.addRow("github.com/bogachenko/bfe-chat-service/internal/conversations/conversations.go:1.2,2.5 2 1")
 if counts.allTotal!=10||counts.allCovered!=7||counts.criticalTotal!=8||counts.criticalCovered!=5||!counts.criticalPresent {t.Fatalf("counts=%+v",counts)}
}

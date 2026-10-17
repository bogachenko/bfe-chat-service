package authentication_test

import (
 "bytes"
 "net/http"
 "testing"
 "time"
 "context"
 "net/http/httptest"
 "strings"

 "github.com/centrifugal/centrifuge"
)

// This tests real Centrifuge WebSocket frames, the production OnConnecting
// authentication handler, fresh DPoP proofs, and a revocable UDS fixture.
// It is NOT evidence of connectivity to a deployed BFE Drive instance.
func TestFC02RevokedDriveSessionRejectsLiveWebSocketReconnect(t *testing.T) {
 f:=newFixture(t)
 node,err:=centrifuge.New(centrifuge.Config{})
 if err!=nil{t.Fatal(err)}
 f.validator.BindOnConnecting(node,"wss://chat.example.test/connection")
 if err:=node.Run();err!=nil{t.Fatal(err)}
 t.Cleanup(func(){ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();_ =node.Shutdown(ctx)})
 server:=httptest.NewServer(centrifuge.NewWebsocketHandler(node,centrifuge.WebsocketConfig{}))
 t.Cleanup(server.Close)
 address:=strings.TrimPrefix(server.URL,"http://")
 opcode,reply:=connectWSProof(t,address,f.token,f.valid(t,"first-live-connect"),0)
 assertWebSocketConnect(t,opcode,reply,"valid session")
 if hits:=f.hits.Load();hits!=1{t.Fatalf("BFE validation requests after initial connect=%d, want 1",hits)}
 f.status.Store(http.StatusUnauthorized)
 opcode,reply=connectWSProof(t,address,f.token,f.valid(t,"fresh-after-revocation"),0)
 assertWebSocketReject(t,opcode,reply,"revoked session with fresh DPoP")
 if hits:=f.hits.Load();hits!=2{t.Fatalf("BFE validation requests after rejected reconnect=%d, want 2",hits)}
 f.status.Store(http.StatusServiceUnavailable)
 opcode,reply=connectWSProof(t,address,f.token,f.valid(t,"fresh-uds-error"),0)
 assertWebSocketReject(t,opcode,reply,"unavailable drive")
 if hits:=f.hits.Load();hits!=3{t.Fatalf("BFE validation requests after unavailable=%d, want 3",hits)}
 f.status.Store(http.StatusOK)
 opcode,reply=connectWSProof(t,address,f.token,f.valid(t,"new-after-service-recovery"),0)
 if opcode!=1||!bytes.Contains(reply,[]byte(`"connect"`)){t.Fatalf("service recovery failed: opcode=%d reply=%s",opcode,reply)}
 if hits:=f.hits.Load();hits!=4{t.Fatalf("BFE validation requests after recovery=%d, want 4",hits)}
}

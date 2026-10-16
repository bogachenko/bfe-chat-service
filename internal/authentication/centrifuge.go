package authentication

import (
	"context"
	"encoding/json"

	"github.com/centrifugal/centrifuge"
)

// BindOnConnecting registers the FC-02 handler on a Centrifuge node.
// The first connect frame carries the existing opaque BFE access token in
// ConnectEvent.Token and a fresh DPoP proof in ConnectEvent.Data.
// Neither value belongs in the WebSocket URL or application logs.
// Subscription authorization is deliberately NOT granted here (DP-M04/05).
func (v *Validator) BindOnConnecting(node *centrifuge.Node, publicWebSocketURL string) {
	node.OnConnecting(func(ctx context.Context, event centrifuge.ConnectEvent) (centrifuge.ConnectReply, error) {
		var payload struct {
			DPoP string `json:"dpop"`
		}
		if len(event.Data) == 0 || len(event.Data) > 16*1024 ||
			json.Unmarshal(event.Data, &payload) != nil || payload.DPoP == "" {
			return centrifuge.ConnectReply{}, centrifuge.DisconnectInvalidToken
		}

		caller, err := v.ValidateOnConnecting(ctx, event.Token, payload.DPoP, publicWebSocketURL)
		if err != nil {
			// Never send UDS or proof internals to unauthenticated clients.
			return centrifuge.ConnectReply{}, centrifuge.DisconnectInvalidToken
		}

		return centrifuge.ConnectReply{
			Credentials: &centrifuge.Credentials{UserID: caller.AccountID},
		}, nil
	})
}

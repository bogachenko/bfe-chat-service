// Package realtime implements DP-M05's transient Centrifuge delivery boundary.
// PostgreSQL remains the only durable source of chat history.
package realtime

import (
	"context"
	"errors"
	"strings"

	"github.com/centrifugal/centrifuge"
	"github.com/bogachenko/bfe-chat-service/internal/authentication"
	"github.com/bogachenko/bfe-chat-service/internal/authorization"
	"github.com/bogachenko/bfe-chat-service/internal/chat"
)

var (
	ErrDeliverySkipped = errors.New("DELIVERY_SKIPPED")
	ErrDeliveryFailed  = errors.New("DELIVERY_FAILED")
)

const prefix = "chat:"

// Channel addresses one viewer of one owner's conversation. Separate
// viewer channels prevent formerly assigned recipients from seeing publications
// sent to newly assigned recipients.
func Channel(owner, viewer chat.AccountID) string {
	return prefix + string(owner) + ":" + string(viewer)
}

func channelOwner(channel string, viewer chat.AccountID) (chat.AccountID, bool) {
	parts := strings.Split(channel, ":")
	if len(parts) != 3 || parts[0] != "chat" || parts[2] != string(viewer) {
		return "", false
	}
	owner := chat.AccountID(parts[1])
	return owner, owner.Valid() && viewer.Valid()
}

type Policy interface {
	AuthorizeOperation(context.Context, authentication.Caller, chat.AccountID, authorization.Operation) error
}

type LockedAssignments interface {
	WithAssignmentLock(context.Context, string, func(string) error) error
}

type Publisher interface {
	Publish(string, []byte, ...centrifuge.PublishOption) (centrifuge.PublishResult, error)
}

type Service struct {
	Auth        *authentication.Validator
	Permissions Policy
	Assignments LockedAssignments
	Publisher   Publisher
}

// Bind installs FC-02 for every connection/reconnection, then FC-07 for
// every client subscription. No client publications or history are enabled.
func (s Service) Bind(node *centrifuge.Node, publicWebSocketURL string) error {
	if node == nil || s.Auth == nil || s.Permissions == nil || s.Assignments == nil || s.Publisher == nil {
		return ErrDeliveryFailed
	}
	s.Auth.BindOnConnecting(node, publicWebSocketURL)
	node.OnConnect(func(client *centrifuge.Client) {
		client.OnSubscribe(func(event centrifuge.SubscribeEvent, cb centrifuge.SubscribeCallback) {
			viewer := chat.AccountID(client.UserID())
			owner, ok := channelOwner(event.Channel, viewer)
			if !ok || s.Permissions.AuthorizeOperation(client.Context(), authentication.Caller{AccountID: string(viewer)}, owner, authorization.Subscribe) != nil {
				cb(centrifuge.SubscribeReply{}, centrifuge.ErrorPermissionDenied)
				return
			}
			cb(centrifuge.SubscribeReply{}, nil)
		})
	})
	return nil
}

// DeliverCommittedMessage implements FC-09. Invoke only after successful
// FC-11 commit. The supplied recipient is historical, never an authority.
// The lock covers authorization and synchronous publishing, serializing both
// with FC-12. A changed assignment skips recipient delivery entirely.
func (s Service) DeliverCommittedMessage(ctx context.Context, owner, committedRecipient chat.AccountID, data []byte) error {
	if !owner.Valid() || !committedRecipient.Valid() || s.Assignments == nil || s.Publisher == nil || len(data) == 0 {
		return ErrDeliverySkipped
	}
	return s.Assignments.WithAssignmentLock(ctx, string(owner), func(current string) error {
		if !chat.AccountID(current).Valid() {
			return ErrDeliverySkipped
		}
		if err := s.publish(owner, owner, data); err != nil {
			return err
		}
		if current != string(committedRecipient) || current == string(owner) {
			return nil
		}
		return s.publish(owner, committedRecipient, data)
	})
}

func (s Service) publish(owner, viewer chat.AccountID, data []byte) error {
	if _, err := s.Publisher.Publish(Channel(owner, viewer), data); err != nil {
		return ErrDeliveryFailed
	}
	return nil
}

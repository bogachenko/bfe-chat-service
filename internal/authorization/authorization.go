// Package authorization implements DP-M04 FC-07 access decisions.
// Callers must be authenticated with FC-01 at each protected request boundary.
package authorization

import (
	"context"
	"errors"

	"github.com/bogachenko/bfe-chat-service/internal/authentication"
	"github.com/bogachenko/bfe-chat-service/internal/chat"
)

var ErrForbidden = errors.New("FORBIDDEN")

type Operation string

const (
	Send       Operation = "SEND"
	Read       Operation = "READ"
	Assign     Operation = "ASSIGN"
	AdminRead  Operation = "ADMIN_READ"
	Subscribe  Operation = "SUBSCRIBE"
	Deliver    Operation = "DELIVER"
)

// AssignmentStore reads the authoritative, current assignment from PostgreSQL.
// No stored or cached permission snapshot may be substituted.
type AssignmentStore interface {
	GetAssignment(context.Context, string) (string, error)
}

type Authorizer struct {
	Assignments AssignmentStore
}

// AuthorizeOperation applies FC-07 to an already verified Caller.
// A missing assignment or storage failure denies access without revealing data.
func (a Authorizer) AuthorizeOperation(ctx context.Context, caller authentication.Caller, owner chat.AccountID, action Operation) error {
	if !owner.Valid() || !chat.AccountID(caller.AccountID).Valid() || a.Assignments == nil {
		return ErrForbidden
	}
	if action == Assign {
		if caller.AdministrativeStatus == "ADMIN" {
			return nil
		}
		return ErrForbidden
	}
	if action == AdminRead || (action == Read && caller.AdministrativeStatus == "ADMIN") {
		return nil
	}
	recipient, err := a.Assignments.GetAssignment(ctx, string(owner))
	if err != nil || !chat.AccountID(recipient).Valid() {
		return ErrForbidden
	}
	switch action {
	case Send, Read, Subscribe, Deliver:
		if caller.AccountID == string(owner) || caller.AccountID == recipient {
			return nil
		}
	}
	return ErrForbidden
}

// AuthorizeSend satisfies DP-M03's policy boundary.
func (a Authorizer) AuthorizeSend(ctx context.Context, caller authentication.Caller, owner chat.AccountID) error {
	return a.AuthorizeOperation(ctx, caller, owner, Send)
}

// AuthorizeRead also permits current ADMIN audit reads without granting SEND.
// No view-audit events are written by this module.
func (a Authorizer) AuthorizeRead(ctx context.Context, caller authentication.Caller, owner chat.AccountID) error {
	return a.AuthorizeOperation(ctx, caller, owner, Read)
}

// LockedAssignmentStore coordinates authorization with FC-12 replacement.
type LockedAssignmentStore interface {
	WithAssignmentLock(context.Context, string, func(string) error) error
}

// AdmitDelivery checks the current recipient and performs the synchronous
// admission/publish action under the same per-owner lock as FC-12. The supplied
// action must not enqueue deferred publication that can begin after unlock.
func (a Authorizer) AdmitDelivery(ctx context.Context, caller authentication.Caller, owner chat.AccountID, publish func() error) error {
	if !owner.Valid() || !chat.AccountID(caller.AccountID).Valid() || publish == nil {
		return ErrForbidden
	}
	locked, ok := a.Assignments.(LockedAssignmentStore)
	if !ok {
		return ErrForbidden
	}
	return locked.WithAssignmentLock(ctx, string(owner), func(recipient string) error {
		if !chat.AccountID(recipient).Valid() || (caller.AccountID != recipient && caller.AccountID != string(owner)) {
			return ErrForbidden
		}
		return publish()
	})
}

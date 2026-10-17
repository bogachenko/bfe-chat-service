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
	if action == AdminRead {
		if caller.AdministrativeStatus == "ADMIN" {
			return nil
		}
		return ErrForbidden
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

// AuthorizeRead permits only owner/current-recipient reads. Administrative
// audit reads use explicit ADMIN_READ; they never grant sending privileges.
func (a Authorizer) AuthorizeRead(ctx context.Context, caller authentication.Caller, owner chat.AccountID) error {
	return a.AuthorizeOperation(ctx, caller, owner, Read)
}

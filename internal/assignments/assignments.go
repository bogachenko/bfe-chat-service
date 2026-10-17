// Package assignments implements DP-M02's explicit recipient assignment operations.
// Authentication and administrative authorization are rechecked for each change.
package assignments

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bogachenko/bfe-chat-service/internal/authentication"
	"github.com/bogachenko/bfe-chat-service/internal/chat"
)

var (
	ErrForbidden          = errors.New("FORBIDDEN")
	ErrInvalidRecipient   = errors.New("INVALID_RECIPIENT")
	ErrAssignmentConflict = errors.New("ASSIGNMENT_CONFLICT")
	ErrNoAssignment       = errors.New("NO_ASSIGNMENT")
	ErrStorageUnavailable = errors.New("STORAGE_UNAVAILABLE")
)

// Store is the approved persistence boundary. Write implementations must use
// FC-12, which serializes assignment changes with message commits.
type Store interface {
	CommitAssignment(ctx context.Context, owner, recipient string) error
	GetAssignment(ctx context.Context, owner string) (string, error)
}

// Authenticator validates the current BFE session and DPoP proof at the
// protected operation boundary. A cached ADMIN assertion is insufficient.
type Authenticator interface {
	ValidateCaller(context.Context, string, string, authentication.RequestBinding) (authentication.Caller, error)
}

type Service struct {
	Auth  Authenticator
	Store Store
}

// ReplaceRecipient implements FC-03. Only an explicitly selected BFE AccountID
// can be assigned; BFE Auth is not queried for target account status.
func (s Service) ReplaceRecipient(ctx context.Context, token, proof string, binding authentication.RequestBinding, owner, recipient chat.AccountID) (chat.Assignment, error) {
	if s.Auth == nil {
		return chat.Assignment{}, authentication.ErrAuthUnavailable
	}
	caller, err := s.Auth.ValidateCaller(ctx, token, proof, binding)
	if err != nil {
		return chat.Assignment{}, err
	}
	if caller.AdministrativeStatus != "ADMIN" {
		return chat.Assignment{}, ErrForbidden
	}
	a, err := chat.NewAssignment(owner, recipient)
	if err != nil {
		return chat.Assignment{}, ErrInvalidRecipient
	}
	if s.Store == nil {
		return chat.Assignment{}, ErrStorageUnavailable
	}
	if err := s.Store.CommitAssignment(ctx, string(owner), string(recipient)); err != nil {
		var sqlState interface{ SQLState() string }
		if errors.As(err, &sqlState) {
			switch sqlState.SQLState() {
			case "40001", "40P01", "23505":
				return chat.Assignment{}, ErrAssignmentConflict
			}
		}
		return chat.Assignment{}, ErrStorageUnavailable
	}
	return a, nil
}

// GetAssignment implements FC-04 for internal, already-authorized callers.
// Its result is a snapshot, never a grant to send or subscribe.
func (s Service) GetAssignment(ctx context.Context, owner chat.AccountID) (chat.Assignment, error) {
	if !owner.Valid() {
		return chat.Assignment{}, ErrInvalidRecipient
	}
	if s.Store == nil {
		return chat.Assignment{}, ErrStorageUnavailable
	}
	recipient, err := s.Store.GetAssignment(ctx, string(owner))
	if errors.Is(err, sql.ErrNoRows) {
		return chat.Assignment{}, ErrNoAssignment
	}
	if err != nil {
		return chat.Assignment{}, ErrStorageUnavailable
	}
	a, err := chat.NewAssignment(owner, chat.AccountID(recipient))
	if err != nil {
		return chat.Assignment{}, ErrStorageUnavailable
	}
	return a, nil
}

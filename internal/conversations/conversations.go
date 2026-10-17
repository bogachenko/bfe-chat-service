// Package conversations implements the approved DP-M03 send and history
// boundaries. Authentication and authorization are required on every operation.
package conversations

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bogachenko/bfe-chat-service/internal/authentication"
	"github.com/bogachenko/bfe-chat-service/internal/chat"
	"github.com/bogachenko/bfe-chat-service/internal/documentcontext"
	"github.com/bogachenko/bfe-chat-service/internal/storage"
)

var (
	ErrForbidden = errors.New("FORBIDDEN")
	ErrNoAssignment = errors.New("NO_ASSIGNMENT")
	ErrStorageUnavailable = errors.New("STORAGE_UNAVAILABLE")
	ErrValidation = errors.New("VALIDATION_ERROR")
)

// Authenticator revalidates the caller at the protected operation boundary.
type Authenticator interface {
	ValidateCaller(context.Context, string, string, authentication.RequestBinding) (authentication.Caller, error)
}

// Authorizer is the DP-M04 policy boundary. Implementations must check
// authoritative current assignment, not cached connection permissions.
type Authorizer interface {
	AuthorizeSend(context.Context, authentication.Caller, chat.AccountID) error
	AuthorizeRead(context.Context, authentication.Caller, chat.AccountID) error
}

type Store interface {
	CommitMessage(context.Context, string, string, string, string, sql.NullString, sql.NullString) (storage.Message, error)
	ReadHistory(context.Context, string, int64, int) ([]storage.HistoryMessage, error)
}

type Service struct {
	Auth Authenticator
	Permissions Authorizer
	Store Store
}

type MessageInput struct {
	IdempotencyKey string
	Text string
	Document *documentcontext.Document
}

// SendMessage implements FC-05. FC-11 resolves the recipient under the
// transaction lock shared with FC-12. A replay returns the original result.
func (s Service) SendMessage(ctx context.Context, token, proof string, binding authentication.RequestBinding, owner chat.AccountID, input MessageInput) (storage.Message, error) {
	caller, err := s.authenticate(ctx, token, proof, binding)
	if err != nil {
		return storage.Message{}, err
	}
	if !owner.Valid() || !chat.AccountID(input.IdempotencyKey).Valid() {
		return storage.Message{}, ErrValidation
	}
	if s.Permissions == nil || s.Store == nil {
		return storage.Message{}, ErrStorageUnavailable
	}
	if err := s.Permissions.AuthorizeSend(ctx, caller, owner); err != nil {
		return storage.Message{}, ErrForbidden
	}
	content, err := documentcontext.ValidateDocumentContext(input.Text, input.Document)
	if err != nil {
		return storage.Message{}, ErrValidation
	}
	var id, title sql.NullString
	if content.Document != nil {
		id = sql.NullString{String: content.Document.ID, Valid: true}
		title = sql.NullString{String: content.Document.Title, Valid: true}
	}
	result, err := s.Store.CommitMessage(ctx, string(owner), caller.AccountID, input.IdempotencyKey, content.Text, id, title)
	if err != nil {
		return storage.Message{}, classifyWriteError(err)
	}
	return result, nil
}

// ReadHistory implements FC-06. Reading never creates a conversation, writes
// a view audit event, or grants sending permissions.
func (s Service) ReadHistory(ctx context.Context, token, proof string, binding authentication.RequestBinding, owner chat.AccountID, afterID int64, limit int) ([]storage.HistoryMessage, error) {
	caller, err := s.authenticate(ctx, token, proof, binding)
	if err != nil {
		return nil, err
	}
	if !owner.Valid() || afterID < 0 || limit < 1 || limit > 100 {
		return nil, ErrValidation
	}
	if s.Permissions == nil || s.Store == nil {
		return nil, ErrStorageUnavailable
	}
	if err := s.Permissions.AuthorizeRead(ctx, caller, owner); err != nil {
		return nil, ErrForbidden
	}
	result, err := s.Store.ReadHistory(ctx, string(owner), afterID, limit)
	if err != nil {
		return nil, ErrStorageUnavailable
	}
	return result, nil
}

func (s Service) authenticate(ctx context.Context, token, proof string, binding authentication.RequestBinding) (authentication.Caller, error) {
	if s.Auth == nil {
		return authentication.Caller{}, authentication.ErrAuthUnavailable
	}
	return s.Auth.ValidateCaller(ctx, token, proof, binding)
}

func classifyWriteError(err error) error {
	var state interface{ SQLState() string }
	if errors.As(err, &state) {
		switch state.SQLState() {
		case "P0001":
			return ErrNoAssignment
		case "22023", "22P02", "23514":
			return ErrValidation
		}
	}
	return ErrStorageUnavailable
}

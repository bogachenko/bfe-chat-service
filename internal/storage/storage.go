// Package storage implements the approved FC-11/FC-12 persistence boundary.
// Callers MUST authenticate and authorize the operation before invoking it.
// Connect using the approved pgx database/sql driver; no new driver is installed here.
package storage

import (
	"context"
	"database/sql"
	"errors"
)

var ErrInvalidInput = errors.New("missing required account ID or idempotency key")

type Store struct{ DB *sql.DB }

type Message struct {
	ID          int64
	RecipientID string
	Replayed    bool
}

// CommitMessage implements FC-11. Recipient assignment is resolved inside
// PostgreSQL under the same owner lock used by CommitAssignment.
// The caller must not publish an event before this operation succeeds.
func (s Store) CommitMessage(ctx context.Context, owner, sender, key, body string, documentID, documentTitle sql.NullString) (Message, error) {
	if s.DB == nil || owner == "" || sender == "" || key == "" {
		return Message{}, ErrInvalidInput
	}
	var m Message
	err := s.DB.QueryRowContext(ctx, `
		SELECT message_id, recipient_account_id::text, replayed
		FROM chat_commit_message($1::uuid, $2::uuid, $3::uuid, $4, $5, $6)
	`, owner, sender, key, body, documentID, documentTitle).Scan(&m.ID, &m.RecipientID, &m.Replayed)
	if err != nil {
		return Message{}, err
	}
	return m, nil
}

// CommitAssignment implements FC-12 for an already-authorized ADMIN.
// A failed database operation does not replace the existing recipient.
func (s Store) CommitAssignment(ctx context.Context, owner, recipient string) error {
	if s.DB == nil || owner == "" || recipient == "" {
		return ErrInvalidInput
	}
	var saved string
	return s.DB.QueryRowContext(ctx, `
		SELECT recipient_account_id::text
		FROM chat_commit_assignment($1::uuid, $2::uuid)
	`, owner, recipient).Scan(&saved)
}

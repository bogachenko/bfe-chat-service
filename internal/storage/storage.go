// Package storage implements the approved FC-11/FC-12 persistence boundary.
// Callers MUST authenticate and authorize the operation before invoking it.
// Connect using the approved pgx database/sql driver; no new driver is installed here.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
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

// GetAssignment implements the read-only FC-04 storage operation.
// No BFE target-account lookup or assignment mutation is performed.
func (s Store) GetAssignment(ctx context.Context, owner string) (string, error) {
	if s.DB == nil || owner == "" {
		return "", ErrInvalidInput
	}
	var recipient string
	err := s.DB.QueryRowContext(ctx, `
		SELECT recipient_account_id::text
		FROM chat_assignments
		WHERE owner_account_id = $1::uuid
	`, owner).Scan(&recipient)
	return recipient, err
}

type HistoryMessage struct {
	ID             int64
	OwnerID        string
	SenderID       string
	RecipientID    string
	IdempotencyKey string
	Body           string
	DocumentID     sql.NullString
	DocumentTitle  sql.NullString
	CreatedAt      time.Time
}

// ReadHistory implements FC-06 persistence. An owner filter and keyset cursor
// prevent cross-conversation reads. Permission checking belongs to DP-M04.
func (s Store) ReadHistory(ctx context.Context, owner string, afterID int64, limit int) ([]HistoryMessage, error) {
	if s.DB == nil || owner == "" || afterID < 0 || limit < 1 || limit > 100 {
		return nil, ErrInvalidInput
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, owner_account_id::text, sender_account_id::text,
		       recipient_account_id::text, idempotency_key::text, body,
		       document_id, document_title, created_at
		FROM chat_messages
		WHERE owner_account_id = $1::uuid AND id > $2
		ORDER BY id ASC LIMIT $3
	`, owner, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	messages := make([]HistoryMessage, 0)
	for rows.Next() {
		var m HistoryMessage
		if err := rows.Scan(&m.ID, &m.OwnerID, &m.SenderID,
			&m.RecipientID, &m.IdempotencyKey, &m.Body,
			&m.DocumentID, &m.DocumentTitle, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}


// WithAssignmentLock serializes a protected publication decision and its
// synchronous action with FC-12 recipient replacement for the same owner.
// The callback must finish the physical admission to publication before return.
func (s Store) WithAssignmentLock(ctx context.Context, owner string, action func(string) error) error {
	if s.DB == nil || owner == "" || action == nil {
		return ErrInvalidInput
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended($1::uuid::text, 0))
	`, owner); err != nil {
		return err
	}
	var recipient string
	if err := tx.QueryRowContext(ctx, `
		SELECT recipient_account_id::text FROM chat_assignments
		WHERE owner_account_id = $1::uuid
	`, owner).Scan(&recipient); err != nil {
		return err
	}
	if err := action(recipient); err != nil {
		return err
	}
	return tx.Commit()
}

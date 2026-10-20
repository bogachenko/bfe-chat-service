package issue25

import (
	"context"
	"database/sql"
)

// NotificationRecipients returns only active members; the durable registration
// repeats authorization under the shared conversation gate.
func (s Store) NotificationRecipients(ctx context.Context, conversation string) ([]struct{ Viewer, Epoch string }, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT account_id::text,epoch::text FROM issue25_memberships
 WHERE conversation_id=$1 AND leave_boundary IS NULL ORDER BY account_id`, conversation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []struct{ Viewer, Epoch string }{}
	for rows.Next() {
		var r struct{ Viewer, Epoch string }
		if err = rows.Scan(&r.Viewer, &r.Epoch); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
func (s Store) NotifyCommitted(ctx context.Context, p Publisher, conversation string, message Message) error {
	if p == nil || message.Replayed {
		return nil
	}
	viewers, err := s.NotificationRecipients(ctx, conversation)
	if err != nil {
		return err
	}
	for _, v := range viewers {
		id, e := newUUID()
		if e != nil {
			return e
		}
		wake, e := WakeToken()
		if e != nil {
			return e
		}
		a := Attempt{ID: id, Conversation: conversation, Viewer: v.Viewer, Epoch: v.Epoch, MessageID: message.ID, Ordinal: message.Ordinal, WakeToken: wake, Channel: "chat:e:" + v.Epoch}
		if e = s.RegisterAttempt(ctx, a); e != nil {
			if e == ErrForbidden || e == sql.ErrNoRows {
				continue
			}
			return e
		}
		// Notifications are best-effort, and UNKNOWN is not retried.
		_, _ = s.PublishAttempt(ctx, id, p)
	}
	return nil
}

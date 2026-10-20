package issue25

import (
	"context"
	"database/sql"
)

type HistoryItem struct {
	ID       string           `json:"messageId"`
	Ordinal  int64            `json:"ordinal"`
	SenderID string           `json:"senderId"`
	Text     string           `json:"text"`
	Document *HistoryDocument `json:"document,omitempty"`
}
type HistoryDocument struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func (s Store) ReadHistoryDetailed(ctx context.Context, conversation, viewer string, after int64, limit int) ([]HistoryItem, error) {
	if after < 0 || limit < 1 || limit > 100 {
		return nil, ErrInvalid
	}
	out := []HistoryItem{}
	err := s.tx(ctx, conversation, func(tx *sql.Tx) error {
		// Authorization and history snapshot share the same transaction gate as REMOVE.
		if _, _, e := active(tx, ctx, conversation, viewer); e != nil {
			return e
		}
		rows, e := tx.QueryContext(ctx, `SELECT m.id::text,m.ordinal,m.sender_id::text,m.body,m.document_id,m.document_title
  FROM issue25_messages m WHERE m.conversation_id=$1 AND m.ordinal>$3
  AND EXISTS(SELECT 1 FROM issue25_memberships h WHERE h.conversation_id=m.conversation_id AND h.account_id=$2
  AND m.ordinal>=h.join_boundary AND (h.leave_boundary IS NULL OR m.ordinal<h.leave_boundary))
  ORDER BY m.ordinal LIMIT $4`, conversation, viewer, after, limit)
		if e != nil {
			return e
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var item HistoryItem
			var id, title sql.NullString
			if e = rows.Scan(&item.ID, &item.Ordinal, &item.SenderID, &item.Text, &id, &title); e != nil {
				return e
			}
			if id.Valid && title.Valid {
				item.Document = &HistoryDocument{ID: id.String, Title: title.String}
			}
			out = append(out, item)
		}
		return rows.Err()
	})
	return out, err
}

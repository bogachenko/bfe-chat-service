package issue25

import (
	"context"
	"database/sql"
)

type ConversationItem struct {
	ID     string  `json:"id"`
	Kind   string  `json:"kind"`
	Name   *string `json:"name,omitempty"`
	Hidden bool    `json:"hidden"`
	Role   string  `json:"role"`
}

func (s Store) ListConversations(ctx context.Context, user string) ([]ConversationItem, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT c.id::text,c.kind,c.name,coalesce(us.hidden,false),m.role
 FROM issue25_conversations c JOIN issue25_memberships m ON m.conversation_id=c.id
 LEFT JOIN issue25_user_state us ON us.conversation_id=c.id AND us.account_id=m.account_id
 WHERE m.account_id=$1 AND m.leave_boundary IS NULL
 ORDER BY c.created_at DESC,c.id`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ConversationItem{}
	for rows.Next() {
		var x ConversationItem
		var name sql.NullString
		if err = rows.Scan(&x.ID, &x.Kind, &name, &x.Hidden, &x.Role); err != nil {
			return nil, err
		}
		if name.Valid {
			x.Name = &name.String
		}
		result = append(result, x)
	}
	return result, rows.Err()
}

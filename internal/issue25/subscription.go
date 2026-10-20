package issue25

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// AuthorizeEpochChannel accepts opaque membership identifiers only after
// the caller has been validated through UDS + DPoP at WS connection.
func (s Store) AuthorizeEpochChannel(ctx context.Context, channel, viewer string) error {
	if !strings.HasPrefix(channel, "chat:e:") || len(channel) != len("chat:e:")+36 {
		return ErrForbidden
	}
	epoch := strings.TrimPrefix(channel, "chat:e:")
	if s.DB == nil {
		return ErrInvalid
	}
	var matched bool
	err := s.DB.QueryRowContext(ctx, `SELECT EXISTS(
 SELECT 1 FROM issue25_memberships WHERE epoch=$1::uuid AND account_id=$2::uuid AND leave_boundary IS NULL)`, epoch, viewer).Scan(&matched)
	if errors.Is(err, sql.ErrNoRows) || !matched && err == nil {
		return ErrForbidden
	}
	return err
}

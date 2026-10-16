// Package chat defines assignment rules shared by the approved chat modules.
package chat

import (
	"errors"
	"strings"
)

var (
	ErrInvalidAccountID = errors.New("invalid account ID")
	ErrNoAssignment = errors.New("recipient is not assigned")
)

// AccountID is an opaque BFE Drive account UUID. It is not a username.
type AccountID string

// Valid reports whether the identifier has the canonical UUID shape.
// This is syntax validation only; it does not claim the account is active.
func (id AccountID) Valid() bool {
	s := string(id)
	if len(s) != 36 {
		return false
	}
	for i, ch := range s {
		switch i {
		case 8, 13, 18, 23:
			if ch != '-' {
				return false
			}
		default:
			if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f' || ch >= 'A' && ch <= 'F') {
				return false
			}
		}
	}
	return true
}

// Assignment captures an explicit recipient selection, never an automatic pick.
type Assignment struct {
	Owner AccountID
	Recipient AccountID
}

// NewAssignment rejects absent, malformed or self-selected recipient IDs.
// Authorization and persistence must be applied by the caller's approved modules.
func NewAssignment(owner, recipient AccountID) (Assignment, error) {
	if !owner.Valid() || !recipient.Valid() || strings.EqualFold(string(owner), string(recipient)) {
		return Assignment{}, ErrInvalidAccountID
	}
	return Assignment{Owner: owner, Recipient: recipient}, nil
}

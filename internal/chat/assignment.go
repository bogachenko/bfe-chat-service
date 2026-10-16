// Package chat defines assignment rules shared by the approved chat modules.
package chat

import "errors"

var (
	ErrInvalidAccountID = errors.New("invalid account ID")
	ErrNoAssignment     = errors.New("recipient is not assigned")
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
	for i := 0; i < len(s); i++ {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if s[i] != '-' {
				return false
			}
		} else if !isHexDigit(s[i]) {
			return false
		}
	}
	return true
}

func isHexDigit(ch byte) bool {
	return ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f' || ch >= 'A' && ch <= 'F'
}

// Assignment captures an explicit recipient selection, never an automatic pick.
type Assignment struct {
	Owner     AccountID
	Recipient AccountID
}

// NewAssignment rejects absent or malformed account IDs.
// Authorization and persistence must be applied by the caller's approved modules.
func NewAssignment(owner, recipient AccountID) (Assignment, error) {
	if !owner.Valid() || !recipient.Valid() {
		return Assignment{}, ErrInvalidAccountID
	}
	return Assignment{Owner: owner, Recipient: recipient}, nil
}

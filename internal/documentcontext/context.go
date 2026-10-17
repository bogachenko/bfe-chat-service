// Package documentcontext validates per-message references to BFE Drive documents.
// It does not fetch documents, store file contents, or perform Drive ACL checks.
package documentcontext

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// ErrValidation indicates that a message has no content or invalid document metadata.
var ErrValidation = errors.New("VALIDATION_ERROR")

// Document is a reference owned by an individual message, not a conversation.
type Document struct {
	ID    string
	Title string
}

// MessageContent is validated content to be persisted by the message module.
type MessageContent struct {
	Text     string
	Document *Document
}

// ValidateDocumentContext implements FC-10. A message may contain text,
// one document reference, or both. Missing or malformed references fail closed.
func ValidateDocumentContext(text string, document *Document) (MessageContent, error) {
	normalized := MessageContent{Text: strings.TrimSpace(text)}
	if document != nil {
		id := strings.TrimSpace(document.ID)
		title := strings.TrimSpace(document.Title)
		if !validValue(id) || !validValue(title) {
			return MessageContent{}, ErrValidation
		}
		normalized.Document = &Document{ID: id, Title: title}
	}
	if !validText(normalized.Text) || (normalized.Text == "" && normalized.Document == nil) {
		return MessageContent{}, ErrValidation
	}
	return normalized, nil
}

// PostgreSQL text cannot store NUL or malformed UTF-8. Reject invalid
// message metadata as VALIDATION_ERROR before reaching the persistence layer.
func validText(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}

func validValue(value string) bool {
	return value != "" && validText(value)
}

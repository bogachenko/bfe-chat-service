// Package transport implements DP-M08 FC-13/FC-14 protocol adaptation.
// All authentication, authorization, storage, and delivery decisions belong to
// the underlying DP-M01 through DP-M07 services.
package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/bogachenko/bfe-chat-service/internal/assignments"
	"github.com/bogachenko/bfe-chat-service/internal/authentication"
	"github.com/bogachenko/bfe-chat-service/internal/conversations"
	"github.com/bogachenko/bfe-chat-service/internal/realtime"
	"github.com/centrifugal/centrifuge"
)

type Handler struct {
	Messages     conversations.Service
	Assignments  assignments.Service
	WebSocket    http.Handler
	Realtime     realtime.Service
	PublicOrigin string
}

func NewHandler(messages conversations.Service, recipient assignments.Service, realTime realtime.Service, node *centrifuge.Node, socketURL, publicOrigin string) (http.Handler, error) {
	if node == nil || messages.Auth == nil || messages.Permissions == nil || messages.Store == nil {
		return nil, errors.New("transport dependencies missing")
	}
	if recipient.Auth == nil || recipient.Store == nil {
		return nil, errors.New("assignment dependencies missing")
	}
	if err := validateOrigin(socketURL, publicOrigin); err != nil {
		return nil, err
	}
	if err := realTime.Bind(node, socketURL); err != nil {
		return nil, err
	}
	return Handler{Messages: messages, Assignments: recipient, WebSocket: centrifuge.NewWebsocketHandler(node, centrifuge.WebsocketConfig{}), Realtime: realTime, PublicOrigin: strings.TrimSuffix(publicOrigin, "/")}, nil
}
func validateOrigin(socketURL, publicOrigin string) error {
	origin, err := url.Parse(publicOrigin)
	if err != nil {
		return errors.New("invalid public origin")
	}
	if err := validatePublicOrigin(origin); err != nil {
		return err
	}
	socket, err := url.Parse(socketURL)
	if err != nil {
		return errors.New("invalid websocket endpoint")
	}
	return validateSocketURL(socket, origin.Host)
}

func validatePublicOrigin(origin *url.URL) error {
	if origin.Scheme != "https" || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" {
		return errors.New("invalid public origin")
	}
	if origin.Path != "" && origin.Path != "/" {
		return errors.New("invalid origin path")
	}
	return nil
}

func validateSocketURL(socket *url.URL, host string) error {
	if socket.Scheme != "wss" || socket.Host != host || socket.Path != "/ws" {
		return errors.New("invalid websocket endpoint")
	}
	if socket.User != nil || socket.RawQuery != "" || socket.Fragment != "" {
		return errors.New("invalid websocket endpoint")
	}
	return nil
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.RawQuery != "" || r.URL.User != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	if r.URL.Path == "/ws" {
		h.serveSocket(w, r)
		return
	}
	writeError(w, http.StatusNotFound, "NOT_FOUND")
}
func (h Handler) serveSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	if h.WebSocket == nil {
		writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	h.WebSocket.ServeHTTP(w, r)
}
func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
}
func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, authentication.ErrAuthInvalid), errors.Is(err, authentication.ErrProofInvalid):
		writeError(w, http.StatusUnauthorized, "AUTH_INVALID")
	case errors.Is(err, authentication.ErrAuthUnavailable):
		writeError(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE")
	case errors.Is(err, conversations.ErrForbidden), errors.Is(err, assignments.ErrForbidden):
		writeError(w, http.StatusForbidden, "FORBIDDEN")
	case errors.Is(err, conversations.ErrValidation), errors.Is(err, assignments.ErrInvalidRecipient):
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR")
	case errors.Is(err, conversations.ErrNoAssignment), errors.Is(err, assignments.ErrNoAssignment):
		writeError(w, http.StatusConflict, "NO_ASSIGNMENT")
	case errors.Is(err, assignments.ErrAssignmentConflict):
		writeError(w, http.StatusConflict, "ASSIGNMENT_CONFLICT")
	default:
		writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
	}
}
func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

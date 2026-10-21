// Package transport implements DP-M08 FC-13/FC-14 protocol adaptation.
// All authentication, authorization, storage, and delivery decisions belong to
// the underlying DP-M01 through DP-M07 services.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/bogachenko/bfe-chat-service/internal/authentication"
	"github.com/centrifugal/centrifuge"
)

type Handler struct {
	WebSocket http.Handler
	PublicOrigin string
}

// NewCurrentHandler binds the current DPoP-authenticated epoch-channel
// WebSocket transport without initializing retired assignment services.
func NewCurrentHandler(auth *authentication.Validator, epochs interface {
	AuthorizeEpochChannel(context.Context, string, string) error
}, node *centrifuge.Node, socketURL, publicOrigin string) (http.Handler, error) {
	if auth == nil || epochs == nil || node == nil {
		return nil, errors.New("transport dependencies missing")
	}
	if err := validateOrigin(socketURL, publicOrigin); err != nil {
		return nil, err
	}
	auth.BindOnConnecting(node, socketURL)
	node.OnConnect(func(client *centrifuge.Client) {
		client.OnSubscribe(func(event centrifuge.SubscribeEvent, cb centrifuge.SubscribeCallback) {
			if epochs.AuthorizeEpochChannel(client.Context(), event.Channel, client.UserID()) != nil {
				cb(centrifuge.SubscribeReply{}, centrifuge.ErrorPermissionDenied)
				return
			}
			cb(centrifuge.SubscribeReply{}, nil)
		})
	})
	return Handler{
		WebSocket:    centrifuge.NewWebsocketHandler(node, centrifuge.WebsocketConfig{}),
		PublicOrigin: strings.TrimSuffix(publicOrigin, "/"),
	}, nil
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
func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

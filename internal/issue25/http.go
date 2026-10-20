package issue25

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/bogachenko/bfe-chat-service/internal/authentication"
	"github.com/centrifugal/centrifuge"
)

type CallerValidator interface {
	ValidateCaller(context.Context, string, string, authentication.RequestBinding) (authentication.Caller, error)
}
type SubscriptionRevoker interface {
	Unsubscribe(string, string, ...centrifuge.UnsubscribeOption) error
}
type HTTPHandler struct {
	Revoker      SubscriptionRevoker
	Publisher    Publisher
	Store        Store
	Auth         CallerValidator
	PublicOrigin string
}

func (h HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if h.Auth == nil || h.Store.DB == nil {
		write25(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
		return
	}
	if r.URL.RawQuery != "" {
		write25(w, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	token, proof := r.Header.Get("Authorization"), r.Header.Get("DPoP")
	if !strings.HasPrefix(token, "Bearer ") || proof == "" {
		write25(w, http.StatusUnauthorized, "AUTH_INVALID")
		return
	}
	caller, err := h.Auth.ValidateCaller(r.Context(), strings.TrimPrefix(token, "Bearer "), proof, authentication.RequestBinding{Method: r.Method, Target: strings.TrimSuffix(h.PublicOrigin, "/") + r.URL.EscapedPath()})
	if err != nil {
		http25Error(w, err)
		return
	}
	user := caller.AccountID
	path := strings.TrimPrefix(r.URL.Path, "/v2/conversations")
	if path == r.URL.Path {
		write25(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if path == "" || path == "/" {
		if r.Method != "GET" {
			write25(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
			return
		}
		conversations, err := h.Store.ListConversations(r.Context(), user)
		if err != nil {
			http25Error(w, err)
			return
		}
		write25JSON(w, conversations)
		return
	}
	if len(parts) == 1 && parts[0] == "direct" && r.Method == "POST" {
		var req struct {
			PeerID string `json:"peerId"`
		}
		if !decode25(w, r, &req) {
			return
		}
		if !validUUID(req.PeerID) {
			write25(w, 400, "VALIDATION_ERROR")
			return
		}
		id, err := h.Store.OpenDirect(r.Context(), user, req.PeerID)
		if err != nil {
			http25Error(w, err)
			return
		}
		// Explicitly opening a direct chat restores the caller's own visibility.
		if err = h.Store.HideDirect(r.Context(), id, user, false); err != nil {
			http25Error(w, err)
			return
		}
		write25JSON(w, map[string]string{"conversationId": id})
		return
	}
	if len(parts) == 1 && parts[0] == "group" && r.Method == "POST" {
		var req struct {
			Name    string   `json:"name"`
			Members []string `json:"members"`
		}
		if !decode25(w, r, &req) {
			return
		}
		for _, member := range req.Members {
			if !validUUID(member) {
				write25(w, 400, "VALIDATION_ERROR")
				return
			}
		}
		id, err := h.Store.CreateGroup(r.Context(), user, req.Name, req.Members)
		if err != nil {
			http25Error(w, err)
			return
		}
		write25JSON(w, map[string]string{"conversationId": id})
		return
	}
	if len(parts) < 2 || parts[0] == "" {
		write25(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	conversation, verb := parts[0], parts[1]
	if !validUUID(conversation) {
		write25(w, 400, "VALIDATION_ERROR")
		return
	}
	switch {
	case len(parts) == 2 && verb == "messages" && r.Method == "POST":
		var req struct {
			IdempotencyKey string `json:"idempotencyKey"`
			Text           string `json:"text"`
			Document       *struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"document"`
		}
		if !decode25(w, r, &req) {
			return
		}
		if !validUUID(req.IdempotencyKey) {
			write25(w, 400, "VALIDATION_ERROR")
			return
		}
		p := Payload{Text: req.Text}
		if req.Document != nil {
			p.DocumentID = &req.Document.ID
			p.DocumentTitle = &req.Document.Title
		}
		result, err := h.Store.Send(r.Context(), conversation, user, req.IdempotencyKey, p)
		if err != nil {
			http25Error(w, err)
			return
		}
		// Message persistence is authoritative; notification failure never undoes it.
		_ = h.Store.NotifyCommitted(r.Context(), h.Publisher, conversation, result)
		write25JSON(w, result)
		return
	case len(parts) == 2 && verb == "history" && r.Method == "GET":
		after := int64(0)
		limit := 50
		if v := r.Header.Get("X-After-Ordinal"); v != "" {
			x, e := strconv.ParseInt(v, 10, 64)
			if e != nil || x < 0 {
				write25(w, 400, "VALIDATION_ERROR")
				return
			}
			after = x
		}
		if v := r.Header.Get("X-Limit"); v != "" {
			x, e := strconv.Atoi(v)
			if e != nil {
				write25(w, 400, "VALIDATION_ERROR")
				return
			}
			limit = x
		}
		result, err := h.Store.ReadHistoryDetailed(r.Context(), conversation, user, after, limit)
		if err != nil {
			http25Error(w, err)
			return
		}
		write25JSON(w, result)
		return
	case len(parts) == 2 && verb == "unread" && r.Method == "GET":
		n, err := h.Store.Unread(r.Context(), conversation, user)
		if err != nil {
			http25Error(w, err)
			return
		}
		write25JSON(w, map[string]int64{"unread": n})
		return
	case len(parts) == 2 && verb == "read" && r.Method == "PUT":
		var req struct {
			Ordinal int64 `json:"ordinal"`
		}
		if !decode25(w, r, &req) {
			return
		}
		n, err := h.Store.MarkRead(r.Context(), conversation, user, req.Ordinal)
		if err != nil {
			http25Error(w, err)
			return
		}
		write25JSON(w, map[string]int64{"lastReadOrdinal": n})
		return
	case len(parts) == 2 && verb == "hidden" && r.Method == "PUT":
		var req struct {
			Hidden bool `json:"hidden"`
		}
		if !decode25(w, r, &req) {
			return
		}
		err := h.Store.HideDirect(r.Context(), conversation, user, req.Hidden)
		if err != nil {
			http25Error(w, err)
			return
		}
		write25JSON(w, map[string]bool{"hidden": req.Hidden})
		return
	case len(parts) == 2 && verb == "members" && r.Method == "POST":
		var req struct {
			AccountID string `json:"accountId"`
			Role      string `json:"role"`
		}
		if !decode25(w, r, &req) {
			return
		}
		if req.Role == "" {
			req.Role = "member"
		}
		if !validUUID(req.AccountID) {
			write25(w, 400, "VALIDATION_ERROR")
			return
		}
		epoch, err := h.Store.Join(r.Context(), conversation, user, req.AccountID, req.Role)
		if err != nil {
			http25Error(w, err)
			return
		}
		write25JSON(w, map[string]string{"epoch": epoch})
		return
	case len(parts) == 3 && verb == "members" && r.Method == "DELETE":
		target := parts[2]
		if !validUUID(target) {
			write25(w, 400, "VALIDATION_ERROR")
			return
		}
		// The retired epoch must originate from the same transaction as REMOVE.
		retiredEpoch, err := h.Store.RemoveWithEpoch(r.Context(), conversation, user, target)
		if err != nil {
			http25Error(w, err)
			return
		}
		if retiredEpoch != "" && h.Revoker != nil {
			// ponytail: best-effort revocation is safe because PostgreSQL is the access gate.
			_ = h.Revoker.Unsubscribe(target, "chat:e:"+retiredEpoch)
		}
		write25JSON(w, map[string]bool{"removed": true})
		return
	case len(parts) == 2 && verb == "owner" && r.Method == "PUT":
		var req struct {
			AccountID string `json:"accountId"`
		}
		if !decode25(w, r, &req) {
			return
		}
		if !validUUID(req.AccountID) {
			write25(w, 400, "VALIDATION_ERROR")
			return
		}
		err := h.Store.TransferOwner(r.Context(), conversation, user, req.AccountID)
		if err != nil {
			http25Error(w, err)
			return
		}
		write25JSON(w, map[string]string{"ownerId": req.AccountID})
		return
	case len(parts) == 3 && verb == "admins" && r.Method == "PUT":
		var req struct {
			Admin bool `json:"admin"`
		}
		if !decode25(w, r, &req) {
			return
		}
		if !validUUID(parts[2]) {
			write25(w, 400, "VALIDATION_ERROR")
			return
		}
		err := h.Store.ChangeAdmin(r.Context(), conversation, user, parts[2], req.Admin)
		if err != nil {
			http25Error(w, err)
			return
		}
		write25JSON(w, map[string]bool{"admin": req.Admin})
		return
	}
	write25(w, http.StatusNotFound, "NOT_FOUND")
}
func decode25(w http.ResponseWriter, r *http.Request, d any) bool {
	if r.Body == nil {
		write25(w, 400, "VALIDATION_ERROR")
		return false
	}
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, 65537))
	if err != nil || len(raw) > 65536 || len(raw) == 0 {
		write25(w, 400, "VALIDATION_ERROR")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(d); err != nil {
		write25(w, 400, "VALIDATION_ERROR")
		return false
	}
	var trailing any
	if err = decoder.Decode(&trailing); err != io.EOF {
		write25(w, 400, "VALIDATION_ERROR")
		return false
	}
	return true
}
func http25Error(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, authentication.ErrAuthInvalid), errors.Is(e, authentication.ErrProofInvalid):
		write25(w, 401, "AUTH_INVALID")
	case errors.Is(e, authentication.ErrAuthUnavailable):
		write25(w, 503, "AUTH_UNAVAILABLE")
	case errors.Is(e, ErrForbidden):
		write25(w, 403, "FORBIDDEN")
	case errors.Is(e, ErrConflict):
		write25(w, 409, "CONFLICT_409")
	case errors.Is(e, ErrInvalid):
		write25(w, 400, "VALIDATION_ERROR")
	default:
		write25(w, 503, "STORAGE_UNAVAILABLE")
	}
}
func write25(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
func write25JSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

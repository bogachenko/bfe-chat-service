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

// credentials25 rejects ambiguous Authorization headers before reaching Internal Auth.
func validProofHeader25(proof string) bool {
	return proof != "" && strings.TrimSpace(proof) == proof && !strings.ContainsAny(proof, " \t\r\n")
}

func credentials25(r *http.Request) (string, string, bool) {
	values := r.Header.Values("Authorization")
	proofs := r.Header.Values("DPoP")
	if len(values) != 1 || len(proofs) != 1 || !validProofHeader25(proofs[0]) {
		return "", "", false
	}
	fields := strings.Split(values[0], " ")
	if len(fields) != 2 || fields[0] != "DPoP" || fields[1] == "" || strings.ContainsAny(fields[1], " \t\r\n") {
		return "", "", false
	}
	return fields[1], proofs[0], true
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
	token, proof, ok := credentials25(r)
	if !ok {
		write25(w, http.StatusUnauthorized, "AUTH_INVALID")
		return
	}
	caller, err := h.Auth.ValidateCaller(r.Context(), token, proof, authentication.RequestBinding{Method: r.Method, Target: strings.TrimSuffix(h.PublicOrigin, "/") + r.URL.EscapedPath()})
	if err != nil {
		http25Error(w, err)
		return
	}
	h.route25(w, r, caller.AccountID)
}
func (h HTTPHandler) route25(w http.ResponseWriter, r *http.Request, user string) {
	path := strings.TrimPrefix(r.URL.Path, "/conversations")
	if path == r.URL.Path || (path != "" && !strings.HasPrefix(path, "/")) {
		write25(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if path == "" || path == "/" {
		h.route25_list(w, r, user)
		return
	}
	if len(parts) == 1 {
		h.route25_single(w, r, user, parts[0])
		return
	}
	if parts[0] == "" {
		write25(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	conversation, verb := parts[0], parts[1]
	if !validUUID(conversation) {
		write25(w, 400, "VALIDATION_ERROR")
		return
	}
	handlers := map[string]func(http.ResponseWriter, *http.Request, string, string, []string){
		"POST messages 2":  h.route25_post_messages_2,
		"GET history 2":    h.route25_get_history_2,
		"GET unread 2":     h.route25_get_unread_2,
		"PUT read 2":       h.route25_put_read_2,
		"PUT hidden 2":     h.route25_put_hidden_2,
		"POST members 2":   h.route25_post_members_2,
		"DELETE members 3": h.route25_delete_members_3,
		"PUT owner 2":      h.route25_put_owner_2,
		"PUT admins 3":     h.route25_put_admins_3,
		"PUT name 2":       h.route32_rename_group,
	}
	fn := handlers[r.Method+" "+verb+" "+strconv.Itoa(len(parts))]
	if fn == nil {
		write25(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	fn(w, r, user, conversation, parts)
}
func (h HTTPHandler) route25_single(w http.ResponseWriter, r *http.Request, user, kind string) {
	if kind == "direct" && r.Method == "POST" {
		h.route25_direct(w, r, user)
		return
	}
	if kind == "group" && r.Method == "POST" {
		h.route25_group(w, r, user)
		return
	}
	write25(w, http.StatusNotFound, "NOT_FOUND")
}
func (h HTTPHandler) route25_list(w http.ResponseWriter, r *http.Request, user string) {
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

}
func (h HTTPHandler) route25_direct(w http.ResponseWriter, r *http.Request, user string) {
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

}
func (h HTTPHandler) route25_group(w http.ResponseWriter, r *http.Request, user string) {
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

}
func (h HTTPHandler) route25_post_messages_2(w http.ResponseWriter, r *http.Request, user, conversation string, parts []string) {

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
}
func (h HTTPHandler) route25_get_history_2(w http.ResponseWriter, r *http.Request, user, conversation string, parts []string) {

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
}
func (h HTTPHandler) route25_get_unread_2(w http.ResponseWriter, r *http.Request, user, conversation string, parts []string) {

	n, err := h.Store.Unread(r.Context(), conversation, user)
	if err != nil {
		http25Error(w, err)
		return
	}
	write25JSON(w, map[string]int64{"unread": n})
}
func (h HTTPHandler) route25_put_read_2(w http.ResponseWriter, r *http.Request, user, conversation string, parts []string) {

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
}
func (h HTTPHandler) route25_put_hidden_2(w http.ResponseWriter, r *http.Request, user, conversation string, parts []string) {

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
}
func (h HTTPHandler) route25_post_members_2(w http.ResponseWriter, r *http.Request, user, conversation string, parts []string) {

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
}
func (h HTTPHandler) route25_delete_members_3(w http.ResponseWriter, r *http.Request, user, conversation string, parts []string) {

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
}
func (h HTTPHandler) route25_put_owner_2(w http.ResponseWriter, r *http.Request, user, conversation string, parts []string) {

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
}
func (h HTTPHandler) route25_put_admins_3(w http.ResponseWriter, r *http.Request, user, conversation string, parts []string) {

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
}
func (h HTTPHandler) route32_rename_group(w http.ResponseWriter, r *http.Request, user, conversation string, parts []string) {
	var req struct {
		Name string `json:"name"`
	}
	if !decode25(w, r, &req) {
		return
	}
	if err := h.Store.RenameGroup(r.Context(), conversation, user, req.Name); err != nil {
		http25Error(w, err)
		return
	}
	write25JSON(w, map[string]string{"name": req.Name})
}
func decode25(w http.ResponseWriter, r *http.Request, d any) bool {
	if r.Body == nil {
		write25(w, 400, "VALIDATION_ERROR")
		return false
	}
	defer func() { _ = r.Body.Close() }()
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

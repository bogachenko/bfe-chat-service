package issue25

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bogachenko/bfe-chat-service/internal/authentication"
)

type captureHTTPAuth25 struct {
	calls int
	token string
	proof string
	binding authentication.RequestBinding
}

func (a *captureHTTPAuth25) ValidateCaller(_ context.Context, token, proof string, binding authentication.RequestBinding) (authentication.Caller, error) {
	a.calls++
	a.token, a.proof, a.binding = token, proof, binding
	return authentication.Caller{AccountID: "account"}, nil
}

func TestHTTPv2AuthorizationDPoPCompatibility(t *testing.T) {
	cases := []struct {
		name, authorization, proof string
		authCalls int
		status int
	}{
		{"DPoP", "DPoP opaque-access-token", "signed-proof", 1, http.StatusNotFound},
		{"Bearer forbidden despite proof", "Bearer opaque-access-token", "signed-proof", 0, http.StatusUnauthorized},
		{"missing proof", "DPoP opaque-access-token", "", 0, http.StatusUnauthorized},
		{"missing scheme", "opaque-access-token", "signed-proof", 0, http.StatusUnauthorized},
		{"wrong scheme", "Basic opaque-access-token", "signed-proof", 0, http.StatusUnauthorized},
		{"empty DPoP token", "DPoP ", "signed-proof", 0, http.StatusUnauthorized},
		{"empty Bearer token", "Bearer ", "signed-proof", 0, http.StatusUnauthorized},
		{"spaces token", "DPoP    ", "signed-proof", 0, http.StatusUnauthorized},
		{"leading spaces", "DPoP  opaque-access-token", "signed-proof", 0, http.StatusUnauthorized},
		{"trailing spaces", "DPoP opaque-access-token ", "signed-proof", 0, http.StatusUnauthorized},
		{"combined schemes", "Bearer DPoP opaque-access-token", "signed-proof", 0, http.StatusUnauthorized},
		{"invalid proof", "DPoP opaque-access-token", "not a proof", 0, http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := &captureHTTPAuth25{}
			h := HTTPHandler{Auth: auth, Store: Store{DB: &sql.DB{}}, PublicOrigin: "https://chat.example.test"}
			req := httptest.NewRequest(http.MethodGet, "/conversations/unknown", nil)
			req.Header.Set("Authorization", tc.authorization)
			req.Header.Set("DPoP", tc.proof)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.status || auth.calls != tc.authCalls {
				t.Fatalf("HTTP=%d authCalls=%d; want HTTP=%d authCalls=%d", rec.Code, auth.calls, tc.status, tc.authCalls)
			}
			if tc.authCalls == 1 && (auth.token != "opaque-access-token" || auth.proof != tc.proof ||
				auth.binding.Method != http.MethodGet || auth.binding.Target != "https://chat.example.test/conversations/unknown") {
				t.Fatalf("wrong authentication forwarding: token=%q proof=%q binding=%+v", auth.token, auth.proof, auth.binding)
			}
		})
	}
}

func TestHTTPv2DuplicateAuthorizationRejected(t *testing.T) {
	auth := &captureHTTPAuth25{}
	h := HTTPHandler{Auth: auth, Store: Store{DB: &sql.DB{}}, PublicOrigin: "https://chat.example.test"}
	req := httptest.NewRequest(http.MethodGet, "/conversations/unknown", nil)
	req.Header.Add("Authorization", "DPoP opaque-access-token")
	req.Header.Add("Authorization", "Bearer opaque-access-token")
	req.Header.Set("DPoP", "signed-proof")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || auth.calls != 0 {
		t.Fatalf("duplicate headers: code=%d calls=%d", rec.Code, auth.calls)
	}
}

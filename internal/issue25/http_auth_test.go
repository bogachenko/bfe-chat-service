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
		{"legacy Bearer with proof", "Bearer opaque-access-token", "signed-proof", 1, http.StatusNotFound},
		{"missing proof", "DPoP opaque-access-token", "", 0, http.StatusUnauthorized},
		{"missing scheme", "opaque-access-token", "signed-proof", 0, http.StatusUnauthorized},
		{"wrong scheme", "Basic opaque-access-token", "signed-proof", 0, http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := &captureHTTPAuth25{}
			h := HTTPHandler{Auth: auth, Store: Store{DB: &sql.DB{}}, PublicOrigin: "https://chat.example.test"}
			req := httptest.NewRequest(http.MethodGet, "/v2/conversations/unknown", nil)
			req.Header.Set("Authorization", tc.authorization)
			req.Header.Set("DPoP", tc.proof)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.status || auth.calls != tc.authCalls {
				t.Fatalf("HTTP=%d authCalls=%d; want HTTP=%d authCalls=%d", rec.Code, auth.calls, tc.status, tc.authCalls)
			}
			if tc.authCalls == 1 && (auth.token != "opaque-access-token" || auth.proof != tc.proof ||
				auth.binding.Method != http.MethodGet || auth.binding.Target != "https://chat.example.test/v2/conversations/unknown") {
				t.Fatalf("wrong authentication forwarding: token=%q proof=%q binding=%+v", auth.token, auth.proof, auth.binding)
			}
		})
	}
}

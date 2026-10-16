// Package authentication implements FC-01 and FC-02 against the BFE Drive
// internal auth API at commit 9a33850a5456d8e3ff40f5cdab6ded869da7110f.
// It does not issue or persist sessions, JWTs, or connection tickets.
package authentication

import (
	"context"
	"crypto"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

var (
	ErrAuthInvalid     = errors.New("AUTH_INVALID")
	ErrAuthUnavailable = errors.New("AUTH_UNAVAILABLE")
	ErrProofInvalid    = errors.New("PROOF_INVALID")
)

type Caller struct {
	AccountID            string
	AdministrativeStatus string
}

// RequestBinding is supplied by the trusted HTTP or Centrifuge boundary,
// never derived from untrusted proof claims.
type RequestBinding struct {
	Method string
	Target string // public HTTPS URL, without credentials or fragment
}

type replayKey struct {
	JKT, JTI string
}

// Validator must be shared across all connections within one service process.
// Do not instantiate a fresh validator for each reconnect.
type Validator struct {
	httpClient *http.Client
	freshness  time.Duration
	mu         sync.Mutex
	seen       map[replayKey]time.Time
}

func NewValidator(socketPath string, freshness time.Duration) (*Validator, error) {
	if socketPath == "" || freshness <= 0 {
		return nil, ErrAuthUnavailable
	}
	transport := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}
	return &Validator{
		httpClient: &http.Client{Transport: transport, Timeout: 3 * time.Second},
		freshness:  freshness,
		seen:       make(map[replayKey]time.Time),
	}, nil
}

type driveIdentity struct {
	AccountID            string `json:"accountId"`
	DPoPJKT              string `json:"dpopJkt"`
	AdministrativeStatus string `json:"administrativeStatus"`
}

// ValidateCaller implements FC-01. The UDS transport and HTTP response are
// never exposed as trusted caller identity unless DPoP also verifies.
func (v *Validator) ValidateCaller(ctx context.Context, accessToken, proof string, binding RequestBinding) (Caller, error) {
	if v == nil || v.httpClient == nil || accessToken == "" || proof == "" ||
		strings.ContainsAny(accessToken, " \\t\\r\\n") {
		return Caller{}, ErrAuthInvalid
	}
	identity, err := v.validateSession(ctx, accessToken)
	if err != nil {
		return Caller{}, err
	}
	if err := v.validateProof(proof, accessToken, identity.DPoPJKT, binding); err != nil {
		return Caller{}, ErrProofInvalid
	}
	return Caller{AccountID: identity.AccountID, AdministrativeStatus: identity.AdministrativeStatus}, nil
}

func (v *Validator) validateSession(ctx context.Context, accessToken string) (driveIdentity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix/internal/auth/validate", nil)
	if err != nil {
		return driveIdentity{}, ErrAuthUnavailable
	}
	req.Header.Set("Authorization", "DPoP "+accessToken)
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return driveIdentity{}, ErrAuthUnavailable
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized {
		return driveIdentity{}, ErrAuthInvalid
	}
	if resp.StatusCode != http.StatusOK {
		return driveIdentity{}, ErrAuthUnavailable
	}
	var identity driveIdentity
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 4096)).Decode(&identity); err != nil {
		return driveIdentity{}, ErrAuthUnavailable
	}
	if identity.AccountID == "" || identity.DPoPJKT == "" ||
		(identity.AdministrativeStatus != "ADMIN" && identity.AdministrativeStatus != "NON_ADMIN") {
		return driveIdentity{}, ErrAuthUnavailable
	}
	return identity, nil
}

// ValidateOnConnecting implements FC-02's authentication decision for a
// Centrifuge OnConnecting callback. Every invocation revalidates the BFE
// session and consumes a new proof; subscription ACL belongs to DP-M04.
func (v *Validator) ValidateOnConnecting(ctx context.Context, accessToken, proof string, publicWebSocketURL string) (Caller, error) {
	target, err := url.Parse(publicWebSocketURL)
	if err != nil || target.Scheme != "wss" || target.Host == "" || target.RawQuery != "" || target.Fragment != "" || target.User != nil {
		return Caller{}, ErrProofInvalid
	}
	target.Scheme = "https"
	return v.ValidateCaller(ctx, accessToken, proof, RequestBinding{
		Method: http.MethodGet,
		Target: target.String(),
	})
}

type proofClaims struct {
	JTI      string           `json:"jti"`
	Method   string           `json:"htm"`
	URI      string           `json:"htu"`
	IssuedAt *jwt.NumericDate `json:"iat"`
	ATH      string           `json:"ath"`
}

var allowedAlgorithms = []jose.SignatureAlgorithm{
	jose.EdDSA, jose.ES256, jose.ES384, jose.ES512,
	jose.RS256, jose.RS384, jose.RS512,
	jose.PS256, jose.PS384, jose.PS512,
}

func (v *Validator) validateProof(rawProof, accessToken, expectedJKT string, binding RequestBinding) error {
	expectedURI, err := canonicalTarget(binding.Target, false)
	if err != nil || binding.Method == "" {
		return ErrProofInvalid
	}
	claims, err := verifiedClaims(rawProof, expectedJKT)
	if err != nil || claims.Method != binding.Method {
		return ErrProofInvalid
	}
	actualURI, err := canonicalTarget(claims.URI, true)
	if err != nil || actualURI != expectedURI {
		return ErrProofInvalid
	}
	hash := sha256.Sum256([]byte(accessToken))
	if claims.ATH != base64.RawURLEncoding.EncodeToString(hash[:]) {
		return ErrProofInvalid
	}
	now := time.Now().UTC()
	issued := claims.IssuedAt.Time().UTC()
	expires := issued.Add(v.freshness)
	if issued.After(now.Add(time.Minute)) || !now.Before(expires) {
		return ErrProofInvalid
	}
	return v.checkReplay(replayKey{JKT: expectedJKT, JTI: claims.JTI}, now, expires)
}

func verifiedClaims(rawProof, expectedJKT string) (proofClaims, error) {
	token, err := jwt.ParseSigned(rawProof, allowedAlgorithms)
	if err != nil || len(token.Headers) != 1 {
		return proofClaims{}, ErrProofInvalid
	}
	header := token.Headers[0]
	if !validProofHeader(header) {
		return proofClaims{}, ErrProofInvalid
	}
	var claims proofClaims
	if err := token.Claims(header.JSONWebKey.Key, &claims); err != nil || claims.JTI == "" || claims.IssuedAt == nil {
		return proofClaims{}, ErrProofInvalid
	}
	thumbprint, err := header.JSONWebKey.Thumbprint(crypto.SHA256)
	if err != nil || base64.RawURLEncoding.EncodeToString(thumbprint) != expectedJKT {
		return proofClaims{}, ErrProofInvalid
	}
	return claims, nil
}

func validProofHeader(header jose.Header) bool {
	return header.JSONWebKey != nil && header.JSONWebKey.Valid() &&
		header.JSONWebKey.IsPublic() && header.ExtraHeaders[jose.HeaderType] == "dpop+jwt"
}

func (v *Validator) checkReplay(key replayKey, now, expires time.Time) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	// ponytail: O(n) expiry sweep and one process-local lock; for multiple
	// instances, replace with a shared atomic replay store before deployment.
	for k, deadline := range v.seen {
		if !now.Before(deadline) {
			delete(v.seen, k)
		}
	}
	if deadline, exists := v.seen[key]; exists && now.Before(deadline) {
		return ErrProofInvalid
	}
	v.seen[key] = expires
	return nil
}

func canonicalTarget(raw string, _ bool) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" ||
		u.Host == "" || u.RawQuery != "" {
		return "", ErrProofInvalid
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", ErrProofInvalid
	}
	port := u.Port()
	switch {
	case port != "" && port != "443":
		host = net.JoinHostPort(host, port)
	case strings.Contains(host, ":"):
		host = "[" + host + "]"
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	return fmt.Sprintf("https://%s%s", host, path), nil
}

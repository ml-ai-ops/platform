package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestVerifierAndRBAC(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	exponent := big.NewInt(int64(privateKey.PublicKey.E)).Bytes()
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "key-1", "kty": "RSA",
			"n": base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(exponent),
		}}})
	}))
	defer jwks.Close()
	now := time.Now()
	claims := Claims{Email: "engineer@example.com", Tenant: "team-a", Roles: []string{"engineer"}, RegisteredClaims: jwt.RegisteredClaims{
		Subject: "user-1", Issuer: "https://issuer.example", Audience: []string{"mlaiops"},
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)), IssuedAt: jwt.NewNumericDate(now),
	}}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "key-1"
	raw, _ := token.SignedString(privateKey)
	verifier := New(Config{Issuer: "https://issuer.example", Audience: "mlaiops", JWKSURL: jwks.URL, Tenant: "team-a"})
	principal, err := verifier.Verify(t.Context(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if principal.Email != claims.Email || !Allowed(principal, http.MethodPost, "/api/v1/models") {
		t.Fatalf("unexpected principal: %#v", principal)
	}
	if Allowed(principal, http.MethodPost, "/api/v1/connections") {
		t.Fatal("engineer must not manage connections")
	}
}

func TestTenantMismatchFails(t *testing.T) {
	principal := Principal{Roles: []string{"viewer"}}
	if Allowed(principal, http.MethodPost, "/api/v1/projects") {
		t.Fatal("viewer must be read-only")
	}
}

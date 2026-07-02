package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Principal struct {
	Subject    string
	Email      string
	Tenant     string
	Roles      []string
	Namespaces []string
}

type contextKey struct{}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	value, ok := ctx.Value(contextKey{}).(Principal)
	return value, ok
}

type Claims struct {
	Email      string   `json:"email"`
	Tenant     string   `json:"tenant"`
	Roles      []string `json:"roles"`
	Namespaces []string `json:"namespaces"`
	jwt.RegisteredClaims
}

type Config struct {
	Issuer   string
	Audience string
	JWKSURL  string
	Tenant   string
}

type Verifier struct {
	config  Config
	client  *http.Client
	mu      sync.RWMutex
	keys    map[string]*rsa.PublicKey
	expires time.Time
}

func New(config Config) *Verifier {
	return &Verifier{config: config, client: &http.Client{Timeout: 5 * time.Second}, keys: make(map[string]*rsa.PublicKey)}
}

func (v *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/health" || r.URL.Path == "/api/openapi.json" {
			next.ServeHTTP(w, r)
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if token == "" {
			deny(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		principal, err := v.Verify(r.Context(), token)
		if err != nil {
			deny(w, http.StatusUnauthorized, "invalid bearer token")
			return
		}
		if !Allowed(principal, r.Method, r.URL.Path) {
			deny(w, http.StatusForbidden, "insufficient role")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, principal)))
	})
}

func (v *Verifier) Verify(ctx context.Context, raw string) (Principal, error) {
	claims := &Claims{}
	options := []jwt.ParserOption{jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithIssuer(v.config.Issuer)}
	if v.config.Audience != "" {
		options = append(options, jwt.WithAudience(v.config.Audience))
	}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, errors.New("only RS256 is accepted")
		}
		kid, _ := token.Header["kid"].(string)
		return v.key(ctx, kid)
	}, options...)
	if err != nil || !token.Valid {
		return Principal{}, errors.New("token validation failed")
	}
	if v.config.Tenant != "" && claims.Tenant != v.config.Tenant {
		return Principal{}, errors.New("tenant mismatch")
	}
	return Principal{Subject: claims.Subject, Email: claims.Email, Tenant: claims.Tenant, Roles: claims.Roles, Namespaces: claims.Namespaces}, nil
}

func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	key, valid := v.keys[kid], time.Now().Before(v.expires)
	v.mu.RUnlock()
	if key != nil && valid {
		return key, nil
	}
	if err := v.refresh(ctx); err != nil {
		return nil, err
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	key = v.keys[kid]
	if key == nil {
		return nil, errors.New("signing key not found")
	}
	return key, nil
}

func (v *Verifier) refresh(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, v.config.JWKSURL, nil)
	if err != nil {
		return err
	}
	response, err := v.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS returned %d", response.StatusCode)
	}
	var document struct {
		Keys []struct {
			KID string `json:"kid"`
			KTY string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(response.Body).Decode(&document); err != nil {
		return err
	}
	keys := make(map[string]*rsa.PublicKey)
	for _, item := range document.Keys {
		if item.KTY != "RSA" {
			continue
		}
		modulus, err := base64.RawURLEncoding.DecodeString(item.N)
		if err != nil {
			continue
		}
		exponent, err := base64.RawURLEncoding.DecodeString(item.E)
		if err != nil {
			continue
		}
		e := 0
		for _, value := range exponent {
			e = e<<8 + int(value)
		}
		keys[item.KID] = &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: e}
	}
	if len(keys) == 0 {
		return errors.New("JWKS contained no usable RSA keys")
	}
	v.mu.Lock()
	v.keys, v.expires = keys, time.Now().Add(10*time.Minute)
	v.mu.Unlock()
	return nil
}

func Allowed(principal Principal, method, path string) bool {
	for _, role := range principal.Roles {
		if role == "admin" || role == "operator" {
			return true
		}
		if role == "viewer" && method == http.MethodGet {
			return true
		}
		if role == "engineer" {
			if method == http.MethodGet {
				return true
			}
			for _, prefix := range []string{"/api/v1/projects", "/api/v1/pipelines", "/api/v1/models", "/api/v1/agents", "/api/v1/tools", "/api/v1/features", "/api/v1/traces"} {
				if strings.HasPrefix(path, prefix) {
					return true
				}
			}
		}
	}
	return false
}

func deny(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": http.StatusText(status), "message": message})
}

package auth

import (
	"context"
	"crypto/rsa"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Platform roles, most to least privileged. "service" is the machine identity
// used by in-platform services (agent runtime, pipeline flows, materializer,
// realtime processor) reporting back through the internal token.
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleEngineer = "engineer"
	RoleViewer   = "viewer"
	RoleService  = "service"
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

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, principal)
}

type Claims struct {
	Email      string   `json:"email"`
	Tenant     string   `json:"tenant"`
	Roles      []string `json:"roles"`
	Groups     []string `json:"groups"`
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

// Middleware authenticates requests: OIDC bearer tokens for people, the
// internal platform token for in-cluster services. Authorization (role
// checks) happens in RBAC, which runs on every deployment profile.
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
		if service, ok := servicePrincipal(token); ok {
			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), service)))
			return
		}
		principal, err := v.Verify(r.Context(), token)
		if err != nil {
			deny(w, http.StatusUnauthorized, "invalid bearer token")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), principal)))
	})
}

// servicePrincipal matches the shared internal token that platform services
// use to report state back to the control plane.
func servicePrincipal(bearer string) (Principal, bool) {
	token := os.Getenv("MLAIOPS_INTERNAL_TOKEN")
	if token == "" || subtle.ConstantTimeCompare([]byte(bearer), []byte(token)) != 1 {
		return Principal{}, false
	}
	return Principal{Subject: "internal-service", Roles: []string{RoleService}}, true
}

// RBAC authorizes every API request. Identity comes from the OIDC middleware
// when it ran; otherwise the internal service token or the local development
// principal (role from MLAIOPS_LOCAL_ROLE, admin by default) applies. The
// resolved principal is placed on the context so handlers attribute actions.
func RBAC(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if !strings.HasPrefix(path, "/api/") || path == "/api/v1/health" || path == "/api/openapi.json" {
			next.ServeHTTP(w, r)
			return
		}
		principal, ok := PrincipalFrom(r.Context())
		if !ok {
			bearer := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
			if service, matched := servicePrincipal(bearer); matched {
				principal = service
			} else {
				role := strings.TrimSpace(os.Getenv("MLAIOPS_LOCAL_ROLE"))
				if role == "" {
					role = RoleAdmin
				}
				principal = Principal{Subject: "local-dev", Roles: []string{role}}
			}
			r = r.WithContext(WithPrincipal(r.Context(), principal))
		}
		if !Allowed(principal, r.Method, path) {
			deny(w, http.StatusForbidden, "insufficient role")
			return
		}
		next.ServeHTTP(w, r)
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
	// Dex and most IdPs carry role membership in the "groups" claim; a
	// dedicated "roles" claim wins when both are present.
	roles := claims.Roles
	if len(roles) == 0 {
		roles = claims.Groups
	}
	return Principal{Subject: claims.Subject, Email: claims.Email, Tenant: claims.Tenant, Roles: roles, Namespaces: claims.Namespaces}, nil
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

// engineerWrite lists what the engineer role may mutate: the ML lifecycle,
// but not platform configuration (connections stay admin/operator-only).
var engineerWrite = []string{
	"/api/v1/projects", "/api/v1/pipelines", "/api/v1/models", "/api/v1/agents",
	"/api/v1/tools", "/api/v1/features", "/api/v1/traces", "/api/v1/functions",
	"/api/v1/realtime",
}

// serviceWrite lists the reporting paths in-platform services need: session
// traces, pipeline step transitions, materialization reports and realtime
// stream statistics.
var serviceWrite = []string{
	"/api/v1/traces", "/api/v1/pipelines/runs", "/api/v1/features", "/api/v1/realtime",
}

func Allowed(principal Principal, method, path string) bool {
	read := method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
	for _, role := range principal.Roles {
		switch role {
		case RoleAdmin, RoleOperator:
			return true
		case RoleViewer:
			if read {
				return true
			}
		case RoleEngineer:
			if read || hasAnyPrefix(path, engineerWrite) {
				return true
			}
		case RoleService:
			if read || hasAnyPrefix(path, serviceWrite) {
				return true
			}
		}
	}
	return false
}

func hasAnyPrefix(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// Permissions summarizes what a principal may do, keyed the way the console
// gates its controls. Derived from Allowed so the UI can never drift from
// what the API actually enforces.
func Permissions(principal Principal) map[string]bool {
	return map[string]bool{
		"projects_write":    Allowed(principal, http.MethodPost, "/api/v1/projects"),
		"pipelines_write":   Allowed(principal, http.MethodPost, "/api/v1/pipelines/submit"),
		"models_write":      Allowed(principal, http.MethodPost, "/api/v1/models"),
		"agents_write":      Allowed(principal, http.MethodPost, "/api/v1/agents"),
		"tools_write":       Allowed(principal, http.MethodPost, "/api/v1/tools"),
		"features_write":    Allowed(principal, http.MethodPost, "/api/v1/features"),
		"functions_write":   Allowed(principal, http.MethodPost, "/api/v1/functions"),
		"connections_write": Allowed(principal, http.MethodPost, "/api/v1/connections"),
	}
}

func deny(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": http.StatusText(status), "message": message})
}

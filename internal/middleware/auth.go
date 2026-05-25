package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserClaimsKey contextKey = "user_claims"

// Auth implements JWT and API key authentication middleware.
type Auth struct {
	enabled      bool
	jwtSecret    []byte
	apiKeys      map[string]bool
	excludePaths map[string]bool
}

// NewAuth creates a new authentication middleware.
func NewAuth(enabled bool, jwtSecret string, apiKeys []string, excludePaths []string) *Auth {
	keyMap := make(map[string]bool)
	for _, key := range apiKeys {
		keyMap[key] = true
	}

	pathMap := make(map[string]bool)
	for _, path := range excludePaths {
		pathMap[path] = true
	}

	return &Auth{
		enabled:      enabled,
		jwtSecret:    []byte(jwtSecret),
		apiKeys:      keyMap,
		excludePaths: pathMap,
	}
}

// Middleware returns an HTTP middleware that enforces authentication.
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Check excluded paths
		if a.excludePaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		// Try API key first
		if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
			if a.apiKeys[apiKey] {
				next.ServeHTTP(w, r)
				return
			}
			writeAuthError(w, "invalid API key")
			return
		}

		// Try JWT Bearer token
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeAuthError(w, "missing authentication credentials")
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeAuthError(w, "invalid authorization header format")
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims := jwt.MapClaims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return a.jwtSecret, nil
		})

		if err != nil || !token.Valid {
			writeAuthError(w, "invalid or expired token")
			return
		}

		// Add claims to request context
		ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   "unauthorized",
		"message": message,
	})
}

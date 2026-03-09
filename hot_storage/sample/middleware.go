package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

func corsMiddleware(next http.Handler) http.Handler {
	allowedOrigins := getAllowedOrigins()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if isOriginAllowed(origin, allowedOrigins) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-auth-provider, x-request-id, x-player-token, x-cookie-field")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Vary", "Origin")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getAllowedOrigins() []string {
	originsEnv := os.Getenv("ALLOWED_ORIGINS")
	if originsEnv == "" {
		return []string{"http://localhost:7050", "http://localhost:7051"}
	}
	return strings.Split(originsEnv, ",")
}

func isOriginAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return false
	}
	for _, a := range allowed {
		if strings.TrimSpace(a) == origin {
			return true
		}
	}
	return false
}

// TODO(security): Add per-user rate limiting middleware before production deployment.
// Recommended: golang.org/x/time/rate with a TokenBucket per userId (e.g., 10 req/sec burst 20).
// Critical endpoints: /v2/devices/create, /v1/devices/register
// Without rate limiting, authenticated users can create unbounded devices/accounts.
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userId, authProvider, err := validateAuth(r)
		if err != nil || userId == "" {
			unauthorized(w)
			return
		}
		slog.Debug("authenticated request", slog.String("userId", userId))
		ctx := context.WithValue(r.Context(), fieldUserId, userId)
		ctx = context.WithValue(ctx, fieldAuthProvider, authProvider)
		authenticatedRequest := r.WithContext(ctx)
		next.ServeHTTP(w, authenticatedRequest)
	})
}

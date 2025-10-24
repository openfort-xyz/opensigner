package main

import (
	"context"
	"fmt"
	"net/http"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-request-id, x-player-token")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userId, authProvider, err := validateAuth(r)
		if err != nil || userId == "" {
			unauthorized(w)
			return
		}
		fmt.Printf("\nUser ID: %s\n\n", userId)
		ctx := context.WithValue(r.Context(), fieldUserId, userId)
		ctx = context.WithValue(ctx, fieldAuthProvider, authProvider)
		authenticatedRequest := r.WithContext(ctx)
		next.ServeHTTP(w, authenticatedRequest)
	})
}

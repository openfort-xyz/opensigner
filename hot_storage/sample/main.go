package main

import (
	"fmt"
	"os"

	"log/slog"
)

func main() {
	if authServerURL == "" {
		fmt.Println("AUTH_SERVER_URL environment variable is not set")
		os.Exit(1)
	}

	if err := initEncryptionKey(); err != nil {
		slog.Error(fmt.Sprintf("Failed to initialize encryption: %v", err))
		os.Exit(1)
	}

	if err := initSmsProvider(); err != nil {
		slog.Error(fmt.Sprintf("Failed to initialize SMS provider: %v", err))
		os.Exit(1)
	}

	if err := initWebauthn(); err != nil {
		slog.Error(fmt.Sprintf("Failed to initialize WebAuthn: %v", err))
		os.Exit(1)
	}

	err := initDB()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to initialize DB: %v", err))
		os.Exit(1)
	}

	slog.Info("DB initialized")
	host := os.Getenv("HOST")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprintf("%s:%s", host, port)
	slog.Info(fmt.Sprintf("Server running on %s", addr))
	listenAndServe(addr)
}

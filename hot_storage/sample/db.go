package main

import (
	"fmt"
	"log/slog"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

// Connects to Postgres using environment variables DB_HOST, DB_PORT, DB_NAME.
func initDB() error {
	slog.Info("Initializing DB")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	name := os.Getenv("DB_NAME")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASS")

	if host == "" || port == "" || name == "" || user == "" {
		return fmt.Errorf("DB_HOST, DB_PORT, DB_NAME, and DB_USER environment variables must be set")
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, name)
	newDB, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		return err
	}
	if err := newDB.AutoMigrate(&Device{}); err != nil {
		return err
	}
	if err := newDB.AutoMigrate(&Signer{}); err != nil {
		return err
	}
	if err := newDB.AutoMigrate(&Account{}); err != nil {
		return err
	}

	db = newDB
	slog.Info("DB initialized")
	return nil
}

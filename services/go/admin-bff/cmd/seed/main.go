package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:changeme@localhost:5433/opus_casino?sslmode=disable"
	}
	email := os.Getenv("ADMIN_EMAIL")
	if email == "" {
		email = "admin@example.com"
	}
	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		password = "admin123"
	}

	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(context.Background(), `
		INSERT INTO admin_users (email, password_hash, first_name, last_name, role, status, permissions, created_at, updated_at)
		VALUES ($1, $2, 'System', 'Admin', 'super_admin', 'active', ARRAY['*'], NOW(), NOW())
		ON CONFLICT (email) DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			status = 'active',
			updated_at = NOW()
	`, email, string(hash))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Admin user created/updated: %s\n", email)
}

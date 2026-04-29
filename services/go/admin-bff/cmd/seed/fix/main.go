package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:changeme@localhost:5433/opus_casino?sslmode=disable"
	}
	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer db.Close()

	email := "admin@example.com"
	password := "admin123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	_, err = db.Exec(context.Background(), "UPDATE admin_users SET password_hash = $1 WHERE email = $2", string(hash), email)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("Password updated for", email)
}

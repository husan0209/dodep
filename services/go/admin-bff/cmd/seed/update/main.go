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

	fmt.Println("Generated hash:", string(hash))

	var id int64
	err = db.QueryRow(context.Background(),
		"UPDATE admin_users SET password_hash = $1 WHERE email = $2 RETURNING id",
		string(hash), email,
	).Scan(&id)
	if err != nil {
		fmt.Println("Update error:", err)
		os.Exit(1)
	}
	fmt.Println("Updated user id:", id)

	// Verify immediately
	var stored string
	err = db.QueryRow(context.Background(),
		"SELECT password_hash FROM admin_users WHERE email = $1", email,
	).Scan(&stored)
	if err != nil {
		fmt.Println("Select error:", err)
		os.Exit(1)
	}
	fmt.Println("Stored hash:", stored)
	fmt.Println("Match:", bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil)
}

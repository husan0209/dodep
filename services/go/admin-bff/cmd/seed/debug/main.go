package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/opus-casino/admin-bff/internal/models"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:changeme@localhost:5433/opus_casino?sslmode=disable"
	}
	dbPool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		fmt.Println("pool:", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("gorm:", err)
		os.Exit(1)
	}

	var admin models.AdminUser
	if err := db.Where("email = ?", "admin@example.com").First(&admin).Error; err != nil {
		fmt.Println("find:", err)
		os.Exit(1)
	}

	fmt.Println("ID:", admin.ID)
	fmt.Println("Email:", admin.Email)
	fmt.Println("Status:", admin.Status)
	fmt.Println("Role:", admin.Role)
	fmt.Println("PasswordHash:", admin.PasswordHash)

	password := "admin123"
	err = bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password))
	fmt.Println("Match:", err == nil)
	if err != nil {
		fmt.Println("Error:", err)
	}
}

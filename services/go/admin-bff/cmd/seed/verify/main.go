package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	hash := "$2a$10$paPXr772dHE.7R191kD4..R8t79B7ppa52QCONlH1Qr6wYnYKYxu"
	password := "admin123"
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	fmt.Println("Match:", err == nil)
	if err != nil {
		fmt.Println("Error:", err)
	}
}

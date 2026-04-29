package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	hash := "$2a$10$Iqie7nuIZ11bw7K7CbTPaO0XWKdL2hkuRSTZhR0pcq1Y9dj6WgFH"
	password := "admin123"
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	fmt.Println("Match:", err == nil)
	if err != nil {
		fmt.Println("Error:", err)
	}
}

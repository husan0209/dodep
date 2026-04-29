//go:build ignore

package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
)

func main() {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	fmt.Println("Public key (base64):")
	fmt.Println(base64.StdEncoding.EncodeToString(pub))
	fmt.Println("\nPrivate key (base64):")
	fmt.Println(base64.StdEncoding.EncodeToString(priv))
}

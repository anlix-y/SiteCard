package main

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	pass := "qwe" // выберите надёжный пароль
	hash, _ := bcrypt.GenerateFromPassword(
		[]byte(pass),
		bcrypt.DefaultCost,
	)
	fmt.Printf("bcrypt hash: %s\n", hash)
}

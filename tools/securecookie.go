package main

import (
	"encoding/base64"
	"fmt"

	"github.com/gorilla/securecookie"
)

func main() {
	key := securecookie.GenerateRandomKey(32)
	base64key := base64.StdEncoding.EncodeToString(key)
	fmt.Println(base64key)
}

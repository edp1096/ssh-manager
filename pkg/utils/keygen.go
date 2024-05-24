package utils

import (
	"crypto/sha256"

	"golang.org/x/crypto/pbkdf2"
)

func GenerateKey(password string) (key []byte, err error) {
	salt := sha256.Sum256([]byte(password))
	key = pbkdf2.Key([]byte(password), salt[:], 10000, 32, sha256.New)

	return key, nil
}

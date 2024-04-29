// TODO: try windows signature - https://github.com/mwiater/golangsignedbins
package main // import "keygen"

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var bitSize = flag.Int("s", 2048, "Max key size - 521/1024/2048/4096")

func parseFlag() {
	fpath := os.Args[0]
	os.Args[0] = filepath.Base(os.Args[0])

	flag.Parse()

	os.Args[0] = fpath
}

func savePrivateKey() (privateKey *rsa.PrivateKey, err error) {
	privateKey, err = rsa.GenerateKey(rand.Reader, *bitSize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %s", err)
	}

	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	privateKeyFile, err := os.Create("private_key.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to create private key file: %s", err)
	}
	defer privateKeyFile.Close()

	if err = pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return nil, fmt.Errorf("failed to write private key to file: %s", err)
	}

	return privateKey, nil
}

func savePublicKey(privateKey *rsa.PrivateKey) (err error) {
	publicKey := privateKey.PublicKey

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&publicKey)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %s", err)
	}

	publicKeyPEM := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	}

	publicKeyFile, err := os.Create("public_key.pem")
	if err != nil {
		return fmt.Errorf("failed to create public key file: %s", err)
	}
	defer publicKeyFile.Close()

	if err := pem.Encode(publicKeyFile, publicKeyPEM); err != nil {
		return fmt.Errorf("failed to write public key to file: %s", err)
	}

	return nil
}

func main() {
	parseFlag()

	privateKey, err := savePrivateKey()
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	fmt.Println("Private key generated and saved to private_key.pem")

	err = savePublicKey(privateKey)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	fmt.Println("Public key generated and saved to public_key.pem")
}

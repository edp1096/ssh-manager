package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/crypto/pbkdf2"
)

func exitProcess() {
	err := cmdBrowser.Process.Kill()
	if err != nil {
		if runtime.GOOS == "windows" {
			exec.Command("taskkill", "/fi", "windowtitle eq "+browserWindowTitle).Run()
		} else {
			exec.Command("pkill", "-f", browserWindowTitle).Run()
		}
	}

	time.Sleep(100 * time.Millisecond)

	dataPath := filepath.FromSlash(binaryPath + "/browser_data")
	os.RemoveAll(dataPath)

	os.Exit(0)
}

func checkProcessExists(name string) (bool, error) {
	var err error
	var result bool = false

	procs, err := process.Processes()
	if err != nil {
		return result, err
	}
	for _, p := range procs {
		n, err := p.Name()
		if err != nil {
			continue
		}

		// TODO: catch "tmux: server <nil>"
		if n == name {
			result = true
			break
		}
	}
	return result, nil
}

func getBinaryPath() (binPath, binName string, err error) {
	fullPath, err := os.Executable()
	if err != nil {
		return "", "", err
	}

	binPath = filepath.Dir(fullPath)
	binName = filepath.Base(fullPath)

	return binPath, binName, err
}

// func getCurrentPath() (cwd string, err error) {
// 	cwd, err = os.Getwd()
// 	if err != nil {
// 		return "", err
// 	}
// 	return cwd, err
// }

func generateKey(password string) (key []byte, err error) {
	// salt := make([]byte, 16)
	// _, err = rand.Read(salt)
	// if err != nil {
	// 	fmt.Println("error generating salt:", err)
	// 	return key, err
	// }

	salt := sha256.Sum256([]byte(password))
	// fmt.Println("salt bytes:", salt)

	key = pbkdf2.Key([]byte(password), salt[:], 10000, 32, sha256.New)

	return key, nil
}

func saveHostData(fileName string, key []byte, data interface{}) error {
	var buf bytes.Buffer
	iv := make([]byte, aes.BlockSize)

	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(&buf)
	err = encoder.Encode(data)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	_, err = io.ReadFull(rand.Reader, iv)
	if err != nil {
		return err
	}

	_, err = file.Write(iv)
	if err != nil {
		return err
	}

	stream := cipher.NewCFBEncrypter(block, iv)

	writer := &cipher.StreamWriter{S: stream, W: file}
	_, err = writer.Write(buf.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func loadHostData(fileName string, key []byte, decryptedData interface{}) error {
	encryptedData := make([]byte, 4096)

	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Read(encryptedData)
	if err != nil && err != io.EOF {
		return err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	iv := encryptedData[:aes.BlockSize]
	stream := cipher.NewCFBDecrypter(block, iv)

	encryptedData = encryptedData[aes.BlockSize:]

	reader := &cipher.StreamReader{S: stream, R: bytes.NewReader(encryptedData)}
	decoder := gob.NewDecoder(reader)
	err = decoder.Decode(decryptedData)
	if err != nil {
		return err
	}

	return nil
}

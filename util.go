package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/crypto/pbkdf2"
)

func RenameFolders(pattern, newPrefix string) error {
	folders, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, folder := range folders {
		newName := newPrefix
		err := os.Rename(folder, newName)
		if err != nil {
			return err
		}
		fmt.Printf("Renamed %s to %s\n", folder, newName)
	}

	return nil
}

func exitProcess() {
	// Wait for browser refresh checking
	time.Sleep(500 * time.Millisecond)
	if len(WebSocketConns) > 0 {
		return
	}

	CmdBrowser.Process.Kill()

	time.Sleep(100 * time.Millisecond)

	// Remove browser_data
	dataPath := filepath.FromSlash(BinaryPath + "/browser_data")
	os.RemoveAll(dataPath)

	os.Exit(0)
}

func CheckFileExitsInEnvPath(fname string) (result bool) {
	paths := strings.Split(os.Getenv("PATH"), ":")

	result = false
	for _, p := range paths {
		if _, err := os.Stat(p + "/" + fname); err == nil {
			// fmt.Printf("%s exists in %s\n", fname, p)
			result = true
			break
		}
	}

	return result
}

func CheckProcessExists(name string) (bool, error) {
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

func GetBinaryPath() (binPath, binName string, err error) {
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

func GenerateKey(password string) (key []byte, err error) {
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

func SaveHostData(fileName string, key []byte, data interface{}) error {
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

func LoadHostData(fileName string, key []byte, decryptedData interface{}) error {
	// encryptedData := make([]byte, 4096)

	file, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("loadHostData/open: %s", err)
	}
	defer file.Close()

	// _, err = file.Read(encryptedData)
	// if err != nil && err != io.EOF {
	// 	return fmt.Errorf("loadHostData/read: %s", err)
	// }
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("loadHostData/Stat: %s", err)
	}

	encryptedData := make([]byte, fileInfo.Size())
	_, err = io.ReadFull(file, encryptedData)
	if err != nil {
		return fmt.Errorf("loadHostData/ReadFull: %s", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("loadHostData/NewCipher: %s", err)
	}
	iv := encryptedData[:aes.BlockSize]
	stream := cipher.NewCFBDecrypter(block, iv)

	encryptedData = encryptedData[aes.BlockSize:]

	reader := cipher.StreamReader{S: stream, R: bytes.NewReader(encryptedData)}
	decoder := gob.NewDecoder(&reader)
	err = decoder.Decode(decryptedData)
	if err != nil {
		return fmt.Errorf("loadHostData/Decode: %s", err)
	}

	return nil
}

func RemoveIndexItem[T any](s []T, index int) []T {
	if index < 0 || index >= len(s) {
		return s
	}

	copy(s[index:], s[index+1:])

	var zero T
	s[len(s)-1] = zero
	s = s[:len(s)-1]

	return s
}

func FindPasswordByUUID(categories []HostCategory, uuid string) (password string, found bool) {
	password = ""
	found = false

	for _, c := range categories {
		for _, h := range c.Hosts {
			if h.UniqueID == uuid {
				password = h.Password
				found = true
				return password, found
			}
		}
	}

	return password, found
}

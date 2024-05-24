package host

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"encoding/gob"
	"fmt"
	"io"
	"os"
)

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

	_, err = io.ReadFull(crand.Reader, iv)
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
	file, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("loadHostData/open: %s", err)
	}
	defer file.Close()

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

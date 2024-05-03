package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func unzip(zipFile []byte, targetFolder string) error {
	if _, err := os.Stat(targetFolder); os.IsNotExist(err) {
		os.Mkdir(targetFolder, os.ModePerm)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipFile), int64(len(zipFile)))
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}

	fileList := make(map[string]bool)

	fmt.Println("Extracting from ZIP:")
	for _, file := range zipReader.File {
		filePath := filepath.Join(targetFolder, file.Name)

		// case directory
		if file.FileInfo().IsDir() {
			fileList[filePath] = true
			os.MkdirAll(filePath, os.ModePerm)
			continue
		}

		// case file
		dir := filepath.Dir(filePath)
		if _, ok := fileList[dir]; !ok {
			fileList[dir] = true
			os.MkdirAll(dir, os.ModePerm)
		}

		extractedFile, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open file inside zip: %w", err)
		}
		defer extractedFile.Close()

		outFile, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer outFile.Close()

		_, err = io.Copy(outFile, extractedFile)
		if err != nil {
			return fmt.Errorf("failed to extract file contents: %w", err)
		}

		fmt.Println(filePath)
	}

	return nil
}

func untar(tarFile []byte, targetFolder string) error {
	if _, err := os.Stat(targetFolder); os.IsNotExist(err) {
		os.Mkdir(targetFolder, os.ModePerm)
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(tarFile))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	fileList := make(map[string]bool)

	fmt.Println("Extracting from TAR.GZ:")
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		filePath := filepath.Join(targetFolder, header.Name)

		// case directory
		if header.Typeflag == tar.TypeDir {
			fileList[filePath] = true
			os.MkdirAll(filePath, os.ModePerm)
			continue
		}

		// case file
		dir := filepath.Dir(filePath)
		if _, ok := fileList[dir]; !ok {
			fileList[dir] = true
			os.MkdirAll(dir, os.ModePerm)
		}

		outFile, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, tarReader); err != nil {
			return fmt.Errorf("failed to extract file contents: %w", err)
		}

		fmt.Println(filePath)
	}

	return nil
}

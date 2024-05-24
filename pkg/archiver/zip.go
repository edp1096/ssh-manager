package archiver

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Zip struct {
	FileName   string
	TargetPath string
	FSdata     FileSystem
}

func (z Zip) UnArchive() (err error) {
	embedZipData, err := z.FSdata.ReadFile(z.FileName)
	if err != nil {
		return err
	}

	err = UnZip(embedZipData, z.TargetPath)
	if err != nil {
		return err
	}

	return nil
}

func UnZip(zipFile []byte, targetFolder string) error {
	if _, err := os.Stat(targetFolder); os.IsNotExist(err) {
		os.Mkdir(targetFolder, os.ModePerm)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipFile), int64(len(zipFile)))
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}

	fileList := make(map[string]bool)

	// fmt.Println("Extracting from ZIP:")
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

		// fmt.Println(filePath)
	}

	return nil
}

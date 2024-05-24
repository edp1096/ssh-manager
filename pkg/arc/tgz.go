package arc

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Tgz struct {
	FileName   string
	TargetPath string
	FSdata     FileSystem
}

func (z Tgz) UnArchive() (err error) {
	embedTarData, err := z.FSdata.ReadFile(z.FileName)
	if err != nil {
		return err
	}

	err = UnTar(embedTarData, z.TargetPath)
	if err != nil {
		return err
	}

	return nil
}

func UnTar(tarFile []byte, targetFolder string) error {
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

	// fmt.Println("Extracting from TAR.GZ:")
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

		// fmt.Println(filePath)
	}

	return nil
}

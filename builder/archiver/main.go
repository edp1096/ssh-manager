package main // import "archiver"

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	src := "./bin"
	dest := "./dist"
	pkgName := "my-ssh-manager"

	if _, err := os.Stat(dest); os.IsNotExist(err) {
		err := os.Mkdir(dest, 0755)
		if err != nil {
			fmt.Printf("error creating directory: %v\n", err)
			return
		}
		fmt.Println("directory created:", dest)
	} else if err != nil {
		fmt.Printf("error checking directory existence: %v\n", err)
		return
	}

	currentOS := runtime.GOOS
	currentARCH := runtime.GOARCH

	defaultOsArch := fmt.Sprintf("%s/%s", currentOS, currentARCH)

	osarch := flag.String("osarch", defaultOsArch, "OS/ARCH list")
	flag.Parse()

	osarchList := strings.Fields(*osarch)
	for _, oa := range osarchList {
		oas := strings.Split(oa, "/")
		osName := oas[0]
		archName := oas[1]
		extName := ""
		if osName == "windows" {
			extName = ".exe"
		}

		executablePaths := []string{
			filepath.Join(src, "ssh-client"),
			filepath.Join(src, pkgName),
		}
		trailName := "_" + osName + "_" + archName + extName

		zipFilePath := ""
		if osName == "windows" {
			zipFilePath = filepath.Join(dest, pkgName+"_"+osName+"_"+archName+".zip")
			if err := zipFiles(executablePaths, zipFilePath, extName, trailName); err != nil {
				fmt.Println("Error zip files:", err)
				return
			}
		} else {
			zipFilePath = filepath.Join(dest, pkgName+"_"+osName+"_"+archName+".tar.gz")
			if err := tarGzFiles(executablePaths, zipFilePath, extName, trailName); err != nil {
				fmt.Println("Error tar.gz files:", err)
				return
			}
		}

		fmt.Println("Created compressed file:", zipFilePath)
	}
}

func zipFiles(sourcePaths []string, target, extName, trailName string) error {
	zipFile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, sourcePath := range sourcePaths {
		if err := addFileToZip(zipWriter, sourcePath, extName, trailName); err != nil {
			return err
		}
	}

	return nil
}

func addFileToZip(zipWriter *zip.Writer, sourcePath, extName, trailName string) error {
	sourceFile, err := os.Open(sourcePath + trailName)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	zipEntry, err := zipWriter.Create(filepath.Base(sourcePath + extName))
	if err != nil {
		return err
	}

	_, err = io.Copy(zipEntry, sourceFile)
	if err != nil {
		return err
	}

	return nil
}

func tarGzFiles(sourcePaths []string, target, extName, trailName string) error {
	tarGzFile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer tarGzFile.Close()

	gzipWriter := gzip.NewWriter(tarGzFile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for _, sourcePath := range sourcePaths {
		if err := addFileToTar(tarWriter, sourcePath, extName, trailName); err != nil {
			return err
		}
	}

	return nil
}

func addFileToTar(tarWriter *tar.Writer, sourcePath, extName, trailName string) error {
	sourceFile, err := os.Open(sourcePath + trailName)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	fileInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	tarHeader, err := tar.FileInfoHeader(fileInfo, "")
	if err != nil {
		return err
	}
	tarHeader.Name = filepath.Base(sourcePath + extName)
	if err := tarWriter.WriteHeader(tarHeader); err != nil {
		return err
	}

	_, err = io.Copy(tarWriter, sourceFile)
	if err != nil {
		return err
	}

	return nil
}

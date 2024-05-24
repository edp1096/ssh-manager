package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func getGithubReleaseLatestUris(owner, repo string) ([]string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %v", err)
	}
	defer resp.Body.Close()

	var releaseInfo map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&releaseInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	var URLs []string
	assets := releaseInfo["assets"].([]interface{})
	for _, a := range assets {
		asset := a.(map[string]interface{})
		URLs = append(URLs, asset["browser_download_url"].(string))
	}
	return URLs, nil
}

func downloadFile(uri, dest, fname string) error {
	resp, err := http.Get(uri)
	if err != nil {
		return fmt.Errorf("failed to download file: %v", err)
	}
	defer resp.Body.Close()

	filePath := filepath.Join(dest, fname)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write to file: %v", err)
	}

	fmt.Printf("File downloaded to: %s\n", filePath)
	return nil
}

func DownloadWindowsTerminal() error {
	var err error

	owner := "microsoft"
	repo := "terminal"

	uris, err := getGithubReleaseLatestUris(owner, repo)
	if err != nil {
		return fmt.Errorf("failed to get latest release URLs: %v", err)
	}

	wtFname := "windows-terminal.zip"

	winArchs := map[string]string{"386": "x86", "amd64": "x64", "arm": "arm64"}
	arch := runtime.GOARCH
	// if !slices.Contains(archs, arch) {
	if _, ok := winArchs[arch]; !ok {
		return fmt.Errorf("architecture is not supported: %v", arch)
	}
	arch = winArchs[arch]

	isDownloaded := false
	for _, uri := range uris {
		if !strings.Contains(uri, arch) || !strings.Contains(uri, "zip") {
			continue
		}

		err = downloadFile(uri, ".", wtFname)
		if err != nil {
			return fmt.Errorf("failed to download file: %v", err)
		}
		isDownloaded = true
		break
	}

	if !isDownloaded {
		return fmt.Errorf("download nothing")
	}

	extractPath := "."
	fileZipData, err := os.ReadFile(wtFname)
	if err != nil {
		return fmt.Errorf("failed to read zip file: %s", err)
	}

	err = UnZip(fileZipData, extractPath)
	if err != nil {
		return fmt.Errorf("failed to unzip file: %s", err)
	}

	pattern := "terminal-*"
	newPrefix := "windows-terminal"
	err = RenameFolders(pattern, newPrefix)
	if err != nil {
		return fmt.Errorf("failed to rename folder: %s", err)
	}

	err = os.Remove(wtFname)
	if err != nil {
		fmt.Printf("failed to delete %s: %s", wtFname, err)
	}

	return nil
}

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

	// filename := filepath.Base(uri)
	filename := fname

	filePath := filepath.Join(dest, filename)
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

func downloadWindowsTerminal() error {
	var err error

	owner := "microsoft"
	repo := "terminal"

	uris, err := getGithubReleaseLatestUris(owner, repo)
	if err != nil {
		return fmt.Errorf("failed to get latest release URLs: %v", err)
	}

	for _, uri := range uris {
		if !strings.Contains(uri, "x64") || !strings.Contains(uri, "zip") {
			continue
		}

		err = downloadFile(uri, ".", "windows-terminal.zip")
		if err != nil {
			return fmt.Errorf("failed to download file: %v", err)
		}
	}

	return nil
}

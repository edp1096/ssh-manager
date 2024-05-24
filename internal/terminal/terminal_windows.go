//go:build windows
// +build windows

package terminal

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"ssh-manager/pkg/downloader"
	"ssh-manager/pkg/utils"
)

func downloadWindowsTerminalIfNotExist() {
	ShellRuntimePath = os.Getenv("LocalAppData") + "/Microsoft/WindowsApps/wt.exe"

	if _, err := os.Stat(ShellRuntimePath); os.IsNotExist(err) {
		cwd, _ := os.Getwd()
		ShellRuntimePath = cwd + "/windows-terminal/wt.exe"

		if _, err := os.Stat(ShellRuntimePath); os.IsNotExist(err) {
			err = downloader.DownloadWindowsTerminal()
			if err != nil {
				panic(fmt.Errorf("downloadWindowsTerminal: %s", err))
			}
		}
	}
}

func CheckTerminalExist() {
	downloadWindowsTerminalIfNotExist()
}

// func openTerminal(hostsFile string, categoryIndex int, hostIndex int, newWindow bool) (pid int, err error) {
func openTerminal(arg SshClientArgument) (pid int, err error) {
	WorkingDir, _, _ = utils.GetCWD()

	hostsFile := arg.HostsFile
	categoryIndex := arg.CategoryIndex
	hostIndex := arg.HostIndex
	newWindow := arg.NewWindow
	HostFileKEY := arg.HostFileKEY

	procName := "ssh-client.exe"
	termExists, err := utils.CheckProcessExists(procName)
	if err != nil {
		return -1, fmt.Errorf("proc error: %s", err)
	}

	splitParams := []string{"-w", "0", "sp"}
	if !termExists || newWindow {
		splitParams = []string{}
	}

	shParams := []string{}
	shParams = append(shParams, splitParams...)

	sshclientPath := filepath.FromSlash(WorkingDir + "/" + procName)

	hostsDataFile := ""
	if strings.HasPrefix(hostsFile, "./") {
		hostsDataFile = WorkingDir + "/" + filepath.Base(hostsFile)
	} else {
		hostsDataFile, err = filepath.Abs(hostsFile)
		if err != nil {
			return -1, fmt.Errorf("failed to get full path of hosts data: %s", err)
		}
	}
	hostsDataFile = filepath.FromSlash(hostsDataFile)

	hostFileKEYB64 := base64.URLEncoding.EncodeToString(HostFileKEY)

	sshParams := []string{sshclientPath, "-f", hostsDataFile, "-k", hostFileKEYB64, "-ci", strconv.Itoa(categoryIndex), "-hi", strconv.Itoa(hostIndex)}
	shParams = append(shParams, sshParams...)

	CmdTerminal = exec.Command(ShellRuntimePath, shParams...)
	err = CmdTerminal.Run()
	if err != nil {
		fmt.Println("Error opening terminal:", err)
		return
	}

	pid = CmdTerminal.Process.Pid
	return
}

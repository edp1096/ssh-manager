package main

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type SshArgument struct {
	HostsFile     string `json:"hosts-file"`
	CategoryIndex int    `json:"category-index"`
	HostIndex     int    `json:"host-index"`
}

func openWindowsTerminal(hostsFile string, categoryIndex int, hostIndex int, newWindow bool) (pid int, err error) {
	procName := "ssh-client.exe"
	termExists, err := checkProcessExists(procName)
	if err != nil {
		return -1, fmt.Errorf("proc error: %s", err)
	}

	splitParams := []string{"-w", "0", "sp"}
	if !termExists || newWindow {
		splitParams = []string{}
	}

	shParams := []string{}
	shParams = append(shParams, splitParams...)

	sshclientPath := filepath.FromSlash(binaryPath + "/" + procName)

	hostsDataFile := ""
	if strings.HasPrefix(hostsFile, "./") {
		hostsDataFile = binaryPath + "/" + filepath.Base(hostsFile)
	} else {
		hostsDataFile, err = filepath.Abs(hostsFile)
		if err != nil {
			return -1, fmt.Errorf("failed to get full path of hosts data: %s", err)
		}
	}
	hostsDataFile = filepath.FromSlash(hostsDataFile)

	hostFileKEYB64 := base64.URLEncoding.EncodeToString(hostFileKEY)

	sshParams := []string{sshclientPath, "-f", hostsDataFile, "-k", hostFileKEYB64, "-ci", strconv.Itoa(categoryIndex), "-hi", strconv.Itoa(hostIndex)}
	shParams = append(shParams, sshParams...)

	cmdTerminal = exec.Command(shellRuntimePath, shParams...)
	err = cmdTerminal.Run()
	if err != nil {
		fmt.Println("Error opening terminal:", err)
		return
	}

	pid = cmdTerminal.Process.Pid
	return
}

func openGnomeTerminal(hostsFile string, categoryIndex int, hostIndex int, newWindow bool) (pid int, err error) {
	shellRuntimePath = "tmux"

	procName := "ssh-client"
	termExists, err := checkProcessExists(procName)
	// termExists, err := checkProcessExists(shellRuntimePath)
	if err != nil {
		return -1, fmt.Errorf("error check process exist:%s", err)
	}

	if !termExists || newWindow {
		err = exec.Command("gnome-terminal", "--", "sh", "-c", shellRuntimePath+" -f ./tmux.conf; exec").Run()
		if err != nil {
			return -1, fmt.Errorf("error execute tmux:%s", err)
		}
	} else {
		err = exec.Command("tmux", "splitw", "-h").Run()
		if err != nil {
			return -1, fmt.Errorf("error execute tmux:%s", err)
		}
	}

	hostsDataFile := ""
	if strings.HasPrefix(hostsFile, "./") {
		hostsDataFile = binaryPath + "/" + filepath.Base(hostsFile)
	} else {
		hostsDataFile, err = filepath.Abs(hostsFile)
		if err != nil {
			return -1, fmt.Errorf("failed to get full path of hosts data: %s", err)
		}
	}
	hostsDataFile = filepath.FromSlash(hostsDataFile)

	sshclientPath := filepath.FromSlash(binaryPath + "/" + procName)

	hostFileKEYB64 := base64.URLEncoding.EncodeToString(hostFileKEY)
	sshParams := []string{sshclientPath + " -f " + hostsDataFile + " -k " + hostFileKEYB64 + " -ci " + strconv.Itoa(categoryIndex) + " -hi " + strconv.Itoa(hostIndex), "&&", "exit", "ENTER"}

	shParams := []string{"send"}
	shParams = append(shParams, sshParams...)

	cmdTerminal = exec.Command(shellRuntimePath, shParams...)
	err = cmdTerminal.Run()
	if err != nil {
		fmt.Println("Error opening terminal:", err)
		return
	}

	pid = cmdTerminal.Process.Pid

	return
}

func openSession(arg SshArgument, newWindow bool) {
	hostsFile := arg.HostsFile
	hostIndex := arg.HostIndex
	categoryIndex := arg.CategoryIndex

	if runtime.GOOS == "windows" {
		_, err := openWindowsTerminal(hostsFile, categoryIndex, hostIndex, newWindow)
		if err != nil {
			fmt.Println("Error open terminal:", err)
		}
	} else {
		_, err := openGnomeTerminal(hostsFile, categoryIndex, hostIndex, newWindow)
		if err != nil {
			fmt.Println("Error open terminal:", err)
		}
	}
}

package main

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type SshArgument struct {
	HostsFile string `json:"hosts-file"`
	Index     int    `json:"index"`
}

func openWindowsTerminal(hostsFile string, hostsIndex int) (pid int, err error) {
	procName := "ssh-client.exe"
	termExists, err := checkProcessExists(procName)
	if err != nil {
		log.Fatal("proc error:", err)
	}

	splitParams := []string{}
	if termExists {
		splitParams = []string{"-w", "0", "sp"}
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

	sshParams := []string{sshclientPath, "-f", hostsDataFile, "-i", strconv.Itoa(hostsIndex)}
	shParams = append(shParams, sshParams...)

	cmd = exec.Command(windowsTerminalPath, shParams...)
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error opening terminal:", err)
		return
	}

	pid = cmd.Process.Pid
	return
}

func openSession(arg SshArgument) {
	hostsFile := arg.HostsFile
	hostsIndex := arg.Index

	// pid, err := openWindowsTerminal(hostsFile, hostsIndex)
	_, err := openWindowsTerminal(hostsFile, hostsIndex)
	if err != nil {
		fmt.Println("Error open terminal:", err)
	}
	// fmt.Println("process id:", pid)
}

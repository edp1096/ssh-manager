package main

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"ssh-manager/pkg/utils"
)

type SshArgument struct {
	HostsFile     string `json:"hosts-file"`
	CategoryIndex int    `json:"category-index"`
	HostIndex     int    `json:"host-index"`
}

func openWindowsTerminal(hostsFile string, categoryIndex int, hostIndex int, newWindow bool) (pid int, err error) {
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

	sshclientPath := filepath.FromSlash(BinaryPath + "/" + procName)

	hostsDataFile := ""
	if strings.HasPrefix(hostsFile, "./") {
		hostsDataFile = BinaryPath + "/" + filepath.Base(hostsFile)
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

func openTerminal(hostsFile string, categoryIndex int, hostIndex int, newWindow bool) (pid int, err error) {
	ShellRuntimePath = "tmux"

	procName := "ssh-client"
	termExists, err := utils.CheckProcessExists(procName)
	if err != nil {
		return -1, fmt.Errorf("error check process exist:%s", err)
	}

	termBin := []string{"xterm", "-e", ShellRuntimePath}
	if utils.CheckFileExitsInEnvPath("konsole") {
		termBin = []string{"konsole", "-e", "sh", "-c", ShellRuntimePath}
	}
	if utils.CheckFileExitsInEnvPath("gnome-terminal") {
		termBin = []string{"gnome-terminal", "--", "sh", "-c", ShellRuntimePath}
	}

	if !termExists || newWindow {
		cmdTerm := exec.Command(termBin[0], termBin[1:]...)
		err := cmdTerm.Start()
		if err != nil {
			return -1, fmt.Errorf("error execute tmux:%s", err)
		}

		switch termBin[0] {
		case "gnome-terminal":
			cmdTerm.Wait()
		case "konsole":
			time.Sleep(4500 * time.Millisecond)
		default:
			time.Sleep(1000 * time.Millisecond)
		}
	} else {
		err = exec.Command(ShellRuntimePath, "splitw", "-h").Run()
		if err != nil {
			return -1, fmt.Errorf("error split tmux:%s", err)
		}
	}

	hostsDataFile := ""
	if strings.HasPrefix(hostsFile, "./") {
		hostsDataFile = BinaryPath + "/" + filepath.Base(hostsFile)
	} else {
		hostsDataFile, err = filepath.Abs(hostsFile)
		if err != nil {
			return -1, fmt.Errorf("failed to get full path of hosts data: %s", err)
		}
	}
	hostsDataFile = filepath.FromSlash(hostsDataFile)

	sshclientPath := filepath.FromSlash(BinaryPath + "/" + procName)
	hostFileKEYB64 := base64.URLEncoding.EncodeToString(HostFileKEY)
	sshParams := []string{sshclientPath + " -f " + hostsDataFile + " -k " + hostFileKEYB64 + " -ci " + strconv.Itoa(categoryIndex) + " -hi " + strconv.Itoa(hostIndex), "&&", "exit", "ENTER"}

	shParams := []string{"send"}
	shParams = append(shParams, sshParams...)

	CmdTerminal = exec.Command(ShellRuntimePath, shParams...)
	err = CmdTerminal.Run()
	if err != nil {
		return -1, fmt.Errorf("error opening terminal:%v", err)
	}

	pid = CmdTerminal.Process.Pid

	if !termExists || newWindow {
		CmdTerminal = exec.Command(ShellRuntimePath, []string{"set-option", "-g", "mouse", "on"}...)
		err = CmdTerminal.Run()
		if err != nil {
			return -1, fmt.Errorf("error opening terminal:%v", err)
		}
	}

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
		_, err := openTerminal(hostsFile, categoryIndex, hostIndex, newWindow)
		if err != nil {
			fmt.Println("Error open terminal:", err)
		}
	}
}

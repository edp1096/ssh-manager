//go:build !windows
// +build !windows

package main

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ssh-manager/pkg/utils"
)

func CheckTerminalExist() {}

// func openTerminal(hostsFile string, categoryIndex int, hostIndex int, newWindow bool) (pid int, err error) {
func openTerminal(arg SshArgument) (pid int, err error) {
	hostsFile := arg.HostsFile
	categoryIndex := arg.CategoryIndex
	hostIndex := arg.HostIndex
	newWindow := arg.NewWindow

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
		hostsDataFile = WorkingDir + "/" + filepath.Base(hostsFile)
	} else {
		hostsDataFile, err = filepath.Abs(hostsFile)
		if err != nil {
			return -1, fmt.Errorf("failed to get full path of hosts data: %s", err)
		}
	}
	hostsDataFile = filepath.FromSlash(hostsDataFile)

	sshclientPath := filepath.FromSlash(WorkingDir + "/" + procName)
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

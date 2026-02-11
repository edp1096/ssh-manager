//go:build !windows
// +build !windows

package terminal

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ssh-manager/pkg/utils"
)

func CheckTerminalExist() {
	ShellRuntimePath = "tmux"

	if !utils.CheckFileExitsInEnvPath(ShellRuntimePath) {
		fmt.Println("tmux not found in path environment.")
		fmt.Println("please install tmux.")
		os.Exit(1)
	}
}

// func openTerminal(hostsFile string, categoryIndex int, hostIndex int, newWindow bool) (pid int, err error) {
func openTerminal(arg SshClientArgument) (pid int, err error) {
	WorkingDir, _, _ = utils.GetCWD()

	hostsFile := arg.HostsFile
	categoryIndex := arg.CategoryIndex
	hostIndex := arg.HostIndex
	newWindow := arg.NewWindow
	HostFileKEY := arg.HostFileKEY

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
		opts := []string{"splitw"}
		if !arg.SplitVertical {
			opts = append(opts, "-h")
		}

		err = exec.Command(ShellRuntimePath, opts...).Run()
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
	// sshParams := []string{" clear && tmux clear-history; " + sshclientPath + " -f " + hostsDataFile + " -k " + hostFileKEYB64 + " -ci " + strconv.Itoa(categoryIndex) + " -hi " + strconv.Itoa(hostIndex), "&&", "exit", "ENTER"}
	// shParams := []string{"send-keys"}
	// shParams = append(shParams, sshParams...)

	mainCmd := fmt.Sprintf("clear && tmux clear-history; %s -f %s -k %s -ci %d -hi %d", sshclientPath, hostsDataFile, hostFileKEYB64, categoryIndex, hostIndex)
	cmdStr := fmt.Sprintf("  %s; history -d $(history 1 | awk '{print $1}'); exit", mainCmd)
	shParams := []string{"send-keys", cmdStr, "C-m"}

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

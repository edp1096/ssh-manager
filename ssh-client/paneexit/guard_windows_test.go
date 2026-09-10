package paneexit

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestExitGuardHelper(t *testing.T) {
	if os.Getenv("SSH_MANAGER_EXIT_HELPER") != "1" {
		return
	}
	dir := os.Getenv("SSH_MANAGER_EXIT_DIR")
	id := os.Getenv("SSH_MANAGER_EXIT_ID")
	guard, err := New(dir)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id), nil, 0600); err != nil {
		panic(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, "exit")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			os.Exit(2)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := guard.BeforeProcessExit(); err != nil {
		panic(err)
	}
	fmt.Println(time.Now().UnixMilli())
	os.Exit(0)
}

func TestExitGuardSerializesProcesses(t *testing.T) {
	dir := t.TempDir()
	var commands []*exec.Cmd
	var outputs [3]bytes.Buffer
	for i := range outputs {
		cmd := exec.Command(os.Args[0], "-test.run=^TestExitGuardHelper$")
		cmd.Env = append(os.Environ(), "SSH_MANAGER_EXIT_HELPER=1", "SSH_MANAGER_EXIT_DIR="+dir, "SSH_MANAGER_EXIT_ID="+strconv.Itoa(i))
		cmd.Stdout = &outputs[i]
		cmd.Stderr = &outputs[i]
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, cmd)
		t.Cleanup(func() {
			if cmd.ProcessState == nil {
				cmd.Process.Kill()
				cmd.Wait()
			}
		})
	}
	for i := range outputs {
		deadline := time.Now().Add(5 * time.Second)
		for {
			if _, err := os.Stat(filepath.Join(dir, strconv.Itoa(i))); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("helper did not initialize")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "exit"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	var times []int64
	for i, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err, outputs[i].String())
		}
		value, err := strconv.ParseInt(strings.TrimSpace(outputs[i].String()), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		times = append(times, value)
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	for i := 1; i < len(times); i++ {
		if times[i]-times[i-1] < 950 {
			t.Fatal("exits overlapped", times)
		}
	}
}

func TestEmptyExitGroup(t *testing.T) {
	guard, err := New("")
	if err != nil || guard.BeforeProcessExit() != nil {
		t.Fatal(err)
	}
}

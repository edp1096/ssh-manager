//go:build linux

package terminal

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"ssh-manager/internal/broadcast"
	"ssh-manager/internal/host"
	"ssh-manager/pkg/model"
)

// This test creates real Tilix windows/panes and authenticates real ssh-client
// processes against an isolated, recording SSH server. It executes no shell.
func TestLiveTilixBroadcast(t *testing.T) {
	exe := os.Getenv("SSH_MANAGER_BROADCAST_LIVE_BINARY")
	if exe == "" {
		t.Skip("requires explicit live Tilix opt-in")
	}
	if !filepath.IsAbs(exe) {
		t.Fatal("absolute binary path required")
	}
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(key)
	config := &ssh.ServerConfig{PasswordCallback: func(_ ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
		if string(p) != "test-only" {
			return nil, fmt.Errorf("password")
		}
		return nil, nil
	}}
	config.AddHostKey(signer)
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	var mu sync.Mutex
	received := map[string]string{}
	channels := map[string]ssh.Channel{}
	dimensions := map[string][2]uint32{}
	connections := []net.Conn{}
	defer func() {
		mu.Lock()
		defer mu.Unlock()
		for _, c := range connections {
			c.Close()
		}
	}()
	go func() {
		for {
			c, e := l.Accept()
			if e != nil {
				return
			}
			mu.Lock()
			connections = append(connections, c)
			mu.Unlock()
			go func() {
				defer c.Close()
				s, chans, reqs, e := ssh.NewServerConn(c, config)
				if e != nil {
					return
				}
				defer s.Close()
				go ssh.DiscardRequests(reqs)
				for next := range chans {
					ch, requests, e := next.Accept()
					if e != nil {
						return
					}
					mu.Lock()
					channels[s.User()] = ch
					mu.Unlock()
					go func() {
						for r := range requests {
							var width, height uint32
							if r.Type == "pty-req" {
								var p struct {
									Term                                   string
									Width, Height, PixelWidth, PixelHeight uint32
									Modes                                  string
								}
								if ssh.Unmarshal(r.Payload, &p) == nil {
									width, height = p.Width, p.Height
								}
							}
							if r.Type == "window-change" {
								var p struct{ Width, Height, PixelWidth, PixelHeight uint32 }
								if ssh.Unmarshal(r.Payload, &p) == nil {
									width, height = p.Width, p.Height
								}
							}
							if width > 0 && height > 0 {
								mu.Lock()
								dimensions[s.User()] = [2]uint32{width, height}
								mu.Unlock()
							}
							r.Reply(r.Type == "pty-req" || r.Type == "shell" || r.Type == "window-change", nil)
						}
					}()
					go func() {
						defer ch.Close()
						buf := make([]byte, 4096)
						for {
							n, e := ch.Read(buf)
							if n > 0 {
								mu.Lock()
								received[s.User()] += string(buf[:n])
								mu.Unlock()
								ch.Write(buf[:n])
							}
							if e != nil {
								return
							}
						}
					}()
				}
			}()
		}
	}()
	b := broadcast.New()
	defer b.Close()
	tmp := t.TempDir()
	data := filepath.Join(tmp, "hosts.dat")
	aesKey := bytes.Repeat([]byte{42}, 32)
	marker := fmt.Sprintf("ssh-broadcast-live-%d", time.Now().UnixNano())
	roles := []string{"target1", "target2", "excluded", "source"}
	list := model.HostList{Categories: []model.HostCategory{{Name: "test"}}}
	for _, role := range roles {
		list.Categories[0].Hosts = append(list.Categories[0].Hosts, model.HostInfo{Name: marker + "-" + role, Address: "127.0.0.1", Port: l.Addr().(*net.TCPAddr).Port, Username: role, Password: "test-only"})
	}
	if err = host.SaveHostData(data, aesKey, list); err != nil {
		t.Fatal(err)
	}
	launch := func(w *nativeWindow, index int) int {
		t.Helper()
		role := roles[index]
		addr, token, e := b.Issue(role)
		if e != nil {
			t.Fatal(e)
		}
		argv := []string{filepath.Join(filepath.Dir(exe), "ssh-client"), "-f", data, "-k", base64.URLEncoding.EncodeToString(aesKey), "-ci", "1", "-hi", strconv.Itoa(index + 1), "-relay-address", addr, "-relay-token", token}
		target := w.target()
		args := w.tilixArgs([]string{exe, childFlag, filepath.Join(w.dir, "control")}, target, false)
		args = append([]string{"--title=" + marker + "-" + role}, args...)
		cmd := exec.Command("tilix", args...)
		cmd.Env = cleanTerminalEnv(os.Environ())
		if target != nil {
			cmd.Env = append(cmd.Env, "TILIX_ID="+target.id.TilixID)
		}
		if e = cmd.Start(); e != nil {
			t.Fatal(e)
		}
		failed := make(chan error, 1)
		go func() {
			if e := cmd.Wait(); e != nil {
				failed <- e
			}
		}()
		if e = w.acceptLaunchedPane(argv, target, failed); e != nil {
			t.Fatal(e)
		}
		return cmd.Process.Pid
	}
	w, err := newNativeWindow("tilix")
	if err != nil {
		t.Fatal(err)
	}
	defer w.close()
	launch(w, 0)
	launch(w, 1)
	launch(w, 2)
	sourceWindow, err := newNativeWindow("tilix")
	if err != nil {
		t.Fatal(err)
	}
	defer sourceWindow.close()
	sourcePID := launch(sourceWindow, 3)
	wait := func(check func() bool) {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			if check() {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("live condition timeout", b.Snapshot())
	}
	wait(func() bool { return len(b.Snapshot().Connections) == 4 })
	ids := map[string]string{}
	for _, c := range b.Snapshot().Connections {
		ids[c.Label] = c.ID
	}
	input := func(keys string) {
		t.Helper()
		cmd := exec.Command("python3", "../../tools/tilix-test-input.py", marker+"-source", keys, strconv.Itoa(sourcePID))
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("input: %v %s", e, out)
		}
	}
	get := func(role string) string { mu.Lock(); defer mu.Unlock(); return received[role] }
	if err = b.Configure(ids["source"], []string{ids["target1"], ids["target2"]}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	input("b,r,o,a,d,c,a,s,t,Return,Up,Control_L+c")
	wait(func() bool {
		return strings.Contains(get("target1"), "broadcast\r") && get("source") == get("target1") && get("source") == get("target2")
	})
	if get("excluded") != "" {
		t.Fatal("excluded pane received input")
	}
	input("Control_L+bracketright")
	wait(func() bool { return !b.Snapshot().Enabled })
	before := get("target1")
	input("o,f,f,Return")
	time.Sleep(250 * time.Millisecond)
	if get("target1") != before || get("target2") != before {
		t.Fatal("input continued after emergency stop")
	}
	if err = b.Configure(ids["source"], []string{ids["target2"]}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	input("o,n,e,Return")
	wait(func() bool { return strings.HasSuffix(get("target2"), "one\r") })
	if get("target1") != before || get("excluded") != "" {
		t.Fatal("selection isolation")
	}
	mu.Lock()
	ch := channels["target2"]
	mu.Unlock()
	ch.Close()
	wait(func() bool { return !b.Snapshot().Enabled && len(b.Snapshot().Connections) == 3 })
	if err = b.Configure(ids["source"], []string{ids["target1"]}); err != nil {
		t.Fatal(err)
	}
	input("--close")
	wait(func() bool { return !b.Snapshot().Enabled && len(b.Snapshot().Connections) == 2 })
	t.Log("PASS: real Tilix windows/panes, source and two receivers, excluded pane, control keys, emergency stop, reselection, SSH disconnect and source window destruction")
	// Exercise the production batch launcher against the same isolated server.
	previousExecutable := terminalExecutable
	terminalExecutable = func() (string, error) { return exe, nil }
	defer func() { terminalExecutable = previousExecutable; Cleanup() }()
	t.Setenv("SSH_MANAGER_TERMINAL", "tilix")
	batchArgs := []SshClientArgument{}
	for i := 0; i < 3; i++ {
		address, token, e := b.Issue("batch-" + strconv.Itoa(i))
		if e != nil {
			t.Fatal(e)
		}
		batchArgs = append(batchArgs, SshClientArgument{HostsFile: data, HostFileKEY: aesKey, CategoryIndex: 1, HostIndex: i + 1, RelayAddress: address, RelayToken: token, SplitVertical: i == 2})
	}
	if count, e := OpenBatch(batchArgs); e != nil || count != 3 {
		t.Fatal(count, e)
	}
	if len(nativeState.windows) != 1 || len(nativeState.windows[0].panes) != 3 {
		t.Fatal("batch did not create three isolated panes")
	}
	firstBatch := nativeState.windows[0]
	// No broadcast credentials needed to test the second dedicated window.
	for i := range batchArgs {
		batchArgs[i].RelayAddress = ""
		batchArgs[i].RelayToken = ""
	}
	if count, e := OpenBatch(batchArgs[:2]); e != nil || count != 2 {
		t.Fatal(count, e)
	}
	if len(nativeState.windows) != 2 || len(firstBatch.panes) != 3 || len(nativeState.windows[1].panes) != 2 {
		t.Fatal("second batch mixed with first window")
	}
	missing := batchArgs[0]
	missing.BatchWindow = "missing-batch-window"
	if _, e := openTerminal(missing); e == nil {
		t.Fatal("missing batch fell back to a different window")
	}
	if len(nativeState.windows) != 2 || len(firstBatch.panes) != 3 || len(nativeState.windows[1].panes) != 2 {
		t.Fatal("missing batch changed existing windows")
	}
	if b.Snapshot().Enabled {
		t.Fatal("batch enabled broadcasting")
	}
	t.Log("PASS: production batch launcher, 3-pane and 2-pane dedicated windows, alternating splits, broadcast remains off")
	for _, count := range []int{2, 4, 5, 6, 8, 9} {
		columns := 3
		if count == 5 || count == 8 {
			columns = count
		}
		if count == 2 {
			columns = 1
		}
		if count == 4 {
			columns = 2
		}
		gridHosts := model.HostList{Categories: []model.HostCategory{{Name: "grid-test"}}}
		gridArgs := make([]SshClientArgument, count)
		gridFile := filepath.Join(tmp, fmt.Sprintf("grid-%d.dat", count))
		for i := range gridArgs {
			user := fmt.Sprintf("grid-%d-%d", count, i+1)
			gridHosts.Categories[0].Hosts = append(gridHosts.Categories[0].Hosts, model.HostInfo{Name: user, Address: "127.0.0.1", Port: l.Addr().(*net.TCPAddr).Port, Username: user, Password: "test-only"})
			gridArgs[i] = SshClientArgument{HostsFile: gridFile, HostFileKEY: aesKey, CategoryIndex: 1, HostIndex: i + 1}
		}
		if e := host.SaveHostData(gridFile, aesKey, gridHosts); e != nil {
			t.Fatal(e)
		}
		planned, e := PlanGrid(gridArgs, columns)
		if e != nil {
			t.Fatal(e)
		}
		if opened, e := OpenBatch(planned); e != nil || opened != count {
			t.Fatal("grid", count, opened, e)
		}
		gridWindow := nativeState.windows[len(nativeState.windows)-1]
		if len(gridWindow.panes) != count {
			t.Fatal("wrong grid pane count")
		}
		wait(func() bool {
			mu.Lock()
			defer mu.Unlock()
			for i := range gridArgs {
				if channels[fmt.Sprintf("grid-%d-%d", count, i+1)] == nil {
					return false
				}
			}
			return true
		})
		mu.Lock()
		for i := range gridArgs {
			user := fmt.Sprintf("grid-%d-%d", count, i+1)
			channels[user].Write([]byte(fmt.Sprintf("\r\nGRID %d — HOST %d\r\n", count, i+1)))
		}
		mu.Unlock()
		t.Logf("PASS: Tilix %d-host/%d-column grid, all assigned SSH users authenticated", count, columns)
		wait(func() bool {
			mu.Lock()
			defer mu.Unlock()
			var minW, minH uint32 = 10000, 10000
			var maxW, maxH uint32
			for i := range gridArgs {
				d := dimensions[fmt.Sprintf("grid-%d-%d", count, i+1)]
				if d[0] < 10 || d[1] < 3 {
					return false
				}
				minW = min(minW, d[0])
				maxW = max(maxW, d[0])
				minH = min(minH, d[1])
				maxH = max(maxH, d[1])
			}
			return maxW-minW <= 2 && maxH-minH <= 2
		})
		t.Logf("PASS: Tilix %d-host grid PTYs have equal dimensions within terminal-cell rounding", count)
	}
}

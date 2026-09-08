//go:build linux || freebsd

package terminal

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDesktopSelection(t *testing.T) {
	for _, tt := range []struct{ name, desktop, fallback, override, want string }{
		{"KDE with GTK installed", "KDE", "", "", "konsole"},
		{"Ubuntu with Konsole installed", "ubuntu:GNOME", "", "", "tilix"},
		{"fallback", "", "plasma", "", "konsole"},
		{"explicit override", "GNOME", "", "konsole", "konsole"},
		{"unknown desktop", "sway", "", "", "tilix"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{"XDG_CURRENT_DESKTOP": tt.desktop, "DESKTOP_SESSION": tt.fallback, "SSH_MANAGER_TERMINAL": tt.override}
			got, err := selectBackend(func(k string) string { return env[k] }, func(s string) (string, error) { return "/usr/bin/" + s, nil })
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

func TestMissingTilixDoesNotFallbackToKonsole(t *testing.T) {
	installed := false
	getenv := func(k string) string {
		if k == "XDG_CURRENT_DESKTOP" {
			return "ubuntu:GNOME"
		}
		return ""
	}
	lookup := func(s string) (string, error) {
		if s == "konsole" || (s == "tilix" && installed) {
			return "/usr/bin/" + s, nil
		}
		return "", os.ErrNotExist
	}
	s := terminalStatus(getenv, lookup, "ID=ubuntu\nID_LIKE=debian")
	if s.Ready || s.Backend != "tilix" || !strings.Contains(s.Message, "sudo apt install tilix") {
		t.Fatalf("%+v", s)
	}
	installed = true
	if s = terminalStatus(getenv, lookup, "ubuntu"); !s.Ready {
		t.Fatalf("installation should be picked up without restart: %+v", s)
	}
}

func TestActualEnvironmentStatus(t *testing.T) {
	s := Status()
	t.Logf("backend=%s ready=%v message=%s", s.Backend, s.Ready, s.Message)
}

func TestFreeBSDTerminalStatus(t *testing.T) {
	for _, desktop := range []string{"KDE", "GNOME"} {
		t.Run(desktop, func(t *testing.T) {
			getenv := func(key string) string {
				if key == "XDG_CURRENT_DESKTOP" {
					return desktop
				}
				return ""
			}
			installed := map[string]bool{}
			lookup := func(name string) (string, error) {
				if installed[name] {
					return "/usr/local/bin/" + name, nil
				}
				return "", os.ErrNotExist
			}
			backend := "tilix"
			if desktop == "KDE" {
				backend = "konsole"
			}
			s := terminalStatus(getenv, lookup, "freebsd")
			if s.Ready || s.Backend != backend || !strings.Contains(s.Message, "pkg install "+backend) {
				t.Fatalf("missing terminal: %+v", s)
			}
			installed[backend] = true
			if backend == "konsole" {
				s = terminalStatus(getenv, lookup, "freebsd")
				if s.Ready || !strings.Contains(s.Message, "pkg install glib") {
					t.Fatalf("missing gdbus: %+v", s)
				}
				installed["gdbus"] = true
			}
			if s = terminalStatus(getenv, lookup, "freebsd"); !s.Ready {
				t.Fatalf("installed terminal: %+v", s)
			}
		})
	}
}

func TestTilixTargetAndDirection(t *testing.T) {
	w := &nativeWindow{dir: "/tmp/ssh-manager-terminal-123"}
	helper := []string{"/path with spaces/manager", childFlag, "/tmp/control"}
	p := &nativePane{id: paneIdentity{TilixID: "e7dd738c-43f0-4aaf-adfd-000000000001"}}
	for _, vertical := range []bool{false, true} {
		action := "session-add-right"
		if vertical {
			action = "session-add-down"
		}
		want := append([]string{"--group=ssh-manager-terminal-123", "--action=" + action, "--focus-window", "-x"}, helper...)
		if got := w.tilixArgs(helper, p, vertical); !reflect.DeepEqual(got, want) {
			t.Fatalf("%q", got)
		}
	}
	env := cleanTerminalEnv([]string{"PATH=/bin", "TILIX_ID=personal", "KONSOLE_DBUS_SERVICE=:1.9", "XDG_CURRENT_DESKTOP=GNOME"})
	if len(env) != 2 {
		t.Fatalf("inherited terminal identity not removed: %q", env)
	}
}

func TestKonsoleOnlyActivatesValidatedSession(t *testing.T) {
	id := paneIdentity{Service: ":1.42", Session: "/Sessions/7"}
	for _, tt := range []struct {
		name                     string
		vertical                 bool
		actions, current, result string
		failure                  bool
		calls                    int
	}{
		{"right", false, "(['split-view-left-right'],)", "(7,)", "(true,)", false, 4},
		{"down", true, "(['split-view-top-bottom'],)", "(7,)", "(true,)", false, 4},
		{"unsupported version", false, "([],)", "(7,)", "(true,)", true, 1},
		{"session closed", false, "(['split-view-left-right'],)", "(8,)", "(true,)", true, 3},
		{"rejected action", false, "(['split-view-left-right'],)", "(7,)", "(false,)", true, 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			err := splitKonsole(id, tt.vertical, func(service, path, method string, args ...string) (string, error) {
				calls++
				if service != id.Service {
					t.Fatal("wrong process")
				}
				switch calls {
				case 1:
					if method != "org.kde.KMainWindow.actions" || path != "/konsole/MainWindow_1" {
						t.Fatal(method, path)
					}
					return tt.actions, nil
				case 2:
					if method != "org.kde.konsole.Window.setCurrentSession" || path != "/Windows/1" || !reflect.DeepEqual(args, []string{"7"}) {
						t.Fatal(method, path, args)
					}
					return "()", nil
				case 3:
					if method != "org.kde.konsole.Window.currentSession" {
						t.Fatal(method)
					}
					return tt.current, nil
				case 4:
					if method != "org.kde.KMainWindow.activateAction" {
						t.Fatal(method)
					}
					return tt.result, nil
				}
				t.Fatal("unexpected call")
				return "", nil
			})
			if (err != nil) != tt.failure || calls != tt.calls {
				t.Fatalf("calls=%d error=%v", calls, err)
			}
		})
	}
	if err := splitKonsole(id, false, func(string, string, string, ...string) (string, error) { return "", errors.New("no bus") }); err == nil {
		t.Fatal("bus failure ignored")
	}
}

func TestPaneHandshakeAndLifetime(t *testing.T) {
	w, err := newNativeWindow("tilix")
	if err != nil {
		t.Fatal(err)
	}
	defer w.close()
	info, err := os.Stat(w.dir)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("control directory permissions", err)
	}
	argv := []string{"/path with spaces/ssh-client", "-f", "/tmp/a'b.dat", "-k", "test-only"}
	done := make(chan error, 1)
	go func() { done <- w.acceptPane(argv, nil) }()
	c, err := net.Dial("unix", filepath.Join(w.dir, "control"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(time.Second * 3))
	id := paneIdentity{TilixID: "e7dd738c-43f0-4aaf-adfd-000000000001"}
	json.NewEncoder(c).Encode(id)
	var got []string
	if err = json.NewDecoder(c).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, argv) {
		t.Fatalf("argument corruption: %q", got)
	}
	json.NewEncoder(c).Encode("")
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if w.target() == nil {
		t.Fatal("missing live pane")
	}
	c.Close()
	deadline := time.Now().Add(time.Second)
	for w.target() != nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if w.target() != nil {
		t.Fatal("closed pane remains target")
	}
}

func TestForeignKonsoleHandshakeRejectedBeforeSendingCommand(t *testing.T) {
	w, err := newNativeWindow("konsole")
	if err != nil {
		t.Fatal(err)
	}
	defer w.close()
	done := make(chan error, 1)
	target := &nativePane{id: paneIdentity{Service: ":1.42", Session: "/Sessions/1"}}
	go func() { done <- w.acceptPane([]string{"/must/not/run"}, target) }()
	c, err := net.Dial("unix", filepath.Join(w.dir, "control"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	json.NewEncoder(c).Encode(paneIdentity{Service: ":1.99", Session: "/Sessions/2"})
	if err = <-done; err == nil {
		t.Fatal("foreign process accepted")
	}
	var argv []string
	if err = json.NewDecoder(c).Decode(&argv); err == nil {
		t.Fatal("command leaked to foreign pane")
	}
}

func TestMostRecentLivePane(t *testing.T) {
	a, b := &nativePane{}, &nativePane{}
	a.alive.Store(true)
	b.alive.Store(true)
	w := &nativeWindow{panes: []*nativePane{a, b}}
	if w.target() != b {
		t.Fatal("not latest")
	}
	b.alive.Store(false)
	if w.target() != a {
		t.Fatal("not previous live pane")
	}
	a.alive.Store(false)
	if w.target() != nil {
		t.Fatal("closed pane selected")
	}
}

func TestChildRunsReceivedCommand(t *testing.T) {
	t.Setenv("TILIX_ID", "e7dd738c-43f0-4aaf-adfd-000000000001")
	w, err := newNativeWindow("tilix")
	if err != nil {
		t.Fatal(err)
	}
	defer w.close()
	command := filepath.Join(t.TempDir(), "command with spaces")
	if err := os.WriteFile(command, []byte("#!/bin/sh\n[ \"$1\" = \"a'b c\" ]\n"), 0700); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- runChild(filepath.Join(w.dir, "control")) }()
	if err := w.acceptPane([]string{command, "a'b c"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

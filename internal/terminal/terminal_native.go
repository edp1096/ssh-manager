//go:build linux || freebsd

package terminal

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const childFlag = "--ssh-manager-terminal-child"
const terminalTimeout = 15 * time.Second

type paneIdentity struct {
	TilixID string
	Service string
	Session string
}

type nativePane struct {
	id    paneIdentity
	alive atomic.Bool
}

type nativeWindow struct {
	backend  string
	dir      string
	listener *net.UnixListener
	panes    []*nativePane
}

var nativeState struct {
	sync.Mutex
	windows []*nativeWindow
}

// The helper receives arguments over a private socket, never through terminal
// keystrokes. Konsole inherits this helper command when it creates a split.
// Its connection remains open for the SSH process lifetime, tracking live panes.
func RunChild() bool {
	if len(os.Args) < 2 || os.Args[1] != childFlag {
		return false
	}
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "Invalid terminal helper arguments")
		os.Exit(1)
	}
	if err := runChild(os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "SSH terminal:", err)
		os.Exit(1)
	}
	return true
}

func runChild(socket string) error {
	c, err := net.DialTimeout("unix", socket, terminalTimeout)
	if err != nil {
		return fmt.Errorf("관리 앱과 연결할 수 없습니다: %w", err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(terminalTimeout))
	id := paneIdentity{os.Getenv("TILIX_ID"), os.Getenv("KONSOLE_DBUS_SERVICE"), os.Getenv("KONSOLE_DBUS_SESSION")}
	if err = json.NewEncoder(c).Encode(id); err != nil {
		return err
	}
	var argv []string
	if err = json.NewDecoder(io.LimitReader(c, 1<<20)).Decode(&argv); err != nil {
		return fmt.Errorf("관리 앱에서 분할/연결을 요청하세요 (수동 분할은 지원하지 않습니다): %w", err)
	}
	if len(argv) == 0 || !filepath.IsAbs(argv[0]) {
		return fmt.Errorf("invalid SSH command")
	}
	c.SetDeadline(time.Time{})
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err = cmd.Start(); err != nil {
		json.NewEncoder(c).Encode(err.Error())
		return fmt.Errorf("ssh-client 실행 실패: %w", err)
	}
	if err = json.NewEncoder(c).Encode(""); err != nil {
		return cmd.Wait()
	}
	return cmd.Wait()
}

func (w *nativeWindow) close() {
	w.listener.Close()
	os.RemoveAll(w.dir) // Only the private directory returned by MkdirTemp below.
}

func Cleanup() {
	nativeState.Lock()
	defer nativeState.Unlock()
	for _, w := range nativeState.windows {
		w.close()
	}
	nativeState.windows = nil
}

func (w *nativeWindow) target() *nativePane {
	for i := len(w.panes) - 1; i >= 0; i-- {
		if w.panes[i].alive.Load() {
			return w.panes[i]
		}
	}
	return nil
}

func newNativeWindow(backend string) (*nativeWindow, error) {
	// Short path also avoids Unix socket pathname limits with long TMPDIR paths.
	dir, err := os.MkdirTemp("/tmp", "ssh-manager-terminal-")
	if err != nil {
		return nil, err
	}
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(dir, "control"), Net: "unix"})
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	return &nativeWindow{backend: backend, dir: dir, listener: l}, nil
}

func cleanTerminalEnv(env []string) []string {
	result := make([]string, 0, len(env))
	for _, entry := range env {
		if strings.HasPrefix(entry, "TILIX_ID=") || strings.HasPrefix(entry, "KONSOLE_DBUS_") {
			continue
		}
		result = append(result, entry)
	}
	return result
}

func (w *nativeWindow) tilixArgs(helper []string, target *nativePane, vertical bool) []string {
	// A group isolates this app's window from personal Tilix windows. This is
	// an experimental Tilix option; do not silently fall back to its default group.
	args := []string{"--group=" + filepath.Base(w.dir)}
	if target != nil {
		action := "session-add-right"
		if vertical {
			action = "session-add-down"
		}
		args = append(args, "--action="+action, "--focus-window")
	}
	return append(append(args, "-x"), helper...)
}

func openTerminal(arg SshClientArgument) (int, error) {
	nativeState.Lock()
	defer nativeState.Unlock()
	s := Status()
	if !s.Ready {
		return -1, fmt.Errorf("%s", s.Message)
	}
	if arg.CategoryIndex < 1 || arg.HostIndex < 1 {
		return -1, fmt.Errorf("잘못된 호스트 인덱스입니다.")
	}
	exe, err := os.Executable()
	if err != nil {
		return -1, err
	}
	client := filepath.Join(filepath.Dir(exe), "ssh-client")
	if _, err = exec.LookPath(client); err != nil {
		return -1, fmt.Errorf("앱과 같은 폴더에 실행 가능한 ssh-client가 필요합니다.")
	}
	hostFile := arg.HostsFile
	if strings.HasPrefix(hostFile, "./") {
		hostFile = filepath.Join(filepath.Dir(exe), filepath.Base(hostFile))
	}
	hostFile, err = filepath.Abs(hostFile)
	if err != nil {
		return -1, err
	}
	argv := []string{client, "-f", hostFile, "-k", base64.URLEncoding.EncodeToString(arg.HostFileKEY), "-ci", strconv.Itoa(arg.CategoryIndex), "-hi", strconv.Itoa(arg.HostIndex)}
	argv = append(argv, relayArguments(arg)...)

	// Remove ended windows and select the most recently used live app window.
	var live []*nativeWindow
	var w *nativeWindow
	for _, previous := range nativeState.windows {
		if previous.target() == nil {
			previous.close()
			continue
		}
		live = append(live, previous)
		if previous.backend == s.Backend {
			w = previous
		}
	}
	nativeState.windows = live
	if arg.NewWindow {
		w = nil
	}
	var target *nativePane
	if w == nil {
		w, err = newNativeWindow(s.Backend)
		if err != nil {
			return -1, err
		}
		nativeState.windows = append(nativeState.windows, w)
	} else {
		target = w.target()
	}
	helper := []string{exe, childFlag, filepath.Join(w.dir, "control")}
	pid := 0
	launchFailure := make(chan error, 1)
	if s.Backend == "konsole" && target != nil {
		err = splitKonsole(target.id, arg.SplitVertical, dbusCall)
	} else {
		var args []string
		if s.Backend == "tilix" {
			args = w.tilixArgs(helper, target, arg.SplitVertical)
		} else {
			args = append([]string{"--separate", "-e"}, helper...)
		}
		cmd := exec.Command(s.Backend, args...)
		var diagnostic bytes.Buffer
		cmd.Stderr = &diagnostic
		cmd.Env = cleanTerminalEnv(os.Environ())
		if target != nil {
			cmd.Env = append(cmd.Env, "TILIX_ID="+target.id.TilixID)
		}
		// Only helper/socket arguments go to the launcher, never SSH credentials.
		err = cmd.Start()
		if err == nil {
			pid = cmd.Process.Pid
			go func() {
				if waitErr := cmd.Wait(); waitErr != nil {
					message := strings.TrimSpace(diagnostic.String())
					if len(message) > 2048 {
						message = message[:2048]
					}
					launchFailure <- fmt.Errorf("터미널 실행 실패: %v\n%s", waitErr, message)
				}
			}()
		}
	}
	if err == nil {
		err = w.acceptLaunchedPane(argv, target, launchFailure)
	}
	if err != nil {
		// Invalidate the controller after ambiguous/late responses; never retry a
		// split against an arbitrary active window or send input to an SSH session.
		w.close()
		for _, p := range w.panes {
			p.alive.Store(false)
		}
		return -1, fmt.Errorf("%s 터미널 연결/분할 실패: %w", s.Backend, err)
	}
	return pid, nil
}

func (w *nativeWindow) acceptLaunchedPane(argv []string, target *nativePane, launchFailure <-chan error) error {
	ready := make(chan error, 1)
	go func() { ready <- w.acceptPane(argv, target) }()
	select {
	case err := <-ready:
		return err
	case err := <-launchFailure:
		w.listener.Close()
		<-ready
		return err
	}
}

var tilixIDPattern = regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)
var servicePattern = regexp.MustCompile(`^(:[0-9]+\.[0-9]+|org\.kde\.konsole-[0-9]+)$`)
var sessionPattern = regexp.MustCompile(`^/Sessions/[1-9][0-9]*$`)

func validIdentity(backend string, id paneIdentity) bool {
	if backend == "tilix" {
		return tilixIDPattern.MatchString(id.TilixID)
	}
	return servicePattern.MatchString(id.Service) && sessionPattern.MatchString(id.Session)
}

func (w *nativeWindow) acceptPane(argv []string, target *nativePane) error {
	deadline := time.Now().Add(terminalTimeout)
	w.listener.SetDeadline(deadline)
	c, err := w.listener.AcceptUnix()
	if err != nil {
		return fmt.Errorf("새 분할 영역의 응답이 없습니다. 터미널 버전과 분할 지원을 확인하세요.")
	}
	ok := false
	defer func() {
		if !ok {
			c.Close()
		}
	}()
	c.SetDeadline(deadline)
	decoder := json.NewDecoder(io.LimitReader(c, 8192))
	var id paneIdentity
	if err = decoder.Decode(&id); err != nil {
		return err
	}
	if !validIdentity(w.backend, id) {
		return fmt.Errorf("터미널 식별자를 확인할 수 없습니다.")
	}
	if target != nil && w.backend == "konsole" && id.Service != target.id.Service {
		return fmt.Errorf("Konsole 프로세스가 일치하지 않습니다.")
	}
	for _, pane := range w.panes {
		if pane.id == id {
			return fmt.Errorf("새 분할 영역이 아닌 기존 세션이 응답했습니다.")
		}
	}
	if err = json.NewEncoder(c).Encode(argv); err != nil {
		return err
	}
	var startError string
	if err = decoder.Decode(&startError); err != nil {
		return err
	}
	if startError != "" {
		return fmt.Errorf("ssh-client 실행에 실패했습니다.")
	}
	c.SetDeadline(time.Time{})
	pane := &nativePane{id: id}
	pane.alive.Store(true)
	w.panes = append(w.panes, pane)
	ok = true
	go func() {
		io.Copy(io.Discard, c)
		pane.alive.Store(false)
		c.Close()
	}()
	return nil
}

type dbusCaller func(service, path, method string, args ...string) (string, error)

func dbusCall(service, path, method string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	params := append([]string{"call", "--session", "--dest", service, "--object-path", path, "--method", method}, args...)
	output, err := exec.CommandContext(ctx, "gdbus", params...).Output()
	if err != nil {
		return "", fmt.Errorf("Konsole D-Bus 호출 실패 (%s). 설치된 버전의 인터페이스를 확인하세요.", method)
	}
	return strings.TrimSpace(string(output)), nil
}

func splitKonsole(id paneIdentity, vertical bool, call dbusCaller) error {
	if !validIdentity("konsole", id) {
		return fmt.Errorf("잘못된 Konsole 대상입니다.")
	}
	action := "split-view-left-right"
	if vertical {
		action = "split-view-top-bottom"
	}
	// --separate creates a dedicated process with its initial window numbered 1.
	// Query actions first: do not assume an unsupported action silently worked.
	actions, err := call(id.Service, "/konsole/MainWindow_1", "org.kde.KMainWindow.actions")
	if err != nil {
		return err
	}
	if !strings.Contains(actions, "'"+action+"'") && !strings.Contains(actions, `"`+action+`"`) {
		return fmt.Errorf("이 Konsole 버전에서 %s 액션을 찾을 수 없습니다.", action)
	}
	session := strings.TrimPrefix(id.Session, "/Sessions/")
	if _, err = call(id.Service, "/Windows/1", "org.kde.konsole.Window.setCurrentSession", session); err != nil {
		return err
	}
	current, err := call(id.Service, "/Windows/1", "org.kde.konsole.Window.currentSession")
	if err != nil {
		return err
	}
	if current != "("+session+",)" {
		return fmt.Errorf("분할 대상 Konsole 세션을 선택할 수 없습니다.")
	}
	result, err := call(id.Service, "/konsole/MainWindow_1", "org.kde.KMainWindow.activateAction", action)
	if err != nil {
		return err
	}
	if result != "(true,)" {
		return fmt.Errorf("Konsole이 분할 액션을 거부했습니다.")
	}
	return nil
}

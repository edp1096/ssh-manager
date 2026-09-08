package terminal

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Opt-in: creates real GUI windows, running only short-lived local sleep commands.
// SSH_MANAGER_LIVE_TEST_BINARY must name a freshly built ssh-manager binary.
func TestLiveTilix(t *testing.T) {
	exe := os.Getenv("SSH_MANAGER_LIVE_TEST_BINARY")
	if exe == "" {
		t.Skip("requires explicit live desktop test opt-in")
	}
	if !filepath.IsAbs(exe) {
		t.Fatal("binary path must be absolute")
	}
	w, err := newNativeWindow("tilix")
	if err != nil {
		t.Fatal(err)
	}
	defer w.close()
	launch := func(window *nativeWindow, target *nativePane, vertical bool) {
		t.Helper()
		helper := []string{exe, childFlag, filepath.Join(window.dir, "control")}
		cmd := exec.Command("tilix", window.tilixArgs(helper, target, vertical)...)
		cmd.Env = cleanTerminalEnv(os.Environ())
		if target != nil {
			cmd.Env = append(cmd.Env, "TILIX_ID="+target.id.TilixID)
		}
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		failed := make(chan error, 1)
		go func() {
			if err := cmd.Wait(); err != nil {
				failed <- fmt.Errorf("%v: %s", err, stderr.String())
			}
		}()
		if err := window.acceptLaunchedPane([]string{"/bin/sleep", "8"}, target, failed); err != nil {
			t.Fatal(err)
		}
		t.Logf("live pane acknowledged: %s (vertical=%v)", window.target().id.TilixID, vertical)
	}
	launch(w, nil, false)
	first := w.target()
	launch(w, first, false)
	second := w.target()
	launch(w, second, true)
	if len(w.panes) != 3 || first == second || second == w.target() {
		t.Fatal("distinct split panes not created")
	}
	other, err := newNativeWindow("tilix")
	if err != nil {
		t.Fatal(err)
	}
	defer other.close()
	launch(other, nil, false)
	// Target the original window explicitly while another group/window exists.
	launch(w, w.target(), false)
	if len(other.panes) != 1 || len(w.panes) != 4 {
		t.Fatal("window isolation failed")
	}
	deadline := time.Now().Add(12 * time.Second)
	for (w.target() != nil || other.target() != nil) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if w.target() != nil || other.target() != nil {
		t.Fatal("live panes did not close")
	}
}

func TestLauncherFailureIsReportedImmediately(t *testing.T) {
	w, err := newNativeWindow("tilix")
	if err != nil {
		t.Fatal(err)
	}
	defer w.close()
	failed := make(chan error, 1)
	failed <- fmt.Errorf("unknown option --execute")
	start := time.Now()
	err = w.acceptLaunchedPane(nil, nil, failed)
	if err == nil || err.Error() != "unknown option --execute" {
		t.Fatalf("wrong error: %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("launcher error waited for pane timeout")
	}
}

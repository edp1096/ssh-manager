//go:build linux || freebsd

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ssh-manager/internal/terminal"
)

func TestMissingTerminalStatusAndSessionError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("XDG_CURRENT_DESKTOP", "ubuntu:GNOME")
	t.Setenv("SSH_MANAGER_TERMINAL", "")
	w := httptest.NewRecorder()
	handleTerminalStatus(w, httptest.NewRequest("GET", "/terminal/status", nil))
	var status terminal.TerminalStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Ready || status.Backend != "tilix" || !strings.Contains(status.Message, "tilix") {
		t.Fatalf("%+v", status)
	}
	w = httptest.NewRecorder()
	handleOpenSession(w, httptest.NewRequest("POST", "/session/open", strings.NewReader(`{"hosts-file":"./hosts.dat","category-index":1,"host-index":1}`)))
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "tilix") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

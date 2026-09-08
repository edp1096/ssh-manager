package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"ssh-manager/internal/browser"
)

func TestWindowSizeHandler(t *testing.T) {
	previousDir := WorkingDir
	WorkingDir = t.TempDir()
	t.Cleanup(func() { WorkingDir = previousDir })
	for _, tc := range []struct {
		body string
		code int
	}{
		{`{"width":1100,"height":750}`, 204},
		{`{"width":0,"height":0}`, 400},
		{`{"width":"1100","height":750}`, 400},
		{`broken`, 400},
		{`{"width":1100,"height":750,"position":{"x":-1400,"y":100}}`, 204},
		{`{"width":1100,"height":750,"position":{"x":1000000,"y":100}}`, 400},
	} {
		response := httptest.NewRecorder()
		handleWindowSize(response, httptest.NewRequest("POST", "/window-size", strings.NewReader(tc.body)))
		if response.Code != tc.code {
			t.Fatalf("expected %d, got %d", tc.code, response.Code)
		}
	}
	if got := browser.LoadWindowSize(WorkingDir); got.Width != 1100 || got.Height != 750 || got.Position == nil || *got.Position != (browser.WindowPosition{X: -1400, Y: 100}) {
		t.Fatalf("saved size: %+v", got)
	}
}

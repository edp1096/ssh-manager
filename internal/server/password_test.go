package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"ssh-manager/internal/host"
	"ssh-manager/pkg/model"
)

func TestGetHostPassword(t *testing.T) {
	previousKey := HostFileKEY
	HostFileKEY = make([]byte, 32)
	t.Cleanup(func() { HostFileKEY = previousKey })
	file := filepath.Join(t.TempDir(), "hosts.dat")
	hosts := model.HostList{Categories: []model.HostCategory{{Hosts: []model.HostInfo{{Password: "test-password"}}}}}
	if err := host.SaveHostData(file, HostFileKEY, &hosts); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		category, host string
		status         int
	}{
		{"0", "0", http.StatusOK},
		{"-1", "0", http.StatusBadRequest},
		{"0", "-1", http.StatusBadRequest},
		{"x", "0", http.StatusBadRequest},
		{"0", "", http.StatusBadRequest},
		{"1", "0", http.StatusNotFound},
		{"0", "1", http.StatusNotFound},
	} {
		t.Run(tc.category+"_"+tc.host, func(t *testing.T) {
			params := url.Values{"hosts-file": {file}, "category-idx": {tc.category}, "host-idx": {tc.host}}
			response := httptest.NewRecorder()
			handleGetHostPassword(response, httptest.NewRequest("GET", "/host-password?"+params.Encode(), nil))
			if response.Code != tc.status || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status %d, cache %q", response.Code, response.Header().Get("Cache-Control"))
			}
			if tc.status == http.StatusOK {
				var result map[string]string
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result["password"] != "test-password" {
					t.Fatal("incorrect password response")
				}
			}
		})
	}
}

package server

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"ssh-manager/internal/connectiongroup"
	"ssh-manager/internal/host"
	"ssh-manager/pkg/model"
)

func TestReorderConnectionGroups(t *testing.T) {
	previous := HostFileKEY
	HostFileKEY = make([]byte, 32)
	t.Cleanup(func() { HostFileKEY = previous })
	file := filepath.Join(t.TempDir(), "hosts.dat")
	if err := host.SaveHostData(file, HostFileKEY, &model.HostList{}); err != nil {
		t.Fatal(err)
	}
	var groups []connectiongroup.Group
	for _, name := range []string{"First", "Second"} {
		var err error
		groups, err = connectionGroups.Change(file, "POST", connectiongroup.Group{Name: name, Hosts: []string{"a"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	order, _ := json.Marshal(map[string][]string{"group-ids": {groups[1].ID, groups[0].ID}})
	for _, tc := range []struct {
		body   string
		status int
	}{{string(order), 200}, {`{}`, 400}, {`{"group-ids":null}`, 400}, {`{"group-ids":["missing"]}`, 400}} {
		response := httptest.NewRecorder()
		handleConnectionGroups(response, httptest.NewRequest("PATCH", "/connection-groups?hosts-file="+url.QueryEscape(file), strings.NewReader(tc.body)))
		if response.Code != tc.status {
			t.Fatal(response.Code, response.Body.String())
		}
	}
	loaded, err := connectionGroups.List(file)
	if err != nil || loaded[0].ID != groups[1].ID || loaded[1].ID != groups[0].ID {
		t.Fatal(loaded, err)
	}
}

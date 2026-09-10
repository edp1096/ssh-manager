package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"ssh-manager/internal/connectiongroup"
	"ssh-manager/internal/host"
	"ssh-manager/pkg/model"
)

var connectionGroups connectiongroup.Store

func handleConnectionGroups(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	file, err := filepath.Abs(r.URL.Query().Get("hosts-file"))
	if err != nil {
		http.Error(w, "Invalid hosts file", 400)
		return
	}
	var hosts model.HostList
	if err = host.LoadHostData(file, HostFileKEY, &hosts); err != nil {
		http.Error(w, "Cannot load hosts", 400)
		return
	}
	var groups []connectiongroup.Group
	if r.URL.Query().Get("settings") == "panel" {
		var value *bool
		if r.Method != http.MethodGet {
			var settings struct {
				Pinned *bool `json:"panel-pinned"`
			}
			if r.Method != http.MethodPut || json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&settings) != nil || settings.Pinned == nil {
				http.Error(w, "Invalid panel settings", 400)
				return
			}
			value = settings.Pinned
		}
		pinned, err := connectionGroups.PanelPinned(file, value)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"panel-pinned": pinned})
		return
	}
	if r.Method == http.MethodGet {
		groups, err = connectionGroups.List(file)
	} else {
		var group connectiongroup.Group
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&group) != nil {
			http.Error(w, "Invalid group", 400)
			return
		}
		groups, err = connectionGroups.Change(file, r.Method, group)
	}
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	json.NewEncoder(w).Encode(groups)
}

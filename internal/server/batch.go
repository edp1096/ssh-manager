package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ssh-manager/internal/host"
	"ssh-manager/internal/terminal"
	"ssh-manager/pkg/model"
)

var batchFiles struct {
	sync.Mutex
	dirs []string
}

func selectedHosts(list model.HostList, ids []string) ([]model.HostInfo, error) {
	if len(ids) == 0 || len(ids) > 32 {
		return nil, fmt.Errorf("select between 1 and 32 hosts")
	}
	byID := map[string]model.HostInfo{}
	for _, category := range list.Categories {
		for _, h := range category.Hosts {
			if h.UniqueID != "" {
				if _, ok := byID[h.UniqueID]; ok {
					return nil, fmt.Errorf("duplicate host ID; reload the host list")
				}
				byID[h.UniqueID] = h
			}
		}
	}
	result := make([]model.HostInfo, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		h, ok := byID[id]
		if !ok || id == "" {
			return nil, fmt.Errorf("a selected host no longer exists; reload the list")
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate host selection")
		}
		seen[id] = true
		result = append(result, h)
	}
	return result, nil
}
func snapshotBatch(hosts []model.HostInfo, key []byte) (string, error) {
	dir, err := os.MkdirTemp("", "ssh-manager-batch-")
	if err != nil {
		return "", err
	}
	file := filepath.Join(dir, "hosts.dat")
	list := model.HostList{Categories: []model.HostCategory{{Name: "Batch", Hosts: hosts}}}
	if err = host.SaveHostData(file, key, &list); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	if err = os.Chmod(file, 0600); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	batchFiles.Lock()
	batchFiles.dirs = append(batchFiles.dirs, dir)
	batchFiles.Unlock()
	return file, nil
}
func cleanupBatchFiles() {
	batchFiles.Lock()
	defer batchFiles.Unlock()
	for _, dir := range batchFiles.dirs {
		os.RemoveAll(dir)
	}
	batchFiles.dirs = nil
}

func handleOpenBatch(w http.ResponseWriter, r *http.Request) {
	var request struct {
		File    string   `json:"hosts-file"`
		IDs     []string `json:"host-ids"`
		Layout  string   `json:"layout"`
		Columns int      `json:"columns"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&request) != nil {
		http.Error(w, "Invalid request", 400)
		return
	}
	if request.Layout != "alternating" && request.Layout != "horizontal" && request.Layout != "vertical" && request.Layout != "grid" {
		http.Error(w, "Invalid split layout", 400)
		return
	}
	if request.Layout == "grid" && (request.Columns < 1 || request.Columns > 32) {
		http.Error(w, "Grid requires 1–32 columns", 400)
		return
	}
	if status := terminal.Status(); !status.Ready {
		http.Error(w, status.Message, 503)
		return
	}
	var list model.HostList
	key := append([]byte(nil), HostFileKEY...)
	if err := host.LoadHostData(request.File, key, &list); err != nil {
		http.Error(w, "Cannot load hosts", 400)
		return
	}
	hosts, err := selectedHosts(list, request.IDs)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	file, err := snapshotBatch(hosts, key)
	if err != nil {
		http.Error(w, "Cannot create batch snapshot", 500)
		return
	}
	args := make([]terminal.SshClientArgument, 0, len(hosts))
	for i, h := range hosts {
		address, token, err := inputBroker.Issue(fmt.Sprintf("%s — %s@%s:%d", h.Name, h.Username, h.Address, h.Port))
		if err != nil {
			for _, a := range args {
				inputBroker.Revoke(a.RelayToken)
			}
			http.Error(w, "Cannot prepare batch", 503)
			return
		}
		inputBroker.ExtendPending(token, 10*time.Minute)
		args = append(args, terminal.SshClientArgument{HostsFile: file, HostFileKEY: key, CategoryIndex: 1, HostIndex: i + 1, RelayAddress: address, RelayToken: token, SplitVertical: request.Layout == "vertical" || (request.Layout == "alternating" && i%2 == 0)})
	}
	if request.Layout == "grid" {
		args, err = terminal.PlanGrid(args, request.Columns)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
	}
	opened, err := terminal.OpenBatch(args)
	for _, a := range args[opened:] {
		inputBroker.Revoke(a.RelayToken)
	}
	result := struct {
		Opened int    `json:"opened"`
		Total  int    `json:"total"`
		Error  string `json:"error,omitempty"`
	}{Opened: opened, Total: len(args)}
	if err != nil {
		if opened < len(args) {
			result.Error = fmt.Sprintf("Stopped at %s: %v. Previously opened panes remain open.", hosts[args[opened].HostIndex-1].Name, err)
		} else {
			result.Error = fmt.Sprintf("All launch requests completed, but grid layout failed: %v. Opened panes remain open.", err)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

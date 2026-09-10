package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"ssh-manager/internal/host"
	"ssh-manager/pkg/model"
)

func removeSelectedHosts(list *model.HostList, ids []string) error {
	wanted := make(map[string]bool)
	for _, id := range ids {
		if id == "" || wanted[id] {
			return fmt.Errorf("invalid or duplicate host ID")
		}
		wanted[id] = true
	}
	if len(wanted) == 0 {
		return fmt.Errorf("select at least one host")
	}
	counts := make(map[string]int)
	for _, category := range list.Categories {
		for _, h := range category.Hosts {
			counts[h.UniqueID]++
		}
	}
	for id := range wanted {
		if counts[id] != 1 {
			return fmt.Errorf("host list changed; refresh and select again")
		}
	}
	for i := range list.Categories {
		remaining := make([]model.HostInfo, 0, len(list.Categories[i].Hosts))
		for _, h := range list.Categories[i].Hosts {
			if !wanted[h.UniqueID] {
				remaining = append(remaining, h)
			}
		}
		list.Categories[i].Hosts = remaining
	}
	return nil
}

func handleDeleteSelectedHosts(w http.ResponseWriter, r *http.Request) {
	file := strings.TrimSpace(r.URL.Query().Get("hosts-file"))
	var request struct {
		IDs []string `json:"host-ids"`
	}
	if file == "" || json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request) != nil {
		http.Error(w, "Invalid deletion request", http.StatusBadRequest)
		return
	}
	var list model.HostList
	if err := host.LoadHostData(file, HostFileKEY, &list); err != nil {
		http.Error(w, "Cannot load hosts", http.StatusBadRequest)
		return
	}
	if err := removeSelectedHosts(&list, request.IDs); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := host.SaveHostData(file, HostFileKEY, &list); err != nil {
		http.Error(w, "Cannot save hosts", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"deleted": len(request.IDs)})
}

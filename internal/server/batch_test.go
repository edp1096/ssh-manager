package server

import (
	"path/filepath"
	"testing"

	"ssh-manager/internal/host"
	"ssh-manager/pkg/model"
)

func TestBatchStableIDsAndSnapshot(t *testing.T) {
	list := model.HostList{Categories: []model.HostCategory{{Hosts: []model.HostInfo{{UniqueID: "b", Name: "B", Password: "test-only"}, {UniqueID: "a", Name: "A", PrivateKeyText: "test-key"}}}}}
	selected, err := selectedHosts(list, []string{"a", "b"})
	if err != nil || selected[0].Name != "A" || selected[1].Name != "B" {
		t.Fatal(selected, err)
	}
	for _, ids := range [][]string{nil, {"missing"}, {"a", "a"}} {
		if _, err = selectedHosts(list, ids); err == nil {
			t.Fatal("accepted invalid selection")
		}
	}
	key := make([]byte, 32)
	file, err := snapshotBatch(selected, key)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupBatchFiles()
	if !filepath.IsAbs(file) {
		t.Fatal("relative snapshot")
	}
	list.Categories[0].Hosts[0].Password = "changed"
	var saved model.HostList
	if err = host.LoadHostData(file, key, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Categories[0].Hosts[0].PrivateKeyText != "test-key" || saved.Categories[0].Hosts[1].Password != "test-only" {
		t.Fatal("credentials or order changed")
	}
}

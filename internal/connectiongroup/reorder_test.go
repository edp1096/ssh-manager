package connectiongroup

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReorderGroups(t *testing.T) {
	file := filepath.Join(t.TempDir(), "hosts.dat")
	s := &Store{}
	var original []Group
	for _, name := range []string{"One", "Two", "Three"} {
		var err error
		original, err = s.Change(file, "POST", Group{Name: name, Hosts: []string{"b", "a"}, Layout: "grid", Columns: 2, Fill: "vertical-left"})
		if err != nil {
			t.Fatal(err)
		}
	}
	pinned := true
	if _, err := s.PanelPinned(file, &pinned); err != nil {
		t.Fatal(err)
	}
	want := []Group{original[2], original[0], original[1]}
	got, err := s.Reorder(file, []string{want[0].ID, want[1].ID, want[2].ID})
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal(got, err)
	}
	got, err = (&Store{}).List(file)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("order not persisted", got, err)
	}
	if value, err := s.PanelPinned(file, nil); err != nil || !value {
		t.Fatal("pin changed", err)
	}
	before, err := os.ReadFile(file + ".groups.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, ids := range [][]string{nil, {want[0].ID}, {want[0].ID, want[0].ID, want[2].ID}, {want[0].ID, want[1].ID, "missing"}, {want[0].ID, want[1].ID, want[2].ID, "extra"}} {
		if _, err := s.Reorder(file, ids); err == nil {
			t.Fatal("accepted invalid order", ids)
		}
		after, err := os.ReadFile(file + ".groups.json")
		if err != nil || string(after) != string(before) {
			t.Fatal("invalid order changed file", err)
		}
	}
}

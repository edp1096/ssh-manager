package connectiongroup

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPanelPinPersistence(t *testing.T) {
	file := filepath.Join(t.TempDir(), "hosts.dat")
	s := &Store{}
	if pinned, err := s.PanelPinned(file, nil); err != nil || pinned {
		t.Fatal(pinned, err)
	}
	pinned := true
	if _, err := s.PanelPinned(file, &pinned); err != nil {
		t.Fatal(err)
	}
	groups, err := s.Change(file, "POST", Group{Name: "Test", Hosts: []string{"one"}, Layout: "grid", Columns: 2, Fill: "vertical"})
	if err != nil {
		t.Fatal(err)
	}
	other := &Store{}
	if value, err := other.PanelPinned(file, nil); err != nil || !value {
		t.Fatal(value, err)
	}
	pinned = false
	if _, err := other.PanelPinned(file, &pinned); err != nil {
		t.Fatal(err)
	}
	loaded, err := other.List(file)
	if err != nil || !reflect.DeepEqual(groups, loaded) {
		t.Fatal(loaded, err)
	}
	if value, err := (&Store{}).PanelPinned(file, nil); err != nil || value {
		t.Fatal(value, err)
	}
}

func TestGroupPersistenceAndIsolation(t *testing.T) {
	file := filepath.Join(t.TempDir(), "hosts.dat")
	original := []byte("unchanged encrypted host file")
	if err := os.WriteFile(file, original, 0600); err != nil {
		t.Fatal(err)
	}
	s := &Store{}
	groups, err := s.Change(file, "POST", Group{Name: "운영 서버", Hosts: []string{"b", "a"}})
	if err != nil {
		t.Fatal(err)
	}
	other := &Store{}
	loaded, err := other.List(file)
	if err != nil || !reflect.DeepEqual(groups, loaded) {
		t.Fatal(loaded, err)
	}
	if _, err = s.Change(file, "POST", Group{Name: "운영 서버", Hosts: []string{"a"}}); err == nil {
		t.Fatal("duplicate name accepted")
	}
	g := groups[0]
	g.Name = "Renamed"
	g.Hosts = []string{"a", "missing-id"}
	groups, err = s.Change(file, "PUT", g)
	if err != nil || !reflect.DeepEqual(groups[0], g) {
		t.Fatal(groups, err)
	}
	if data, _ := os.ReadFile(file); !reflect.DeepEqual(data, original) {
		t.Fatal("modified hosts.dat")
	}
	if groups, err = s.List(file + "-other"); err != nil || len(groups) != 0 {
		t.Fatal("host file isolation")
	}
	if groups, err = s.Change(file, "DELETE", Group{ID: g.ID}); err != nil || len(groups) != 0 {
		t.Fatal(groups, err)
	}
}
func TestCorruptGroupFileIsNotOverwritten(t *testing.T) {
	file := filepath.Join(t.TempDir(), "hosts.dat")
	data := []byte("broken JSON")
	if err := os.WriteFile(file+".groups.json", data, 0600); err != nil {
		t.Fatal(err)
	}
	s := &Store{}
	if _, err := s.Change(file, "POST", Group{Name: "new", Hosts: []string{"a"}}); err == nil {
		t.Fatal("accepted corrupt file")
	}
	if actual, _ := os.ReadFile(file + ".groups.json"); !reflect.DeepEqual(actual, data) {
		t.Fatal("overwrote corrupt group file")
	}
}

func TestLegacyLayoutAndPersistence(t *testing.T) {
	file := filepath.Join(t.TempDir(), "hosts.dat")
	legacy := []byte(`{"version":1,"groups":[{"id":"old","name":"Legacy","host-ids":["a"]}]}`)
	if err := os.WriteFile(file+".groups.json", legacy, 0600); err != nil {
		t.Fatal(err)
	}
	s := &Store{}
	groups, err := s.List(file)
	if err != nil || groups[0].Layout != "alternating" {
		t.Fatal(groups, err)
	}
	for _, layout := range []string{"horizontal", "vertical", "alternating"} {
		g := groups[0]
		g.Layout = layout
		if _, err := s.Change(file, "PUT", g); err != nil {
			t.Fatal(err)
		}
		groups, err = (&Store{}).List(file)
		if err != nil || groups[0].Layout != layout {
			t.Fatal(groups, err)
		}
	}
	g := groups[0]
	g.Layout = "invalid"
	if _, err := s.Change(file, "PUT", g); err == nil {
		t.Fatal("invalid layout accepted")
	}
	g.Layout = ""
	g.Name = "Renamed"
	groups, err = s.Change(file, "PUT", g)
	if err != nil || groups[0].Layout != "alternating" {
		t.Fatal(groups, err)
	}
}

func TestGridGroupPersistence(t *testing.T) {
	file := filepath.Join(t.TempDir(), "hosts.dat")
	s := &Store{}
	groups, err := s.Change(file, "POST", Group{Name: "Grid", Hosts: []string{"a", "b", "c"}, Layout: "grid", Columns: 3})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := (&Store{}).List(file)
	if err != nil || !reflect.DeepEqual(groups, loaded) {
		t.Fatal(loaded, err)
	}
	g := groups[0]
	g.Name = "Renamed"
	g.Hosts = []string{"c", "b"}
	groups, err = s.Change(file, "PUT", g)
	if err != nil || groups[0].Columns != 3 {
		t.Fatal(groups, err)
	}
	for _, columns := range []int{-1, 0, 33} {
		g.Columns = columns
		if _, err = s.Change(file, "PUT", g); err == nil {
			t.Fatal("invalid grid columns accepted")
		}
	}
	for _, columns := range []int{1, 5, 8, 32} {
		g.Columns = columns
		if _, err = s.Change(file, "PUT", g); err != nil {
			t.Fatal(err)
		}
		loaded, err = s.List(file)
		if err != nil || loaded[0].Columns != columns {
			t.Fatal(loaded, err)
		}
	}
}

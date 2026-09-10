// Package connectiongroup stores named host-ID selections beside hosts.dat.
// Host credentials and the existing encrypted host-file format are untouched.
package connectiongroup

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

type Group struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Hosts   []string `json:"host-ids"`
	Layout  string   `json:"layout"`
	Columns int      `json:"columns,omitempty"`
	Fill    string   `json:"grid-fill,omitempty"`
}
type document struct {
	Version     int     `json:"version"`
	Groups      []Group `json:"groups"`
	PanelPinned bool    `json:"panel-pinned"`
}
type Store struct{ mu sync.Mutex }

func validate(g Group) error {
	if g.Fill != "" && g.Fill != "horizontal" && g.Fill != "vertical" && g.Fill != "vertical-left" {
		return fmt.Errorf("invalid grid fill")
	}
	if g.Layout != "alternating" && g.Layout != "horizontal" && g.Layout != "vertical" && g.Layout != "grid" {
		return fmt.Errorf("invalid split layout")
	}
	if g.Layout == "grid" && (g.Columns < 1 || g.Columns > 32) {
		return fmt.Errorf("grid requires 1–32 columns")
	}
	if strings.TrimSpace(g.Name) == "" || utf8.RuneCountInString(g.Name) > 80 {
		return fmt.Errorf("group name must be between 1 and 80 characters")
	}
	if len(g.Hosts) == 0 || len(g.Hosts) > 32 {
		return fmt.Errorf("select between 1 and 32 hosts")
	}
	seen := map[string]bool{}
	for _, id := range g.Hosts {
		if id == "" || len(id) > 128 || seen[id] {
			return fmt.Errorf("invalid or duplicate host ID")
		}
		seen[id] = true
	}
	return nil
}
func read(file string) (document, error) {
	d := document{Version: 1, Groups: []Group{}}
	f, err := os.Open(file)
	if os.IsNotExist(err) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, 1<<20))
	if err = dec.Decode(&d); err != nil {
		return d, fmt.Errorf("cannot read connection groups; existing file was not changed: %w", err)
	}
	if d.Version != 1 || len(d.Groups) > 100 {
		return d, fmt.Errorf("unsupported connection group file")
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return d, fmt.Errorf("invalid connection group file")
	}
	seen := map[string]bool{}
	for i := range d.Groups {
		if d.Groups[i].Layout == "" {
			d.Groups[i].Layout = "alternating"
		}
		g := d.Groups[i]
		if g.ID == "" || seen[g.ID] {
			return d, fmt.Errorf("invalid group ID")
		}
		seen[g.ID] = true
		if err = validate(g); err != nil {
			return d, err
		}
	}
	if d.Groups == nil {
		d.Groups = []Group{}
	}
	return d, nil
}
func (s *Store) List(hostFile string) ([]Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := read(hostFile + ".groups.json")
	return d.Groups, err
}
func (s *Store) Change(hostFile, method string, g Group) ([]Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if method == "POST" {
		g.ID = ""
	}
	file := hostFile + ".groups.json"
	d, err := read(file)
	if err != nil {
		return nil, err
	}
	index := -1
	for i, old := range d.Groups {
		if old.ID == g.ID {
			index = i
			break
		}
	}
	if method == "DELETE" {
		if index < 0 {
			return nil, fmt.Errorf("group no longer exists")
		}
		d.Groups = append(d.Groups[:index], d.Groups[index+1:]...)
	} else {
		if g.Layout == "" {
			if index >= 0 {
				g.Layout = d.Groups[index].Layout
			} else {
				g.Layout = "alternating"
			}
		}
		g.Name = strings.TrimSpace(g.Name)
		if err = validate(g); err != nil {
			return nil, err
		}
		for _, old := range d.Groups {
			if old.ID != g.ID && strings.EqualFold(old.Name, g.Name) {
				return nil, fmt.Errorf("a group with that name already exists")
			}
		}
		switch method {
		case "POST":
			if len(d.Groups) >= 100 {
				return nil, fmt.Errorf("maximum of 100 connection groups")
			}
			var id [16]byte
			if _, err = rand.Read(id[:]); err != nil {
				return nil, err
			}
			g.ID = hex.EncodeToString(id[:])
			d.Groups = append(d.Groups, g)
		case "PUT":
			if index < 0 {
				return nil, fmt.Errorf("group no longer exists")
			}
			d.Groups[index] = g
		default:
			return nil, fmt.Errorf("invalid group operation")
		}
	}
	return d.Groups, write(file, d)
}

// Reorder accepts an exact permutation so stale clients cannot drop groups.
func (s *Store) Reorder(hostFile string, ids []string) ([]Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	file := hostFile + ".groups.json"
	d, err := read(file)
	if err != nil {
		return nil, err
	}
	if len(ids) != len(d.Groups) {
		return nil, fmt.Errorf("group list changed; reload before reordering")
	}
	byID := make(map[string]Group, len(d.Groups))
	for _, g := range d.Groups {
		byID[g.ID] = g
	}
	ordered := make([]Group, 0, len(ids))
	for _, id := range ids {
		g, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("invalid or duplicate group ID; reload before reordering")
		}
		ordered = append(ordered, g)
		delete(byID, id)
	}
	d.Groups = ordered
	return d.Groups, write(file, d)
}

// PanelPinned reads or updates the preference without replacing saved groups.
func (s *Store) PanelPinned(hostFile string, pinned *bool) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	file := hostFile + ".groups.json"
	d, err := read(file)
	if err != nil {
		return false, err
	}
	if pinned != nil {
		d.PanelPinned = *pinned
		err = write(file, d)
	}
	return d.PanelPinned, err
}

func write(file string, d document) error {
	tmp, err := os.CreateTemp(filepath.Dir(file), ".connection-groups-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err = json.NewEncoder(tmp).Encode(d); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(tmpName, file); err != nil {
		return err
	}
	return nil
}

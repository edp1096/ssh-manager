// Package filetransfer provides independent SFTP file sessions. It does not
// change the host store or share connections with external SSH terminals.
package filetransfer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"ssh-manager/pkg/model"
)

type Lookup func(file, id string) (model.HostInfo, error)
type session struct {
	host          model.HostInfo
	protocol      string
	ctx           context.Context
	cancel        context.CancelFunc
	policy        string
	batchPolicies map[string]string
}
type Job struct {
	ID          string    `json:"id"`
	Session     string    `json:"session"`
	Direction   string    `json:"direction"`
	Source      string    `json:"source"`
	Destination string    `json:"destination"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	Bytes       int64     `json:"bytes"`
	Total       int64     `json:"total"`
	Overwrite   bool      `json:"overwrite"`
	Interactive bool      `json:"interactive"`
	Batch       string    `json:"batch,omitempty"`
	Conflict    *Conflict `json:"conflict,omitempty"`
	Skipped     int       `json:"skipped,omitempty"`
	decision    chan resolution
	choices     map[string]resolution
	cancel      context.CancelFunc
}
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*session
	jobs     []*Job
	lookup   Lookup
	tail     <-chan struct{}
}

func New(lookup Lookup) *Manager {
	ready := make(chan struct{})
	close(ready)
	return &Manager{sessions: make(map[string]*session), lookup: lookup, tail: ready}
}
func token() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		s.cancel()
	}
	clear(m.sessions)
}
func respond(w http.ResponseWriter, data any, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(data)
}
func decode(w http.ResponseWriter, r *http.Request, value any) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(value)
}
func (m *Manager) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /files/sessions", m.open)
	mux.HandleFunc("DELETE /files/sessions/{id}", m.closeSession)
	mux.HandleFunc("GET /files/sessions/{id}/entries", m.entries)
	mux.HandleFunc("POST /files/jobs", m.startJob)
	mux.HandleFunc("POST /files/jobs/{id}/resolve", m.resolveConflict)
	mux.HandleFunc("DELETE /files/jobs", func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.URL.Query().Get("session")
		if sessionID == "" {
			respond(w, nil, errors.New("A session is required to clear transfer history"))
			return
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		active := make([]*Job, 0, len(m.jobs))
		for _, j := range m.jobs {
			if j.Session != sessionID || j.Status == "queued" || j.Status == "running" || j.Status == "waiting" {
				active = append(active, j)
			}
		}
		m.jobs = active
		respond(w, map[string]bool{"ok": true}, nil)
	})
	mux.HandleFunc("POST /files/sessions/{id}/operation", m.operation)
	mux.HandleFunc("GET /files/jobs", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		respond(w, m.jobs, nil)
	})
	mux.HandleFunc("DELETE /files/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		for _, j := range m.jobs {
			if j.ID == r.PathValue("id") {
				j.cancel()
				respond(w, map[string]bool{"ok": true}, nil)
				return
			}
		}
		respond(w, nil, errors.New("Transfer not found"))
	})
}
func (m *Manager) open(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File       string      `json:"hostsFile"`
		ID         string      `json:"hostId"`
		Connection *Connection `json:"connection"`
	}
	if err := decode(w, r, &req); err != nil {
		respond(w, nil, err)
		return
	}
	var h model.HostInfo
	var err error
	protocol := "sftp"
	if req.Connection != nil {
		err = req.Connection.validate()
		protocol = req.Connection.Protocol
		h = model.HostInfo{Address: req.Connection.Address, Port: req.Connection.Port, Username: req.Connection.Username, Password: req.Connection.Password}
	} else {
		h, err = m.lookup(req.File, req.ID)
	}
	if err != nil {
		respond(w, nil, err)
		return
	}
	c, done, err := connectRemote(r.Context(), h, protocol)
	if err != nil {
		respond(w, nil, fmt.Errorf("File connection failed: %w", err))
		return
	}
	defer done()
	remote, err := c.Getwd()
	if err != nil {
		respond(w, nil, err)
		return
	}
	local, err := os.UserHomeDir()
	if err != nil {
		respond(w, nil, err)
		return
	}
	id := token()
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.sessions[id] = &session{host: h, protocol: protocol, ctx: ctx, cancel: cancel}
	m.mu.Unlock()
	respond(w, map[string]string{"id": id, "local": local, "remote": remote}, nil)
}
func (m *Manager) closeSession(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := r.PathValue("id")
	if s := m.sessions[id]; s != nil {
		s.cancel()
		delete(m.sessions, id)
	}
	respond(w, map[string]bool{"ok": true}, nil)
}
func (m *Manager) get(id string) (*session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[id]
	if s == nil {
		return nil, errors.New("File session closed. Close this tab and reconnect.")
	}
	return s, nil
}

type Entry struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	Modified  time.Time `json:"modified"`
	Directory bool      `json:"directory"`
	Link      bool      `json:"link"`
}

func (m *Manager) entries(w http.ResponseWriter, r *http.Request) {
	s, err := m.get(r.PathValue("id"))
	if err != nil {
		respond(w, nil, err)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	p := r.URL.Query().Get("path")
	side := r.URL.Query().Get("side")
	var infos []os.FileInfo
	var parent string
	join := filepath.Join
	if side == "remote" {
		var c filesystem
		var done func()
		c, done, err = connectRemote(ctx, s.host, s.protocol)
		if err == nil {
			defer done()
			p, err = c.RealPath(p)
			if err == nil {
				infos, err = c.ReadDirContext(ctx, p)
			}
		}
		parent = path.Dir(p)
		join = path.Join
	} else if side == "local" {
		p, err = filepath.Abs(p)
		if err == nil {
			var entries []os.DirEntry
			entries, err = os.ReadDir(p)
			for _, e := range entries {
				info, eErr := e.Info()
				if eErr != nil {
					err = eErr
					break
				}
				infos = append(infos, info)
			}
		}
		parent = filepath.Dir(p)
	} else {
		err = errors.New("Invalid filesystem side")
	}
	if err != nil {
		respond(w, nil, err)
		return
	}
	result := make([]Entry, 0, len(infos))
	for _, f := range infos {
		result = append(result, Entry{f.Name(), join(p, f.Name()), f.Size(), f.ModTime(), f.IsDir(), f.Mode()&os.ModeSymlink != 0})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Directory != result[j].Directory {
			return result[i].Directory
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	respond(w, map[string]any{"path": p, "parent": parent, "entries": result}, nil)
}
func (m *Manager) startJob(w http.ResponseWriter, r *http.Request) {
	var j Job
	if err := decode(w, r, &j); err != nil {
		respond(w, nil, err)
		return
	}
	s, err := m.get(j.Session)
	if err != nil {
		respond(w, nil, err)
		return
	}
	if j.Direction != "upload" && j.Direction != "download" {
		respond(w, nil, errors.New("Invalid transfer direction"))
		return
	}
	if j.Source == "" || j.Destination == "" {
		respond(w, nil, errors.New("Source and destination are required"))
		return
	}
	j.ID = token()
	j.Status = "queued"
	j.Error = ""
	j.Bytes = 0
	j.Total = 0
	j.Skipped = 0
	j.Conflict = nil
	j.choices = make(map[string]resolution)
	if len(j.Batch) > 64 {
		respond(w, nil, errors.New("Invalid transfer batch"))
		return
	}
	if j.Batch == "" {
		j.Batch = token()
	}
	ctx, cancel := context.WithCancel(s.ctx)
	j.cancel = cancel
	m.mu.Lock()
	// Keep memory bounded; only completed history can be evicted.
	if len(m.jobs) >= 500 {
		for i, old := range m.jobs {
			if old.Status != "queued" && old.Status != "running" && old.Status != "waiting" {
				m.jobs = append(m.jobs[:i], m.jobs[i+1:]...)
				break
			}
		}
	}
	if len(m.jobs) >= 500 {
		m.mu.Unlock()
		cancel()
		respond(w, nil, errors.New("Transfer queue is full"))
		return
	}
	m.jobs = append(m.jobs, &j)
	previous := m.tail
	finished := make(chan struct{})
	m.tail = finished
	respond(w, j, nil)
	m.mu.Unlock()
	go m.run(ctx, s.host, s.protocol, &j, previous, finished)
}
func (m *Manager) run(ctx context.Context, h model.HostInfo, protocol string, j *Job, previous <-chan struct{}, finished chan<- struct{}) {
	// Preserve submission order, even when a queued job is cancelled.
	defer func() { <-previous; close(finished) }()
	defer j.cancel()
	var err error
	select {
	case <-previous:
		m.mu.Lock()
		j.Status = "running"
		m.mu.Unlock()
		for {
			err = m.transferRemote(ctx, h, protocol, j)
			var conflict *conflictError
			if !errors.As(err, &conflict) {
				break
			}
			if err = m.waitConflict(ctx, j, conflict.Conflict); err != nil {
				break
			}
		}
	case <-ctx.Done():
		err = ctx.Err()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	j.Conflict = nil
	if err == nil {
		j.Status = "completed"
	} else if ctx.Err() != nil {
		j.Status = "cancelled"
		// Keep recovery paths visible if cancellation interrupted publication.
		j.Error = err.Error()
	} else {
		j.Status = "failed"
		j.Error = err.Error()
	}
}

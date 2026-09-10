// Package broadcast relays input only between explicitly selected app-owned SSH
// clients. It never injects keys into a desktop terminal or records input.
package broadcast

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"
)

type frame struct {
	ID    string `json:"id,omitempty"`
	Kind  string `json:"kind"`
	Token string `json:"token,omitempty"`
	Epoch uint64 `json:"epoch,omitempty"`
	Role  string `json:"role,omitempty"`
	Data  []byte `json:"data,omitempty"`
}
type Connection struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}
type State struct {
	Connections []Connection `json:"connections"`
	Source      string       `json:"source"`
	Targets     []string     `json:"targets"`
	Enabled     bool         `json:"enabled"`
	Reason      string       `json:"reason"`
}
type ticket struct {
	Connection
	expires time.Time
}
type peer struct {
	Connection
	conn  net.Conn
	queue chan frame
}
type Broker struct {
	mu       sync.Mutex
	listener net.Listener
	pending  map[string]ticket
	peers    map[string]*peer
	source   string
	targets  map[string]bool
	epoch    uint64
	reason   string
	closed   bool
}

func New() *Broker {
	return &Broker{pending: map[string]ticket{}, peers: map[string]*peer{}, targets: map[string]bool{}}
}

// Issue grants a single launch a short-lived, one-use capability. The token is
// passed only to that ssh-client, never returned by the browser listing API.
func (b *Broker) Issue(label string) (address, token string, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return "", "", fmt.Errorf("input relay is closed")
	}
	if b.listener == nil {
		b.listener, err = net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			return "", "", err
		}
		go b.accept(b.listener)
	}
	for key, t := range b.pending {
		if time.Now().After(t.expires) {
			delete(b.pending, key)
		}
	}
	var secret [32]byte
	if _, err = rand.Read(secret[:]); err != nil {
		return "", "", err
	}
	token = hex.EncodeToString(secret[:])
	var id [8]byte
	if _, err = rand.Read(id[:]); err != nil {
		return "", "", err
	}
	b.pending[token] = ticket{Connection{hex.EncodeToString(id[:]), label}, time.Now().Add(2 * time.Minute)}
	return b.listener.Addr().String(), token, nil
}

// ExtendPending allows later clients in a sequential batch time to start.
func (b *Broker) ExtendPending(token string, duration time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if t, ok := b.pending[token]; ok {
		t.expires = time.Now().Add(duration)
		b.pending[token] = t
	}
}

func (b *Broker) Revoke(token string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.pending, token)
}
func (b *Broker) accept(l net.Listener) {
	for {
		c, err := l.Accept()
		if err != nil {
			return
		}
		go b.serve(c)
	}
}
func (b *Broker) serve(c net.Conn) {
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	scan := bufio.NewScanner(c)
	scan.Buffer(make([]byte, 4096), 32768)
	var hello frame
	if !scan.Scan() || json.Unmarshal(scan.Bytes(), &hello) != nil || hello.Kind != "hello" {
		return
	}
	b.mu.Lock()
	t, ok := b.pending[hello.Token]
	delete(b.pending, hello.Token)
	if !ok || b.closed || time.Now().After(t.expires) {
		b.mu.Unlock()
		return
	}
	p := &peer{Connection: t.Connection, conn: c, queue: make(chan frame, 64)}
	b.peers[p.ID] = p
	p.queue <- frame{Kind: "state", Epoch: b.epoch, Role: "off", ID: p.ID}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.peers, p.ID)
		if b.source == p.ID || b.targets[p.ID] {
			b.stopLocked("A selected connection closed; broadcast stopped.")
		}
	}()
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		enc := json.NewEncoder(c)
		for {
			var f frame
			select {
			case <-done:
				return
			case f = <-p.queue:
			case <-ticker.C:
				f.Kind = "ping"
			}
			c.SetWriteDeadline(time.Now().Add(3 * time.Second))
			if enc.Encode(f) != nil {
				c.Close()
				return
			}
		}
	}()
	for {
		c.SetReadDeadline(time.Now().Add(12 * time.Second))
		if !scan.Scan() {
			return
		}
		var f frame
		if json.Unmarshal(scan.Bytes(), &f) != nil {
			return
		}
		if f.Kind == "stop" {
			b.mu.Lock()
			b.stopLocked("Stopped from terminal.")
			b.mu.Unlock()
			continue
		}
		if f.Kind == "pong" {
			continue
		}
		if f.Kind != "input" || len(f.Data) == 0 || len(f.Data) > 8192 {
			return
		}
		b.mu.Lock()
		if b.source == p.ID && f.Epoch == b.epoch {
			for id := range b.targets {
				q := b.peers[id]
				select {
				case q.queue <- frame{Kind: "input", Epoch: b.epoch, Data: f.Data}:
				default:
					b.stopLocked("A receiver is too slow; broadcast stopped.")
				}
				if b.source == "" {
					break
				}
			}
		}
		b.mu.Unlock()
	}
}
func (b *Broker) notifyLocked() {
	for id, p := range b.peers {
		role := "off"
		if id == b.source {
			role = "source"
		} else if b.targets[id] {
			role = "target"
		}
		select {
		case p.queue <- frame{Kind: "state", Epoch: b.epoch, Role: role}:
		default:
			p.conn.Close()
		}
	}
}
func (b *Broker) stopLocked(reason string) {
	b.source = ""
	b.targets = map[string]bool{}
	b.epoch++
	b.reason = reason
	b.notifyLocked()
}
func (b *Broker) Configure(source string, targets []string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if source == "" {
		b.stopLocked("")
		return nil
	}
	if b.closed || b.peers[source] == nil {
		return fmt.Errorf("source connection is no longer available")
	}
	selected := map[string]bool{}
	for _, id := range targets {
		if id == source || b.peers[id] == nil {
			return fmt.Errorf("invalid or disconnected receiver")
		}
		selected[id] = true
	}
	if len(selected) == 0 {
		return fmt.Errorf("select at least one receiver")
	}
	b.source = source
	b.targets = selected
	b.epoch++
	b.reason = ""
	b.notifyLocked()
	return nil
}
func (b *Broker) Snapshot() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := State{Connections: []Connection{}, Source: b.source, Targets: []string{}, Enabled: b.source != "", Reason: b.reason}
	for _, p := range b.peers {
		s.Connections = append(s.Connections, p.Connection)
	}
	for id := range b.targets {
		s.Targets = append(s.Targets, id)
	}
	sort.Slice(s.Connections, func(i, j int) bool { return s.Connections[i].ID < s.Connections[j].ID })
	sort.Strings(s.Targets)
	return s
}
func (b *Broker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	if b.listener != nil {
		b.listener.Close()
	}
	for _, p := range b.peers {
		p.conn.Close()
	}
	b.pending = map[string]ticket{}
	b.source = ""
	b.targets = map[string]bool{}
}
func (b *Broker) Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodPost {
		if origin := r.Header.Get("Origin"); (origin != "" && origin != "http://"+r.Host) || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(w, "Cross-origin request rejected", http.StatusForbidden)
			return
		}
		var request struct {
			Source  string   `json:"source"`
			Targets []string `json:"targets"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&request) != nil {
			http.Error(w, "Invalid request", 400)
			return
		}
		if err := b.Configure(request.Source, request.Targets); err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
	}
	json.NewEncoder(w).Encode(b.Snapshot())
}

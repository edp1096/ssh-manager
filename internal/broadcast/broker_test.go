package broadcast

import (
	"encoding/json"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type testPeer struct {
	net.Conn
	enc *json.Encoder
	dec *json.Decoder
	id  string
}

func TestSnapshotUsesLaunchOrder(t *testing.T) {
	b := New()
	defer b.Close()
	var issued []Connection
	for _, label := range []string{"First", "Second", "Third"} {
		_, token, err := b.Issue(label)
		if err != nil {
			t.Fatal(err)
		}
		b.mu.Lock()
		issued = append(issued, b.pending[token].Connection)
		delete(b.pending, token)
		b.mu.Unlock()
	}
	// Simulate reverse client arrival with IDs also sorted opposite to launch.
	b.mu.Lock()
	for i := len(issued) - 1; i >= 0; i-- {
		c := issued[i]
		c.ID = []string{"z", "m", "a"}[i]
		b.peers[c.ID] = &peer{Connection: c}
	}
	b.mu.Unlock()
	s := b.Snapshot()
	for i, c := range s.Connections {
		if c.Label != issued[i].Label {
			t.Fatalf("unexpected order: %+v", s.Connections)
		}
	}
	b.mu.Lock()
	clear(b.peers)
	b.mu.Unlock()
}

func TestHTTPRejectsCrossOriginAndInvalidTargets(t *testing.T) {
	b := New()
	defer b.Close()
	r := httptest.NewRequest("POST", "http://127.0.0.1/session/broadcast", strings.NewReader(`{"source":"","targets":[]}`))
	r.Header.Set("Origin", "https://example.org")
	w := httptest.NewRecorder()
	b.Handler(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	a := attach(t, b, "source")
	if b.Configure(a.id, []string{a.id}) == nil {
		t.Fatal("self receiver accepted")
	}
	if b.Configure(a.id, nil) == nil {
		t.Fatal("empty receivers accepted")
	}
	if b.Configure("missing", []string{a.id}) == nil {
		t.Fatal("missing source accepted")
	}
	w = httptest.NewRecorder()
	b.Handler(w, httptest.NewRequest("GET", "/session/broadcast", nil))
	if strings.Contains(w.Body.String(), "token") {
		t.Fatal("capability exposed to browser")
	}
}

func attach(t *testing.T, b *Broker, label string) *testPeer {
	t.Helper()
	address, token, err := b.Issue(label)
	if err != nil {
		t.Fatal(err)
	}
	c, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	p := &testPeer{Conn: c, enc: json.NewEncoder(c), dec: json.NewDecoder(c)}
	p.send(t, frame{Kind: "hello", Token: token})
	p.id = p.read(t).ID
	if p.id == "" {
		t.Fatal("missing connection ID")
	}
	return p
}
func (p *testPeer) send(t *testing.T, f frame) {
	t.Helper()
	if err := p.enc.Encode(f); err != nil {
		t.Fatal(err)
	}
}
func (p *testPeer) read(t *testing.T) frame {
	t.Helper()
	p.SetReadDeadline(time.Now().Add(time.Second))
	var f frame
	if err := p.dec.Decode(&f); err != nil {
		t.Fatal(err)
	}
	return f
}
func TestSelectionEpochDisconnectAndNoLoop(t *testing.T) {
	b := New()
	defer b.Close()
	a, c, d := attach(t, b, "same-host"), attach(t, b, "same-host"), attach(t, b, "excluded")
	if a.id == c.id {
		t.Fatal("duplicate connection identity")
	}
	if err := b.Configure(a.id, []string{c.id}); err != nil {
		t.Fatal(err)
	}
	as, cs, ds := a.read(t), c.read(t), d.read(t)
	if as.Role != "source" || cs.Role != "target" || ds.Role != "off" {
		t.Fatal(as, cs, ds)
	}
	a.send(t, frame{Kind: "input", Epoch: as.Epoch, Data: []byte("한글\x1b[A\r\x03")})
	if got := c.read(t); string(got.Data) != "한글\x1b[A\r\x03" {
		t.Fatal(got)
	}
	// Receivers cannot republish; stale input and excluded peers are ignored.
	c.send(t, frame{Kind: "input", Epoch: as.Epoch, Data: []byte("loop")})
	d.send(t, frame{Kind: "input", Epoch: as.Epoch, Data: []byte("excluded")})
	a.send(t, frame{Kind: "input", Epoch: as.Epoch - 1, Data: []byte("stale")})
	a.send(t, frame{Kind: "input", Epoch: as.Epoch, Data: []byte("sentinel")})
	if got := c.read(t); string(got.Data) != "sentinel" {
		t.Fatal(got)
	}
	if err := b.Configure("", nil); err != nil {
		t.Fatal(err)
	}
	if a.read(t).Role != "off" || c.read(t).Role != "off" || d.read(t).Role != "off" {
		t.Fatal("stop")
	}
	a.send(t, frame{Kind: "input", Epoch: as.Epoch, Data: []byte("late")})
	if err := b.Configure(a.id, []string{d.id}); err != nil {
		t.Fatal(err)
	}
	as = a.read(t)
	c.read(t)
	d.read(t)
	a.send(t, frame{Kind: "input", Epoch: as.Epoch, Data: []byte("new")})
	if string(d.read(t).Data) != "new" {
		t.Fatal("new selection")
	}
	d.Close()
	deadline := time.Now().Add(time.Second)
	for b.Snapshot().Enabled && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if b.Snapshot().Enabled || len(b.Snapshot().Connections) != 2 {
		t.Fatal(b.Snapshot())
	}
	if b.Configure(a.id, []string{d.id}) == nil {
		t.Fatal("accepted dead receiver")
	}
}
func TestOneUseTokenAndEmergencyStop(t *testing.T) {
	b := New()
	defer b.Close()
	addr, token, err := b.Issue("one")
	if err != nil {
		t.Fatal(err)
	}
	connect := func() net.Conn {
		c, e := net.Dial("tcp", addr)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { c.Close() })
		json.NewEncoder(c).Encode(frame{Kind: "hello", Token: token})
		return c
	}
	c := connect()
	var f frame
	c.SetReadDeadline(time.Now().Add(time.Second))
	if json.NewDecoder(c).Decode(&f) != nil {
		t.Fatal("first token rejected")
	}
	duplicate := connect()
	duplicate.SetReadDeadline(time.Now().Add(time.Second))
	if json.NewDecoder(duplicate).Decode(&frame{}) == nil {
		t.Fatal("token reused")
	}
	d := attach(t, b, "second")
	if err := b.Configure(f.ID, []string{d.id}); err != nil {
		t.Fatal(err)
	}
	d.read(t)
	d.send(t, frame{Kind: "stop"})
	if d.read(t).Role != "off" {
		t.Fatal("emergency stop failed")
	}
}

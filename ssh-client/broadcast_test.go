package main

import (
	"bytes"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"
)

type safeBuffer struct {
	sync.Mutex
	bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	return b.Buffer.Write(p)
}
func (b *safeBuffer) Close() error { return nil }
func (b *safeBuffer) text() string { b.Lock(); defer b.Unlock(); return b.Buffer.String() }
func TestBridgeInputAndNoRebroadcast(t *testing.T) {
	remote := &safeBuffer{}
	b := newInputBridge(remote)
	defer b.Close()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err = b.connect(l.Addr().String(), "test-only"); err != nil {
		t.Fatal(err)
	}
	c, err := l.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	enc, dec := json.NewEncoder(c), json.NewDecoder(c)
	var f relayFrame
	if err = dec.Decode(&f); err != nil || f.Kind != "hello" || f.Token != "test-only" {
		t.Fatal(f, err)
	}
	setRole := func(role string, epoch uint64) {
		t.Helper()
		enc.Encode(relayFrame{Kind: "state", Role: role, Epoch: epoch})
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			b.mu.Lock()
			ok := b.role == role && b.epoch == epoch
			b.mu.Unlock()
			if ok {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("state timeout")
	}
	setRole("source", 1)
	b.Write([]byte("한글\r\x03\x1b[A"))
	c.SetReadDeadline(time.Now().Add(time.Second))
	if err = dec.Decode(&f); err != nil || string(f.Data) != remote.text() {
		t.Fatal(f, err)
	}
	setRole("target", 2)
	enc.Encode(relayFrame{Kind: "input", Epoch: 1, Data: []byte("stale")})
	enc.Encode(relayFrame{Kind: "input", Epoch: 2, Data: []byte("received")})
	// Ping replies share the output queue: the next frame must be pong, not input.
	enc.Encode(relayFrame{Kind: "ping"})
	if err = dec.Decode(&f); err != nil || f.Kind != "pong" {
		t.Fatal(f, err)
	}
	if remote.text() != "한글\r\x03\x1b[Areceived" {
		t.Fatal(remote.text())
	}
	b.Write([]byte{29})
	if err = dec.Decode(&f); err != nil || f.Kind != "stop" {
		t.Fatal(f, err)
	}
	if bytes.Contains([]byte(remote.text()), []byte{29}) {
		t.Fatal("stop key reached remote")
	}
	c.Close()
	deadline := time.Now().Add(time.Second)
	select {
	case <-b.done:
	case <-time.After(time.Until(deadline)):
		t.Fatal("relay disconnect not detected")
	}
	b.Write([]byte("ordinary"))
	if remote.text() != "한글\r\x03\x1b[Areceivedordinary" {
		t.Fatal("ordinary SSH failed after relay exit")
	}
}
func TestRelayLoopbackOnly(t *testing.T) {
	b := newInputBridge(&safeBuffer{})
	defer b.Close()
	if b.connect("192.0.2.1:22", "secret") == nil {
		t.Fatal("non-loopback accepted")
	}
}

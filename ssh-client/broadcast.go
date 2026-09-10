package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// The wire format deliberately has no terminal output or host credentials.
type relayFrame struct {
	ID    string `json:"id,omitempty"`
	Kind  string `json:"kind"`
	Token string `json:"token,omitempty"`
	Epoch uint64 `json:"epoch,omitempty"`
	Role  string `json:"role,omitempty"`
	Data  []byte `json:"data,omitempty"`
}
type inputBridge struct {
	writeMu sync.Mutex
	remote  io.WriteCloser
	mu      sync.Mutex
	role    string
	epoch   uint64
	conn    net.Conn
	queue   chan relayFrame
	done    chan struct{}
	once    sync.Once
}

func newInputBridge(remote io.WriteCloser) *inputBridge {
	return &inputBridge{remote: remote, queue: make(chan relayFrame, 64), done: make(chan struct{})}
}
func (b *inputBridge) connect(address, token string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return fmt.Errorf("relay must be loopback")
	}
	c, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return err
	}
	b.conn = c
	c.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if err = json.NewEncoder(c).Encode(relayFrame{Kind: "hello", Token: token}); err != nil {
		c.Close()
		return err
	}
	go b.readLoop()
	go b.writeLoop()
	return nil
}
func (b *inputBridge) Close() error {
	b.once.Do(func() {
		b.mu.Lock()
		b.role = ""
		b.mu.Unlock()
		close(b.done)
		if b.conn != nil {
			b.conn.Close()
		}
	})
	return nil
}
func (b *inputBridge) send(f relayFrame) {
	select {
	case <-b.done:
		return
	default:
	}
	select {
	case b.queue <- f:
	default:
		b.Close()
	}
}
func (b *inputBridge) writeLoop() {
	defer b.Close()
	enc := json.NewEncoder(b.conn)
	for {
		select {
		case <-b.done:
			return
		case f := <-b.queue:
			b.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
			if enc.Encode(f) != nil {
				return
			}
		}
	}
}
func (b *inputBridge) readLoop() {
	defer b.Close()
	scan := bufio.NewScanner(b.conn)
	scan.Buffer(make([]byte, 4096), 32768)
	for {
		b.conn.SetReadDeadline(time.Now().Add(12 * time.Second))
		if !scan.Scan() {
			return
		}
		var f relayFrame
		if json.Unmarshal(scan.Bytes(), &f) != nil {
			return
		}
		switch f.Kind {
		case "ping":
			b.send(relayFrame{Kind: "pong"})
		case "state":
			if len(f.ID) == 16 {
				name := strings.Map(func(r rune) rune {
					if r < 32 || r == 127 {
						return -1
					}
					return r
				}, host.Name)
				fmt.Fprintf(os.Stdout, "\033]0;%s [%s]\007\r\nSSH connection: %s [%s]\r\n", name, f.ID[:8], name, f.ID[:8])
			}
			b.mu.Lock()
			b.role = f.Role
			b.epoch = f.Epoch
			b.mu.Unlock()
		case "input":
			b.mu.Lock()
			valid := b.role == "target" && b.epoch == f.Epoch
			b.mu.Unlock()
			if len(f.Data) > 8192 {
				return
			}
			if valid {
				if _, err := b.writeRemote(f.Data); err != nil {
					return
				}
			}
		default:
			return
		}
	}
}
func (b *inputBridge) writeRemote(data []byte) (int, error) {
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	return b.remote.Write(data)
}

// Only locally typed input enters Write. Relayed input goes straight to
// writeRemote and can never be published a second time.
func (b *inputBridge) Write(data []byte) (int, error) {
	b.mu.Lock()
	role, epoch := b.role, b.epoch
	b.mu.Unlock()
	if (role == "source" || role == "target") && bytes.Contains(data, []byte{29}) {
		b.mu.Lock()
		b.role = "off"
		b.mu.Unlock()
		b.send(relayFrame{Kind: "stop"}) // Ctrl+] is an emergency stop, not remote input.
		return len(data), nil
	}
	n, err := b.writeRemote(data)
	if err != nil {
		b.Close()
		return n, err
	}
	if role == "source" {
		for start := 0; start < n; start += 8192 {
			end := start + 8192
			if end > n {
				end = n
			}
			b.send(relayFrame{Kind: "input", Epoch: epoch, Data: append([]byte(nil), data[start:end]...)})
		}
	}
	return n, nil
}

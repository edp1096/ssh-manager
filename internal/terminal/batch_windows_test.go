package terminal

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestWindowsPaneReadiness(t *testing.T) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	for _, token := range []string{"wrong-token", "expected-token"} {
		conn, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		if err = json.NewEncoder(conn).Encode(token); err != nil {
			t.Fatal(err)
		}
		conn.Close()
	}
	if err = waitWindowsPane(listener, "expected-token", time.Second); err != nil {
		t.Fatal(err)
	}
	if err = waitWindowsPane(listener, "missing-token", 20*time.Millisecond); err == nil {
		t.Fatal("missing pane did not time out")
	}
}

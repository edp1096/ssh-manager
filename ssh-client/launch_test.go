package main

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestLaunchReady(t *testing.T) {
	if err := notifyLaunchReady("", ""); err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{"example.com:1234", "127.0.0.1", ""} {
		if err := notifyLaunchReady(address, "token"); err == nil {
			t.Fatal("accepted invalid startup address")
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err = notifyLaunchReady(listener.Addr().String(), "one-use-token"); err != nil {
		t.Fatal(err)
	}
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(time.Second))
	var token string
	if err = json.NewDecoder(conn).Decode(&token); err != nil || token != "one-use-token" {
		t.Fatal("incorrect acknowledgement", err)
	}
}

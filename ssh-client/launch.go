package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

func notifyLaunchReady(address, token string) error {
	if address == "" && token == "" {
		return nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" || token == "" {
		return fmt.Errorf("invalid local startup address or token")
	}
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return json.NewEncoder(conn).Encode(token)
}

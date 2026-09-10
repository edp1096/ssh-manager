package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/exec"
	"time"
)

const windowsPaneTimeout = 15 * time.Second

func launchWindowsBatch(wt, client string, args []SshClientArgument) (int, error) {
	params, err := windowsBatchArguments(client, args)
	if err != nil {
		return 0, err
	}
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	opened, start := 0, 2
	for end := 2; end <= len(params); end++ {
		if end < len(params) && params[end] != ";" {
			continue
		}
		command := append(append([]string{}, params[:2]...), params[start:end]...)
		createsPane := params[start] == "new-tab" || params[start] == "split-pane"
		token := batchID()
		if createsPane {
			command = append(command, "-launch-ready-address", listener.Addr().String(), "-launch-ready-token", token)
			if len(args) >= 3 {
				command = append(command, "-launch-exit-group", params[1])
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), windowsPaneTimeout)
		err = exec.CommandContext(ctx, wt, command...).Run()
		cancel()
		if err != nil {
			return opened, fmt.Errorf("Windows Terminal command failed; check opened panes: %w", err)
		}
		if createsPane {
			// A running client proves the pane was initialized. Queuing all splits
			// during WT startup can hang its UI when splitting an earlier column.
			if err = waitWindowsPane(listener, token, windowsPaneTimeout); err != nil {
				return opened, fmt.Errorf("Windows Terminal pane did not become ready; rebuild ssh-client.exe together with the app: %w", err)
			}
			opened++
		}
		start = end + 1
	}
	return opened, nil
}

func waitWindowsPane(listener *net.TCPListener, token string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	listener.SetDeadline(deadline)
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			return err
		}
		conn.SetDeadline(deadline)
		var received string
		err = json.NewDecoder(io.LimitReader(conn, 1024)).Decode(&received)
		conn.Close()
		if err == nil && received == token {
			return nil
		}
	}
}

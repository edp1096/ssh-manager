package filetransfer

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"ssh-manager/pkg/model"
)

// Reset deadlines for every network operation: large transfers are allowed,
// but an unresponsive peer cannot keep a request alive indefinitely.
type idleConn struct{ net.Conn }

func (c idleConn) Read(p []byte) (int, error) {
	c.SetReadDeadline(time.Now().Add(30 * time.Second))
	return c.Conn.Read(p)
}
func (c idleConn) Write(p []byte) (int, error) {
	c.SetWriteDeadline(time.Now().Add(30 * time.Second))
	return c.Conn.Write(p)
}

func connect(ctx context.Context, h model.HostInfo) (*sftp.Client, func(), error) {
	auth := ssh.Password(h.Password)
	if h.PrivateKeyText != "" {
		key, err := ssh.ParsePrivateKey([]byte(h.PrivateKeyText))
		if err != nil {
			return nil, nil, err
		}
		auth = ssh.PublicKeys(key)
	}
	address := net.JoinHostPort(h.Address, strconv.Itoa(h.Port))
	conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, nil, err
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	cleanup := func() { stop(); conn.Close() }
	sshConn, channels, requests, err := ssh.NewClientConn(idleConn{conn}, address, &ssh.ClientConfig{
		User: h.Username, Auth: []ssh.AuthMethod{auth}, HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	client := ssh.NewClient(sshConn, channels, requests)
	files, err := sftp.NewClient(client)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return files, func() { cleanup(); files.Close(); client.Close() }, nil
}

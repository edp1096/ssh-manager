package filetransfer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
	"ssh-manager/pkg/model"
)

type Connection struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (c Connection) validate() error {
	if c.Protocol != "ftp" && c.Protocol != "ftps" && c.Protocol != "ftps-implicit" {
		return errors.New("Choose FTP, explicit FTPS or implicit FTPS")
	}
	if strings.TrimSpace(c.Address) == "" || c.Port < 1 || c.Port > 65535 {
		return errors.New("A server address and port (1–65535) are required")
	}
	if strings.ContainsAny(c.Address+c.Username+c.Password, "\r\n\x00") {
		return errors.New("Connection fields cannot contain control characters")
	}
	return nil
}
func connectRemote(ctx context.Context, h model.HostInfo, protocol string) (filesystem, func(), error) {
	return dialRemote(ctx, h, protocol, nil)
}

func dialRemote(ctx context.Context, h model.HostInfo, protocol string, trust *tls.Config) (filesystem, func(), error) {
	if protocol == "" || protocol == "sftp" {
		c, done, err := connect(ctx, h)
		if err != nil {
			return nil, nil, err
		}
		return sftpFS{c}, done, nil
	}
	if protocol != "ftp" && protocol != "ftps" && protocol != "ftps-implicit" {
		return nil, nil, errors.New("Unknown protocol")
	}
	ctx, cancel := context.WithCancel(ctx)
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: h.Address, ClientSessionCache: tls.NewLRUClientSessionCache(8)}
	if trust != nil {
		tlsConfig = trust.Clone()
	}
	firstDial := true
	options := []ftp.DialOption{ftp.DialWithTimeout(10 * time.Second), ftp.DialWithDialFunc(func(network, address string) (net.Conn, error) {
		conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		stop := context.AfterFunc(ctx, func() { conn.Close() })
		wrapped := &ftpConn{idleConn: idleConn{conn}, stop: stop}
		control := firstDial
		firstDial = false
		if protocol == "ftps-implicit" || (protocol == "ftps" && !control) {
			return tls.Client(wrapped, tlsConfig), nil
		}
		return wrapped, nil
	})}
	if protocol == "ftps" {
		options = append(options, ftp.DialWithExplicitTLS(tlsConfig))
	}
	if protocol == "ftps-implicit" {
		options = append(options, ftp.DialWithTLS(tlsConfig))
	}
	c, err := ftp.Dial(net.JoinHostPort(h.Address, strconv.Itoa(h.Port)), options...)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	if err = c.Login(h.Username, h.Password); err != nil {
		cancel()
		c.Quit()
		return nil, nil, err
	}
	return &ftpFS{c}, func() { cancel(); c.Quit() }, nil
}

type ftpConn struct {
	idleConn
	stop func() bool
}

func (c *ftpConn) Close() error { c.stop(); return c.Conn.Close() }

type ftpFS struct{ c *ftp.ServerConn }

func (f *ftpFS) Getwd() (string, error) { return f.c.CurrentDir() }
func (f *ftpFS) RealPath(p string) (string, error) {
	if strings.ContainsAny(p, "\r\n\x00") {
		return "", errors.New("Invalid FTP path")
	}
	if !path.IsAbs(p) {
		home, err := f.Getwd()
		if err != nil {
			return "", err
		}
		p = path.Join(home, p)
	}
	return path.Clean(p), nil
}
func (f *ftpFS) ReadDirContext(ctx context.Context, p string) ([]os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	list, err := f.c.List(p)
	if err != nil {
		return nil, err
	}
	result := make([]os.FileInfo, 0, len(list))
	for _, e := range list {
		if e.Name != "." && e.Name != ".." {
			result = append(result, ftpInfo{e})
		}
	}
	return result, nil
}
func (f *ftpFS) Lstat(p string) (os.FileInfo, error) {
	if path.Clean(p) == "/" {
		return ftpInfo{&ftp.Entry{Name: "/", Type: ftp.EntryTypeFolder}}, nil
	}
	list, err := f.c.List(path.Dir(p))
	if err != nil {
		return nil, err
	}
	for _, e := range list {
		if e.Name == path.Base(p) {
			return ftpInfo{e}, nil
		}
	}
	return nil, &os.PathError{Op: "stat", Path: p, Err: os.ErrNotExist}
}
func (f *ftpFS) Open(p string) (io.ReadCloser, error) { return f.c.Retr(p) }

// STOR has no exclusive-create primitive. Only random staging paths are
// written; publication checks the final destination separately.
func (f *ftpFS) OpenFile(p string, flags int) (io.WriteCloser, error) {
	if _, err := f.Lstat(p); err == nil {
		return nil, os.ErrExist
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	r, w := io.Pipe()
	result := make(chan error, 1)
	go func() { err := f.c.Stor(p, r); r.CloseWithError(err); result <- err }()
	return &ftpWriter{PipeWriter: w, result: result}, nil
}

type ftpWriter struct {
	*io.PipeWriter
	result chan error
	once   sync.Once
	err    error
}

func (w *ftpWriter) Close() error {
	w.once.Do(func() { w.PipeWriter.Close(); w.err = <-w.result })
	return w.err
}
func (f *ftpFS) Mkdir(p string) error           { return f.c.MakeDir(p) }
func (f *ftpFS) Remove(p string) error          { return f.c.Delete(p) }
func (f *ftpFS) RemoveDirectory(p string) error { return f.c.RemoveDir(p) }
func (f *ftpFS) Rename(a, b string) error       { return f.c.Rename(a, b) }

type ftpInfo struct{ e *ftp.Entry }

func (f ftpInfo) Name() string { return f.e.Name }
func (f ftpInfo) Size() int64 {
	if f.e.Size > 1<<63-1 {
		return 1<<63 - 1
	}
	return int64(f.e.Size)
}
func (f ftpInfo) Mode() os.FileMode {
	switch f.e.Type {
	case ftp.EntryTypeFolder:
		return os.ModeDir | 0700
	case ftp.EntryTypeLink:
		return os.ModeSymlink | 0600
	}
	return 0600
}
func (f ftpInfo) ModTime() time.Time { return f.e.Time }
func (f ftpInfo) IsDir() bool        { return f.e.Type == ftp.EntryTypeFolder }
func (f ftpInfo) Sys() any           { return nil }

func validName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\:\r\n\x00") {
		return fmt.Errorf("Unsupported filename: %q", name)
	}
	return nil
}

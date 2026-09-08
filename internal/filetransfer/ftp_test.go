package filetransfer

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"ssh-manager/pkg/model"
)

// A loopback FTP fixture implements only commands used by this client. All
// filesystem operations stay inside t.TempDir; TLS uses a test-only CA.
func testFTP(t *testing.T, root, protocol string) (model.HostInfo, *tls.Config) {
	t.Helper()
	certServer := httptest.NewTLSServer(nil)
	cert := certServer.TLS.Certificates[0]
	certServer.Close()
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	clientTLS := &tls.Config{RootCAs: roots, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}
	serverTLS := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveFTP(conn, root, protocol, serverTLS)
		}
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	number, _ := strconv.Atoi(port)
	return model.HostInfo{Address: host, Port: number, Username: "test", Password: "test-password"}, clientTLS
}
func serveFTP(conn net.Conn, root, protocol string, tlsConfig *tls.Config) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(20 * time.Second))
	if protocol == "ftps-implicit" {
		conn = tls.Server(conn, tlsConfig)
	}
	reader := bufio.NewReader(conn)
	reply := func(code int, text string) { fmt.Fprintf(conn, "%d %s\r\n", code, text) }
	reply(220, "Test FTP")
	var passive net.Listener
	defer func() {
		if passive != nil {
			passive.Close()
		}
	}()
	var renameFrom string
	protected := false
	loggedIn := false
	local := func(p string) string {
		return filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(path.Clean("/"+p), "/")))
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		parts := strings.SplitN(strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), " ", 2)
		cmd := parts[0]
		arg := ""
		if len(parts) > 1 {
			arg = parts[1]
		}
		if !loggedIn && cmd != "USER" && cmd != "PASS" && cmd != "AUTH" && cmd != "QUIT" {
			reply(530, "Login required")
			continue
		}
		switch cmd {
		case "AUTH":
			reply(234, "Start TLS")
			conn = tls.Server(conn, tlsConfig)
			reader = bufio.NewReader(conn)
		case "USER":
			if arg == "test" {
				reply(331, "Password required")
			} else {
				reply(530, "Unknown user")
			}
		case "PASS":
			if arg == "test-password" {
				loggedIn = true
				reply(230, "Logged in")
			} else {
				reply(530, "Wrong password")
			}
		case "FEAT":
			fmt.Fprint(conn, "211-Features\r\n MLST type*;size*;modify*;\r\n UTF8\r\n211 End\r\n")
		case "TYPE", "OPTS", "PBSZ":
			reply(200, "OK")
		case "PROT":
			protected = arg == "P"
			reply(200, "OK")
		case "PWD":
			reply(257, `"/"`)
		case "EPSV":
			if passive != nil {
				passive.Close()
			}
			passive, err = net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return
			}
			reply(229, fmt.Sprintf("Entering Extended Passive Mode (|||%d|)", passive.Addr().(*net.TCPAddr).Port))
		case "MLSD", "LIST", "STOR", "RETR":
			if passive == nil {
				reply(425, "Use EPSV")
				continue
			}
			var file *os.File
			var entries []os.DirEntry
			if cmd == "STOR" {
				file, err = os.Create(local(arg))
			} else if cmd == "RETR" {
				file, err = os.Open(local(arg))
			} else {
				entries, err = os.ReadDir(local(arg))
			}
			if err != nil {
				passive.Close()
				passive = nil
				reply(550, "Unavailable")
				continue
			}
			reply(150, "Data follows")
			data, e := passive.Accept()
			passive.Close()
			passive = nil
			if e != nil {
				if file != nil {
					file.Close()
				}
				return
			}
			data.SetDeadline(time.Now().Add(15 * time.Second))
			if protected {
				data = tls.Server(data, tlsConfig)
			}
			if cmd == "STOR" {
				_, err = io.Copy(file, data)
				file.Close()
			} else if cmd == "RETR" {
				_, err = io.Copy(data, file)
				file.Close()
			} else {
				for _, entry := range entries {
					info, e := entry.Info()
					if e != nil {
						continue
					}
					kind := "file"
					if info.IsDir() {
						kind = "dir"
					}
					fmt.Fprintf(data, "type=%s;size=%d;modify=20260908000000; %s\r\n", kind, info.Size(), entry.Name())
				}
			}
			data.Close()
			if err != nil {
				reply(426, "Transfer failed")
			} else {
				reply(226, "Transfer complete")
			}
		case "MKD":
			if err = os.Mkdir(local(arg), 0700); err != nil {
				reply(550, "Cannot create")
			} else {
				reply(257, `"`+arg+`"`)
			}
		case "DELE", "RMD":
			if err = os.Remove(local(arg)); err != nil {
				reply(550, "Cannot delete")
			} else {
				reply(250, "Deleted")
			}
		case "RNFR":
			if _, err = os.Lstat(local(arg)); err != nil {
				reply(550, "Missing")
			} else {
				renameFrom = local(arg)
				reply(350, "Rename target")
			}
		case "RNTO":
			if renameFrom == "" {
				reply(503, "RNFR required")
			} else if err = os.Rename(renameFrom, local(arg)); err != nil {
				reply(550, "Cannot rename")
			} else {
				reply(250, "Renamed")
			}
			renameFrom = ""
		case "QUIT":
			reply(221, "Goodbye")
			return
		default:
			reply(502, "Not implemented")
		}
	}
}

func TestFTPAndFTPS(t *testing.T) {
	for _, protocol := range []string{"ftp", "ftps", "ftps-implicit"} {
		t.Run(protocol, func(t *testing.T) {
			root, local := t.TempDir(), t.TempDir()
			h, trust := testFTP(t, root, protocol)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			remote, done, err := dialRemote(ctx, h, protocol, trust)
			if err != nil {
				t.Fatal(err)
			}
			defer done()
			if home, err := remote.Getwd(); err != nil || home != "/" {
				t.Fatalf("PWD: %s %v", home, err)
			}
			if err := remote.Mkdir("/folder"); err != nil {
				t.Fatal(err)
			}
			m := New(nil)
			defer m.Close()
			for _, payload := range []string{"", "TLS transfer 한글"} {
				src := filepath.Join(local, "file.txt")
				os.WriteFile(src, []byte(payload), 0600)
				j := &Job{Direction: "upload", Overwrite: true}
				if err := m.copyFile(ctx, localFS{}, remote, src, "/folder/file.txt", h, protocol, j); err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(filepath.Join(root, "folder", "file.txt"))
				if err != nil || string(got) != payload {
					t.Fatalf("upload %q %v", got, err)
				}
				dst := filepath.Join(local, "download.txt")
				j = &Job{Direction: "download", Overwrite: true}
				if err := m.copyFile(ctx, remote, localFS{}, "/folder/file.txt", dst, h, protocol, j); err != nil {
					t.Fatal(err)
				}
				got, err = os.ReadFile(dst)
				if err != nil || string(got) != payload {
					t.Fatalf("download %q %v", got, err)
				}
			}
			if err := remote.Rename("/folder/file.txt", "/folder/renamed.txt"); err != nil {
				t.Fatal(err)
			}
			if err := remote.Remove("/folder/renamed.txt"); err != nil {
				t.Fatal(err)
			}
			if err := remote.RemoveDirectory("/folder"); err != nil {
				t.Fatal(err)
			}
			if protocol != "ftp" {
				if _, done, err := connectRemote(ctx, h, protocol); err == nil {
					done()
					t.Fatal("untrusted TLS certificate accepted")
				}
			}
		})
	}
}

func TestFTPRecursiveTransfer(t *testing.T) {
	root, local, download := t.TempDir(), t.TempDir(), t.TempDir()
	h, _ := testFTP(t, root, "ftp")
	src := filepath.Join(local, "tree")
	os.MkdirAll(filepath.Join(src, "empty"), 0700)
	os.Mkdir(filepath.Join(src, "nested"), 0700)
	os.WriteFile(filepath.Join(src, "nested", "file.txt"), []byte("recursive"), 0600)
	m := New(nil)
	defer m.Close()
	j := &Job{Direction: "upload", Source: src, Destination: "/"}
	if err := m.transferRemote(context.Background(), h, "ftp", j); err != nil {
		t.Fatal(err)
	}
	j = &Job{Direction: "download", Source: "/tree", Destination: download}
	if err := m.transferRemote(context.Background(), h, "ftp", j); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(download, "tree", "nested", "file.txt"))
	if err != nil || string(got) != "recursive" {
		t.Fatalf("recursive: %s %v", got, err)
	}
	if info, err := os.Stat(filepath.Join(download, "tree", "empty")); err != nil || !info.IsDir() {
		t.Fatal("empty folder missing")
	}
}

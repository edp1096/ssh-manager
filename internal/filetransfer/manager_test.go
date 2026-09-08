package filetransfer

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"ssh-manager/pkg/model"
)

func testHost(t *testing.T, root string, acceptedKeys ...ssh.PublicKey) model.HostInfo {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{PasswordCallback: func(c ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
		if c.User() != "test" || string(p) != "test-password" {
			return nil, errors.New("Invalid credentials")
		}
		return nil, nil
	}}
	config.AddHostKey(signer)
	config.PublicKeyCallback = func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		for _, accepted := range acceptedKeys {
			if c.User() == "test" && bytes.Equal(key.Marshal(), accepted.Marshal()) {
				return nil, nil
			}
		}
		return nil, errors.New("Unknown key")
	}
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
			go func() {
				defer conn.Close()
				server, channels, requests, err := ssh.NewServerConn(conn, config)
				if err != nil {
					return
				}
				defer server.Close()
				go ssh.DiscardRequests(requests)
				for incoming := range channels {
					if incoming.ChannelType() != "session" {
						incoming.Reject(ssh.UnknownChannelType, "unsupported")
						continue
					}
					ch, requests, err := incoming.Accept()
					if err != nil {
						return
					}
					go func() {
						defer ch.Close()
						for req := range requests {
							var subsystem struct{ Name string }
							ssh.Unmarshal(req.Payload, &subsystem)
							ok := req.Type == "subsystem" && subsystem.Name == "sftp"
							req.Reply(ok, nil)
							if ok {
								fs, e := sftp.NewServer(ch, sftp.WithServerWorkingDirectory(root))
								if e == nil {
									fs.Serve()
									fs.Close()
								}
								return
							}
						}
					}()
				}
			}()
		}
	}()
	address, port, _ := net.SplitHostPort(listener.Addr().String())
	number, _ := strconv.Atoi(port)
	return model.HostInfo{Address: address, Port: number, Username: "test", Password: "test-password", UniqueID: "test-host", Name: "Test SFTP"}
}

func TestTransfers(t *testing.T) {
	remote, local, download := t.TempDir(), t.TempDir(), t.TempDir()
	h := testHost(t, remote)
	m := New(nil)
	defer m.Close()
	data := bytes.Repeat([]byte("SFTP round-trip \x00 한글\n"), 10000)
	file := filepath.Join(local, "sample 한글.txt")
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	job := &Job{ID: token(), Direction: "upload", Source: file, Destination: filepath.ToSlash(remote)}
	if err := m.transfer(context.Background(), h, job); err != nil {
		t.Fatal(err)
	}
	if job.Bytes != int64(len(data)) {
		t.Fatalf("progress=%d", job.Bytes)
	}
	if err := m.transfer(context.Background(), h, job); err == nil {
		t.Fatal("existing destination overwritten")
	}
	job = &Job{ID: token(), Direction: "download", Source: filepath.ToSlash(filepath.Join(remote, filepath.Base(file))), Destination: download}
	if err := m.transfer(context.Background(), h, job); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(download, filepath.Base(file)))
	if err != nil || !bytes.Equal(data, got) {
		t.Fatalf("round trip mismatch: %v", err)
	}
	if err := m.transfer(context.Background(), h, job); err == nil {
		t.Fatal("existing local destination overwritten")
	}
	job = &Job{ID: token(), Direction: "upload", Source: local, Destination: filepath.ToSlash(remote)}
	if err := m.transfer(context.Background(), h, job); err != nil {
		t.Fatal("directory transfer failed:", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := m.transfer(ctx, h, job); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	h.Password = "wrong"
	if _, _, err := connect(context.Background(), h); err == nil {
		t.Fatal("bad password accepted")
	}
}

func TestAPI(t *testing.T) {
	root := t.TempDir()
	h := testHost(t, root)
	m := New(func(file, id string) (model.HostInfo, error) {
		if id != h.UniqueID {
			return model.HostInfo{}, errors.New("not found")
		}
		return h, nil
	})
	defer m.Close()
	mux := http.NewServeMux()
	m.Register(mux)
	request := func(method, url, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, url, strings.NewReader(body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	w := request("POST", "/files/sessions", `{"hostId":"test-host"}`)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var result map[string]string
	json.Unmarshal(w.Body.Bytes(), &result)
	id := result["id"]
	w = request("GET", "/files/sessions/"+id+"/entries?side=remote&path=.", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"entries":[]`) {
		t.Fatal(w.Body.String())
	}
	w = request("GET", "/files/sessions/"+id+"/entries?side=unknown", "")
	if w.Code == 200 {
		t.Fatal("invalid side accepted")
	}
	w = request("POST", "/files/jobs", fmt.Sprintf(`{"session":%q,"direction":"upload","source":"missing","destination":"."}`, id))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var job Job
	json.Unmarshal(w.Body.Bytes(), &job)
	deadline := time.Now().Add(5 * time.Second)
	for {
		w = request("GET", "/files/jobs", "")
		var list []Job
		json.Unmarshal(w.Body.Bytes(), &list)
		if len(list) == 1 && list[0].Status == "failed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job did not fail:", w.Body.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	request("DELETE", "/files/sessions/"+id, "")
	w = request("GET", "/files/sessions/"+id+"/entries?side=remote", "")
	if w.Code == 200 {
		t.Fatal("closed session usable")
	}
}

func TestCancellationRemovesIncompleteDownload(t *testing.T) {
	remote, local := t.TempDir(), t.TempDir()
	h := testHost(t, remote)
	m := New(nil)
	defer m.Close()
	source := filepath.Join(remote, "large.bin")
	f, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(256 << 20); err != nil {
		t.Fatal(err)
	}
	f.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	j := &Job{ID: token(), Direction: "download", Source: filepath.ToSlash(source), Destination: local}
	finished := make(chan error, 1)
	go func() { finished <- m.transfer(ctx, h, j) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		m.mu.Lock()
		n := j.Bytes
		m.mu.Unlock()
		if n > 0 {
			cancel()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("transfer did not start")
		}
		time.Sleep(time.Millisecond)
	}
	if err := <-finished; err == nil {
		t.Fatal("cancelled transfer succeeded")
	}
	if _, err := os.Stat(filepath.Join(local, "large.bin")); !os.IsNotExist(err) {
		t.Fatalf("incomplete download remains: %v", err)
	}
}

func TestPrivateKeyAuthentication(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	h := testHost(t, t.TempDir(), signer.PublicKey())
	block, err := ssh.MarshalPrivateKey(key, "test")
	if err != nil {
		t.Fatal(err)
	}
	h.PrivateKeyText = string(pem.EncodeToMemory(block))
	h.Password = "wrong"
	c, done, err := connect(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	if _, err = c.Getwd(); err != nil {
		t.Fatal(err)
	}
}

func TestCancellationRemovesIncompleteUpload(t *testing.T) {
	remote, local := t.TempDir(), t.TempDir()
	h := testHost(t, remote)
	m := New(nil)
	defer m.Close()
	source := filepath.Join(local, "large.bin")
	f, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(256 << 20); err != nil {
		t.Fatal(err)
	}
	f.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	j := &Job{ID: token(), Direction: "upload", Source: source, Destination: filepath.ToSlash(remote)}
	finished := make(chan error, 1)
	go func() { finished <- m.transfer(ctx, h, j) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		m.mu.Lock()
		n := j.Bytes
		m.mu.Unlock()
		if n > 0 {
			cancel()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("transfer did not start")
		}
		time.Sleep(time.Millisecond)
	}
	if err := <-finished; err == nil {
		t.Fatal("cancelled transfer succeeded")
	}
	if _, err := os.Stat(filepath.Join(remote, "large.bin")); !os.IsNotExist(err) {
		t.Fatalf("incomplete upload remains: %v", err)
	}
}

func TestQueueOrderAndSessionCancellation(t *testing.T) {
	for _, closeSession := range []bool{false, true} {
		t.Run(fmt.Sprint(closeSession), func(t *testing.T) {
			remote, local := t.TempDir(), t.TempDir()
			h := testHost(t, remote)
			source := filepath.Join(local, "file.txt")
			if err := os.WriteFile(source, []byte("first"), 0600); err != nil {
				t.Fatal(err)
			}
			m := New(nil)
			defer m.Close()
			ctx, cancel := context.WithCancel(context.Background())
			m.sessions["test"] = &session{host: h, ctx: ctx, cancel: cancel}
			gate := make(chan struct{})
			m.tail = gate
			mux := http.NewServeMux()
			m.Register(mux)
			for range 2 {
				body, _ := json.Marshal(Job{Session: "test", Direction: "upload", Source: source, Destination: filepath.ToSlash(remote)})
				w := httptest.NewRecorder()
				mux.ServeHTTP(w, httptest.NewRequest("POST", "/files/jobs", bytes.NewReader(body)))
				if w.Code != 200 {
					t.Fatal(w.Body.String())
				}
			}
			if closeSession {
				m.Close()
			}
			close(gate)
			deadline := time.Now().Add(5 * time.Second)
			for {
				m.mu.Lock()
				first, second := m.jobs[0].Status, m.jobs[1].Status
				m.mu.Unlock()
				if (!closeSession && first == "completed" && second == "failed") || (closeSession && first == "cancelled" && second == "cancelled") {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("statuses: %s, %s", first, second)
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}

// Opt-in browser harness: serves the real file API with an isolated test host.
func TestClearHistoryIsSessionScoped(t *testing.T) {
	m := New(nil)
	defer m.Close()
	m.jobs = []*Job{{ID: "a-done", Session: "a", Status: "completed"}, {ID: "b-done", Session: "b", Status: "completed"}, {ID: "a-running", Session: "a", Status: "running"}}
	mux := http.NewServeMux()
	m.Register(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("DELETE", "/files/jobs", nil))
	if w.Code == 200 || len(m.jobs) != 3 {
		t.Fatal("Unscoped clear must not remove history")
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("DELETE", "/files/jobs?session=a", nil))
	if w.Code != 200 || len(m.jobs) != 2 || m.jobs[0].ID != "b-done" || m.jobs[1].ID != "a-running" {
		t.Fatal("Clear affected another session or an active transfer")
	}
}

func TestBrowserHarness(t *testing.T) {
	if os.Getenv("SFTP_BROWSER_HARNESS") != "1" {
		t.Skip("browser harness")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "remote.txt"), []byte("remote test"), 0600)
	h := testHost(t, root)
	m := New(func(file, id string) (model.HostInfo, error) { return h, nil })
	defer m.Close()
	mux := http.NewServeMux()
	m.Register(mux)
	mux.Handle("/", http.FileServer(http.Dir("../../web")))
	ftpRoot := t.TempDir()
	ftpHost, _ := testFTP(t, ftpRoot, "ftp")
	mux.HandleFunc("GET /__test/ftp", func(w http.ResponseWriter, r *http.Request) {
		respond(w, Connection{Protocol: "ftp", Address: ftpHost.Address, Port: ftpHost.Port, Username: ftpHost.Username, Password: ftpHost.Password}, nil)
	})
	stop := make(chan struct{}, 1)
	mux.HandleFunc("POST /__test/stop", func(w http.ResponseWriter, r *http.Request) {
		select {
		case stop <- struct{}{}:
		default:
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	fmt.Println("BROWSER_HARNESS_URL=" + server.URL)
	select {
	case <-stop:
	case <-time.After(90 * time.Second):
	}
}

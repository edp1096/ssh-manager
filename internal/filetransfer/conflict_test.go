package filetransfer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecursiveConflictChoices(t *testing.T) {
	for _, direction := range []string{"upload", "download"} {
		t.Run(direction, func(t *testing.T) {
			local, remote := t.TempDir(), t.TempDir()
			source, dest := local, remote
			if direction == "download" {
				source, dest = remote, local
			}
			for _, root := range []string{source, dest} {
				if err := os.MkdirAll(filepath.Join(root, "folder", "nested"), 0700); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"a", "nested/b"} {
					data := "old"
					if root == source {
						data = "replacement"
					}
					if err := os.WriteFile(filepath.Join(root, "folder", name), []byte(data), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			h := testHost(t, remote)
			m := New(nil)
			defer m.Close()
			ctx, cancel := context.WithCancel(context.Background())
			m.sessions["s"] = &session{host: h, protocol: "sftp", ctx: ctx, cancel: cancel}
			j := &Job{Session: "s", Direction: direction, Source: filepath.Join(source, "folder"), Destination: dest, Interactive: true, choices: map[string]resolution{}}
			var first *conflictError
			if err := m.transferRemote(ctx, h, "sftp", j); !errors.As(err, &first) {
				t.Fatalf("expected first conflict: %v", err)
			}
			// This file is skipped; a later apply-to-session overwrite must not
			// silently replace the earlier explicit file-only choice.
			j.choices[first.Conflict.Destination.Path] = resolution{Action: "skip", destination: first.Conflict.Destination}
			var second *conflictError
			if err := m.transferRemote(ctx, h, "sftp", j); !errors.As(err, &second) {
				t.Fatalf("expected nested conflict: %v", err)
			}
			if first.Conflict.Destination.Path == second.Conflict.Destination.Path {
				t.Fatal("same conflict repeated")
			}
			m.sessions["s"].policy = "overwrite"
			if err := m.transferRemote(ctx, h, "sftp", j); err != nil {
				t.Fatal(err)
			}
			old, _ := os.ReadFile(first.Conflict.Destination.Path)
			replaced, _ := os.ReadFile(second.Conflict.Destination.Path)
			if string(old) != "old" || string(replaced) != "replacement" || j.Skipped != 1 {
				t.Fatalf("unexpected result: %s / %s / %d", old, replaced, j.Skipped)
			}
		})
	}
}

func TestInteractiveConflicts(t *testing.T) {
	for _, scope := range []string{"file", "batch", "session"} {
		for _, action := range []string{"skip", "overwrite"} {
			t.Run(scope+"/"+action, func(t *testing.T) {
				local, remote := t.TempDir(), t.TempDir()
				h := testHost(t, remote)
				m := New(nil)
				defer m.Close()
				ctx, cancel := context.WithCancel(context.Background())
				m.sessions["s"] = &session{host: h, protocol: "sftp", ctx: ctx, cancel: cancel}
				mux := http.NewServeMux()
				m.Register(mux)
				request := func(method, url string, body any) *httptest.ResponseRecorder {
					b, _ := json.Marshal(body)
					w := httptest.NewRecorder()
					mux.ServeHTTP(w, httptest.NewRequest(method, url, bytes.NewReader(b)))
					return w
				}
				jobs := func() []Job {
					var list []Job
					if err := json.Unmarshal(request("GET", "/files/jobs", nil).Body.Bytes(), &list); err != nil {
						t.Fatal(err)
					}
					return list
				}
				wait := func(id, status string) Job {
					t.Helper()
					deadline := time.Now().Add(5 * time.Second)
					for time.Now().Before(deadline) {
						for _, j := range jobs() {
							if j.ID == id && j.Status == status {
								return j
							}
						}
						time.Sleep(time.Millisecond)
					}
					t.Fatalf("waiting for %s: %+v", status, jobs())
					return Job{}
				}
				start := func(name, batch string) Job {
					t.Helper()
					src, dst := filepath.Join(local, name), filepath.Join(remote, name)
					if err := os.WriteFile(src, []byte("new content"), 0600); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(dst, []byte("old"), 0600); err != nil {
						t.Fatal(err)
					}
					w := request("POST", "/files/jobs", Job{Session: "s", Direction: "upload", Source: src, Destination: remote, Interactive: true, Batch: batch})
					if w.Code != 200 {
						t.Fatal(w.Body.String())
					}
					var j Job
					json.Unmarshal(w.Body.Bytes(), &j)
					return j
				}
				resolve := func(j Job, choiceScope string) {
					t.Helper()
					w := request("POST", "/files/jobs/"+j.ID+"/resolve", resolution{ConflictID: j.Conflict.ID, Action: action, Scope: choiceScope})
					if w.Code != 200 {
						t.Fatal(w.Body.String())
					}
					if request("POST", "/files/jobs/"+j.ID+"/resolve", resolution{ConflictID: j.Conflict.ID, Action: action, Scope: choiceScope}).Code == 200 {
						t.Fatal("duplicate accepted")
					}
				}
				a := start("a", "batch1")
				pending := wait(a.ID, "waiting")
				if pending.Conflict.Source.Size != 11 || pending.Conflict.Destination.Size != 3 {
					t.Fatal("missing conflict metadata")
				}
				request("DELETE", "/files/jobs?session=s", nil)
				if len(jobs()) != 1 {
					t.Fatal("clear removed waiting job")
				}
				b := start("b", "batch1")
				resolve(pending, scope)
				wait(a.ID, "completed")
				if scope == "file" {
					resolve(wait(b.ID, "waiting"), "file")
				}
				result := wait(b.ID, "completed")
				expected := "new content"
				if action == "skip" {
					expected = "old"
					if result.Skipped != 1 || result.Bytes != 0 {
						t.Fatal("skip accounting", result)
					}
				}
				for _, name := range []string{"a", "b"} {
					got, _ := os.ReadFile(filepath.Join(remote, name))
					if string(got) != expected {
						t.Fatalf("%s: %s", name, got)
					}
				}
				c := start("c", "batch2")
				if scope != "session" {
					resolve(wait(c.ID, "waiting"), "file")
				}
				wait(c.ID, "completed")
				// A fresh connection must ask again, even on the same host.
				request("DELETE", "/files/sessions/s", nil)
				ctx2, cancel2 := context.WithCancel(context.Background())
				m.mu.Lock()
				m.sessions["s"] = &session{host: h, protocol: "sftp", ctx: ctx2, cancel: cancel2}
				m.mu.Unlock()
				d := start("d", "batch3")
				wait(d.ID, "waiting")
				request("DELETE", "/files/sessions/s", nil)
				wait(d.ID, "cancelled")
			})
		}
	}
}

func TestConflictCancelBatch(t *testing.T) {
	m := New(nil)
	defer m.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	j := &Job{ID: "one", Session: "s", Batch: "b", cancel: cancel, choices: map[string]resolution{}}
	m.jobs = []*Job{j, {ID: "two", Session: "s", Batch: "b", cancel: cancel2}}
	result := make(chan error, 1)
	go func() { result <- m.waitConflict(ctx, j, &Conflict{ID: "conflict"}) }()
	deadline := time.Now().Add(time.Second)
	for {
		m.mu.Lock()
		ready := j.Status == "waiting"
		m.mu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("not waiting")
		}
		time.Sleep(time.Millisecond)
	}
	mux := http.NewServeMux()
	m.Register(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/files/jobs/one/resolve", bytes.NewBufferString(`{"conflictId":"conflict","action":"cancel","scope":"file"}`)))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if err := <-result; err != context.Canceled {
		t.Fatal(err)
	}
	if ctx2.Err() != context.Canceled {
		t.Fatal("queued batch sibling not cancelled")
	}
}

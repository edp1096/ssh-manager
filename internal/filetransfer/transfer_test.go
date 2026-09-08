package filetransfer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type failingRenameFS struct {
	filesystem
	fail func(string, string) bool
}

func (f failingRenameFS) Rename(a, b string) error {
	if f.fail(a, b) {
		return errors.New("injected rename failure")
	}
	return f.filesystem.Rename(a, b)
}

func TestPublishRollback(t *testing.T) {
	for _, restoreFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "restored", true: "backup-retained"}[restoreFails], func(t *testing.T) {
			root := t.TempDir()
			dest, temp := filepath.Join(root, "original"), filepath.Join(root, "staged")
			os.WriteFile(dest, []byte("old"), 0600)
			os.WriteFile(temp, []byte("new"), 0600)
			fs := failingRenameFS{localFS{}, func(from, to string) bool {
				return from == temp || (restoreFails && strings.HasSuffix(from, ".backup"))
			}}
			err := publish(fs, temp, dest, true)
			if err == nil {
				t.Fatal("expected publication failure")
			}
			if !restoreFails {
				got, e := os.ReadFile(dest)
				if e != nil || string(got) != "old" {
					t.Fatalf("original not restored: %q %v", got, e)
				}
			} else {
				if !strings.Contains(err.Error(), "original file remains at") {
					t.Fatal("missing recovery path:", err)
				}
				entries, _ := os.ReadDir(root)
				found := false
				for _, entry := range entries {
					if strings.HasSuffix(entry.Name(), ".backup") {
						got, _ := os.ReadFile(filepath.Join(root, entry.Name()))
						found = string(got) == "old"
					}
				}
				if !found {
					t.Fatal("original backup missing")
				}
			}
		})
	}
}

func TestCancelledOverwriteKeepsOriginal(t *testing.T) {
	for _, direction := range []string{"upload", "download"} {
		t.Run(direction, func(t *testing.T) {
			remote, local := t.TempDir(), t.TempDir()
			h := testHost(t, remote)
			m := New(nil)
			defer m.Close()
			source, destination := local, remote
			if direction == "download" {
				source, destination = remote, local
			}
			file := filepath.Join(source, "large.bin")
			f, err := os.Create(file)
			if err != nil {
				t.Fatal(err)
			}
			f.Truncate(256 << 20)
			f.Close()
			original := filepath.Join(destination, "large.bin")
			os.WriteFile(original, []byte("keep original"), 0600)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			j := &Job{Direction: direction, Source: file, Destination: destination, Overwrite: true}
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
				t.Fatal("expected cancellation")
			}
			got, err := os.ReadFile(original)
			if err != nil || string(got) != "keep original" {
				t.Fatalf("original damaged: %q %v", got, err)
			}
			entries, _ := os.ReadDir(destination)
			if len(entries) != 1 {
				t.Fatalf("staging files remain: %v", entries)
			}
		})
	}
}

func TestRecursiveRejectsSymlinkBeforeWriting(t *testing.T) {
	local, remote := t.TempDir(), t.TempDir()
	h := testHost(t, remote)
	m := New(nil)
	defer m.Close()
	if err := os.Symlink(local, filepath.Join(local, "loop")); err != nil {
		t.Skip(err)
	}
	j := &Job{Direction: "upload", Source: local, Destination: remote}
	if err := m.transfer(context.Background(), h, j); err == nil {
		t.Fatal("symlink accepted")
	}
	entries, _ := os.ReadDir(remote)
	if len(entries) != 0 {
		t.Fatal("destination modified during failed scan")
	}
}

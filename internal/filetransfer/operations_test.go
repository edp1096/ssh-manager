package filetransfer

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"testing"
)

func TestRecursiveDelete(t *testing.T) {
	for _, protocol := range []string{"local", "sftp", "ftp"} {
		t.Run(protocol, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "folder")
			if err := os.MkdirAll(filepath.Join(target, "nested", "empty"), 0700); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(target, "nested", "data.txt")
			if err := os.WriteFile(file, []byte("delete fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(root, "keep.txt")
			if err := os.WriteFile(outside, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(root, filepath.Join(target, "outside-link")); err != nil {
				t.Log("symlink fixture unavailable:", err)
			}
			var fs filesystem = localFS{}
			join := filepath.Join
			if protocol != "local" {
				h := testHost(t, root)
				if protocol == "ftp" {
					h, _ = testFTP(t, root, "ftp")
					target = "/folder"
				}
				var done func()
				var err error
				fs, done, err = connectRemote(context.Background(), h, protocol)
				if err != nil {
					t.Fatal(err)
				}
				defer done()
				join = path.Join
			}
			if err := deleteEntry(context.Background(), fs, target, join, false); err == nil {
				t.Fatal("non-recursive deletion accepted non-empty folder")
			}
			if _, err := os.Stat(file); err != nil {
				t.Fatal("unconfirmed contents were deleted")
			}
			if err := deleteEntry(context.Background(), fs, "/", join, true); err == nil {
				t.Fatal("root deletion accepted")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := deleteEntry(ctx, fs, target, join, true); err == nil {
				t.Fatal("cancelled deletion succeeded")
			}
			if err := deleteEntry(context.Background(), fs, target, join, true); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(filepath.Join(root, "folder")); !os.IsNotExist(err) {
				t.Fatalf("folder remains: %v", err)
			}
			if got, err := os.ReadFile(outside); err != nil || string(got) != "keep" {
				t.Fatalf("link target changed: %s %v", got, err)
			}
		})
	}
}

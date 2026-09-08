package filetransfer

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
)

// Remote paths always use slash separators. Local paths use filepath.
type filesystem interface {
	Getwd() (string, error)
	RealPath(string) (string, error)
	ReadDirContext(context.Context, string) ([]os.FileInfo, error)
	Lstat(string) (os.FileInfo, error)
	Open(string) (io.ReadCloser, error)
	OpenFile(string, int) (io.WriteCloser, error)
	Mkdir(string) error
	Remove(string) error
	RemoveDirectory(string) error
	Rename(string, string) error
}

type sftpFS struct{ *sftp.Client }

func (s sftpFS) Open(p string) (io.ReadCloser, error) { return s.Client.Open(p) }
func (s sftpFS) OpenFile(p string, flags int) (io.WriteCloser, error) {
	return s.Client.OpenFile(p, flags)
}

type localFS struct{}

func (localFS) Getwd() (string, error)            { return os.Getwd() }
func (localFS) RealPath(p string) (string, error) { return filepath.Abs(p) }
func (localFS) ReadDirContext(ctx context.Context, p string) ([]os.FileInfo, error) {
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	result := make([]os.FileInfo, 0, len(entries))
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		result = append(result, info)
	}
	return result, nil
}
func (localFS) Lstat(p string) (os.FileInfo, error)  { return os.Lstat(p) }
func (localFS) Open(p string) (io.ReadCloser, error) { return os.Open(p) }
func (localFS) OpenFile(p string, flags int) (io.WriteCloser, error) {
	return os.OpenFile(p, flags, 0600)
}
func (localFS) Mkdir(p string) error           { return os.Mkdir(p, 0700) }
func (localFS) Remove(p string) error          { return os.Remove(p) }
func (localFS) RemoveDirectory(p string) error { return os.Remove(p) }
func (localFS) Rename(a, b string) error       { return os.Rename(a, b) }

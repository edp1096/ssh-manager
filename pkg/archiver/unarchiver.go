package archiver

import (
	"embed"
	"os"
)

type FileSystem interface {
	ReadFile(name string) ([]byte, error)
}

type EmbedFS struct {
	fs embed.FS
}

func (emb EmbedFS) ReadFile(name string) ([]byte, error) {
	return emb.fs.ReadFile(name)
}

type RealFS struct{}

func (RealFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

type Archiver interface {
	UnArchive() error
}

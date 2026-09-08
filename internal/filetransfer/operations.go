package filetransfer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

func (m *Manager) operation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Side      string `json:"side"`
		Action    string `json:"action"`
		Path      string `json:"path"`
		Name      string `json:"name"`
		Recursive bool   `json:"recursive"`
	}
	if err := decode(w, r, &req); err != nil {
		respond(w, nil, err)
		return
	}
	s, err := m.get(r.PathValue("id"))
	if err != nil {
		respond(w, nil, err)
		return
	}
	if req.Side != "local" && req.Side != "remote" {
		respond(w, nil, errors.New("Invalid filesystem side"))
		return
	}
	remote := req.Side == "remote"
	if !validAbsolute(req.Path, remote) {
		respond(w, nil, errors.New("An absolute path is required"))
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	var fs filesystem = localFS{}
	join := filepath.Join
	if remote {
		var done func()
		fs, done, err = connectRemote(ctx, s.host, s.protocol)
		if err != nil {
			respond(w, nil, err)
			return
		}
		defer done()
		join = path.Join
	}
	switch req.Action {
	case "mkdir":
		if err = validName(req.Name); err == nil {
			err = fs.Mkdir(join(req.Path, req.Name))
		}
	case "rename":
		if err = validName(req.Name); err == nil {
			if join(req.Path, "..") == join(req.Path) {
				err = errors.New("Cannot rename a filesystem root")
				break
			}
			destination := join(req.Path, "..", req.Name)
			if _, e := fs.Lstat(destination); e == nil {
				err = errors.New("Destination already exists")
			} else if !os.IsNotExist(e) {
				err = e
			} else {
				err = fs.Rename(req.Path, destination)
			}
		}
	case "delete":
		err = deleteEntry(ctx, fs, req.Path, join, req.Recursive)
	default:
		err = errors.New("Unknown file operation")
	}
	respond(w, map[string]bool{"ok": err == nil}, err)
}

// Scan before deleting, never follow links, and require explicit recursive
// intent. An interrupted deletion is not transactional; report partial work.
func deleteEntry(ctx context.Context, fs filesystem, target string, join func(...string) string, recursive bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	target = join(target)
	if join(target, "..") == target {
		return errors.New("Cannot delete a filesystem root")
	}
	if err := safeDirectory(fs, join(target, ".."), join); err != nil {
		return err
	}
	type entry struct {
		path      string
		directory bool
	}
	items := []entry{}
	count := 0
	var scan func(string, int) error
	scan = func(p string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		count++
		if depth > 128 || count > 100000 {
			return errors.New("Deletion exceeds the 128-level / 100000-entry limit")
		}
		info, err := fs.Lstat(p)
		if err != nil {
			return err
		}
		directory := info.IsDir() && info.Mode()&os.ModeSymlink == 0
		if directory {
			children, err := fs.ReadDirContext(ctx, p)
			if err != nil {
				return err
			}
			if len(children) > 0 && !recursive {
				return errors.New("Folder is not empty; deletion requires confirmation to include its contents")
			}
			for _, child := range children {
				if err := validName(child.Name()); err != nil {
					return err
				}
				if err := scan(join(p, child.Name()), depth+1); err != nil {
					return err
				}
			}
		}
		items = append(items, entry{p, directory})
		return nil
	}
	if err := scan(target, 0); err != nil {
		return fmt.Errorf("Cannot delete %q: %w", target, err)
	}
	for _, item := range items {
		err := ctx.Err()
		if err == nil {
			err = safeDirectory(fs, join(item.path, ".."), join)
		}
		if err == nil {
			var info os.FileInfo
			info, err = fs.Lstat(item.path)
			if err == nil && (info.IsDir() && info.Mode()&os.ModeSymlink == 0) != item.directory {
				err = errors.New("Entry type changed during deletion")
			}
			if err == nil {
				if item.directory {
					err = fs.RemoveDirectory(item.path)
				} else {
					err = fs.Remove(item.path)
				}
			}
		}
		if err != nil {
			return fmt.Errorf("Deletion stopped at %q; some entries may already have been removed: %w", item.path, err)
		}
	}
	return nil
}

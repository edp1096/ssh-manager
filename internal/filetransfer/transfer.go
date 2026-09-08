package filetransfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"ssh-manager/pkg/model"
)

type incompleteFileError struct {
	path string
	err  error
}

func (e *incompleteFileError) Error() string {
	return fmt.Sprintf("Temporary file may remain at %s: %v", e.path, e.err)
}

func (m *Manager) transfer(ctx context.Context, h model.HostInfo, j *Job) error {
	return m.transferRemote(ctx, h, "sftp", j)
}
func (m *Manager) transferRemote(ctx context.Context, h model.HostInfo, protocol string, j *Job) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	remote, done, err := connectRemote(ctx, h, protocol)
	if err != nil {
		return err
	}
	defer done()
	var source, target filesystem = localFS{}, remote
	sourceJoin, targetJoin := filepath.Join, path.Join
	name := filepath.Base(j.Source)
	if j.Direction == "download" {
		source, target = remote, localFS{}
		sourceJoin, targetJoin = path.Join, filepath.Join
		name = path.Base(j.Source)
	}
	if err := validName(name); err != nil {
		return err
	}
	if err := safeDirectory(target, j.Destination, targetJoin); err != nil {
		return err
	}
	type item struct {
		source, dest string
		info         os.FileInfo
	}
	items := []item{}
	var total int64
	var scan func(string, string, int) error
	scan = func(src, dst string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if depth > 128 || len(items) >= 100000 {
			return errors.New("Folder exceeds the 128-level / 100000-entry transfer limit")
		}
		info, err := source.Lstat(src)
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("Symbolic links and special files are not transferred: %s", src)
		}
		items = append(items, item{src, dst, info})
		if !info.IsDir() {
			if info.Size() < 0 || total > int64(^uint64(0)>>1)-info.Size() {
				return errors.New("Transfer size overflow")
			}
			total += info.Size()
			return nil
		}
		entries, err := source.ReadDirContext(ctx, src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := validName(entry.Name()); err != nil {
				return err
			}
			if err := scan(sourceJoin(src, entry.Name()), targetJoin(dst, entry.Name()), depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := scan(j.Source, targetJoin(j.Destination, name), 0); err != nil {
		return err
	}
	m.mu.Lock()
	j.Total = total
	m.mu.Unlock()
	// Preflight known conflicts before changing anything. Recheck on publication.
	missingDirs := map[string]bool{}
	decisions := map[string]string{}
	for _, it := range items {
		if missingDirs[targetJoin(it.dest, "..")] {
			if it.info.IsDir() {
				missingDirs[it.dest] = true
			}
			continue
		}
		info, err := target.Lstat(it.dest)
		if err != nil {
			if os.IsNotExist(err) {
				if it.info.IsDir() {
					missingDirs[it.dest] = true
				}
				continue
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || info.IsDir() != it.info.IsDir() {
			return fmt.Errorf("Destination type conflict: %s", it.dest)
		}
		if !info.IsDir() {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("Cannot overwrite special file: %s", it.dest)
			}
			if j.Interactive {
				action := m.conflictAction(j, it.dest, info)
				if action == "" {
					return &conflictError{&Conflict{ID: token(), Source: fileEntry(it.source, it.info), Destination: fileEntry(it.dest, info)}}
				}
				decisions[it.dest] = action
			} else if !j.Overwrite {
				return fmt.Errorf("Destination exists: %s (enable Overwrite to replace it)", it.dest)
			}
		}
	}
	for _, it := range items {
		if err := ctx.Err(); err != nil {
			return err
		}
		if decisions[it.dest] == "skip" {
			m.mu.Lock()
			j.Skipped++
			j.Total -= it.info.Size()
			m.mu.Unlock()
			continue
		}
		if it.info.IsDir() {
			info, err := target.Lstat(it.dest)
			if os.IsNotExist(err) {
				err = target.Mkdir(it.dest)
			} else if err == nil && !info.IsDir() {
				err = fmt.Errorf("Destination is not a directory: %s", it.dest)
			}
			if err != nil {
				return err
			}
			continue
		}
		overwrite := j.Overwrite
		if j.Interactive {
			overwrite = decisions[it.dest] == "overwrite"
		}
		if err := m.copyFileOverwrite(ctx, source, target, it.source, it.dest, h, protocol, j, overwrite); err != nil {
			return err
		}
	}
	return nil
}

// Refuse destination symlink ancestors rather than inadvertently writing
// outside a selected directory during recursive transfers.
func safeDirectory(fs filesystem, p string, join func(...string) string) error {
	clean := join(p)
	for {
		info, err := fs.Lstat(clean)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("Destination is not a regular directory: %s", clean)
		}
		parent := join(clean, "..")
		if parent == clean || clean == "." {
			return nil
		}
		clean = parent
	}
}

func (m *Manager) copyFile(ctx context.Context, source, target filesystem, src, dest string, h model.HostInfo, protocol string, j *Job) (result error) {
	return m.copyFileOverwrite(ctx, source, target, src, dest, h, protocol, j, j.Overwrite)
}
func (m *Manager) copyFileOverwrite(ctx context.Context, source, target filesystem, src, dest string, h model.HostInfo, protocol string, j *Job, overwrite bool) (result error) {
	reader, err := source.Open(src)
	if err != nil {
		return err
	}
	defer reader.Close()
	temp := sibling(target, dest, ".part")
	writer, err := target.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		writer.Close()
		if committed {
			return
		}
		cleanup := target
		done := func() {}
		if j.Direction == "upload" {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var err error
			cleanup, done, err = connectRemote(cleanupCtx, h, protocol)
			if err != nil {
				result = errors.Join(result, &incompleteFileError{temp, err})
				return
			}
		}
		defer done()
		if err := cleanup.Remove(temp); err != nil && !os.IsNotExist(err) {
			result = errors.Join(result, &incompleteFileError{temp, err})
		}
	}()
	buf := make([]byte, 128<<10)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := reader.Read(buf)
		if n > 0 {
			written, writeErr := writer.Write(buf[:n])
			m.mu.Lock()
			j.Bytes += int64(written)
			m.mu.Unlock()
			if writeErr != nil {
				return writeErr
			}
			if written != n {
				return io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	// FTP sends its completion reply on Close, so both streams must close
	// before issuing subsequent commands on the same control connection.
	if err := reader.Close(); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := publish(target, temp, dest, overwrite); err != nil {
		return err
	}
	committed = true
	return nil
}

func publish(fs filesystem, temp, dest string, overwrite bool) error {
	info, err := fs.Lstat(dest)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		if !overwrite {
			return fmt.Errorf("Destination exists: %s", dest)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Cannot overwrite directory, link or special file: %s", dest)
		}
		if s, ok := fs.(sftpFS); ok {
			if _, supported := s.HasExtension("posix-rename@openssh.com"); supported {
				return s.PosixRename(temp, dest)
			}
		}
		if _, ok := fs.(localFS); ok {
			return fs.Rename(temp, dest)
		}
		// FTP has no portable atomic replace. Keep the original as a backup
		// until the staged file is published, and restore it if publication fails.
		backup := sibling(fs, dest, ".backup")
		if err := fs.Rename(dest, backup); err != nil {
			return err
		}
		if err := fs.Rename(temp, dest); err != nil {
			if restoreErr := fs.Rename(backup, dest); restoreErr != nil {
				return fmt.Errorf("Publish failed: %v; original file remains at %s (restore failed: %v)", err, backup, restoreErr)
			}
			return err
		}
		if err := fs.Remove(backup); err != nil {
			return fmt.Errorf("Transfer published, but old backup remains at %s: %w", backup, err)
		}
		return nil
	}
	if _, ok := fs.(localFS); ok {
		// Link is an atomic no-replace operation, unlike a stat+rename pair.
		if err := os.Link(temp, dest); err != nil {
			return err
		}
		return os.Remove(temp)
	}
	return fs.Rename(temp, dest)
}

func validAbsolute(p string, remote bool) bool {
	if strings.ContainsAny(p, "\r\n\x00") {
		return false
	}
	if remote {
		return path.IsAbs(p)
	}
	return filepath.IsAbs(p)
}

func sibling(fs filesystem, dest, suffix string) string {
	name := ".ssh-manager-" + token() + suffix
	if _, ok := fs.(localFS); ok {
		return filepath.Join(filepath.Dir(dest), name)
	}
	return path.Join(path.Dir(dest), name)
}

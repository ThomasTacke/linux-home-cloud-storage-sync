package linker

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// copyTree copies the file or folder src to dst, which must not exist. It keeps
// permission bits (e.g. 0600 for keys, 0700 for ~/.ssh) and modification times, and
// copies symlinks as symlinks. Anything else (sockets, devices) is an error.
func copyTree(src, dst string) error {
	type dir struct {
		path    string
		perm    fs.FileMode
		modTime time.Time
	}
	// Folders are created writable for us and get their real permissions and times at
	// the end, deepest first: writing their content would change them otherwise
	var dirs []dir

	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		to := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}

		switch mode := info.Mode(); {
		case mode&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			// No Chtimes here: it would follow the link and touch its destination
			return os.Symlink(link, to)
		case mode.IsDir():
			if err := os.Mkdir(to, 0o700); err != nil {
				return err
			}
			dirs = append(dirs, dir{to, mode.Perm(), info.ModTime()})
			return nil
		case mode.IsRegular():
			if err := copyFile(path, to, mode.Perm()); err != nil {
				return err
			}
			return os.Chtimes(to, info.ModTime(), info.ModTime())
		default:
			return fmt.Errorf("%s: can't copy %s", path, mode.Type())
		}
	})
	if err != nil {
		return err
	}

	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Chmod(dirs[i].path, dirs[i].perm); err != nil {
			return err
		}
		if err := os.Chtimes(dirs[i].path, dirs[i].modTime, dirs[i].modTime); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(from, to string, perm fs.FileMode) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(to, perm) // exact bits, independent of the umask
}

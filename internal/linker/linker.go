// Package linker handles one symlinkMap entry: link target -> source, and, only when
// asked for, move a local copy into the cloud (adopt), set an existing one aside (force)
// or remove the link again (unlink). It never deletes real files or folders.
package linker

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// State is what Link or Unlink found at, or did to, a target.
type State int

const (
	// SourceMissing: the cloud copy doesn't exist.
	SourceMissing State = iota
	// Created: the symlink was created.
	Created
	// WouldCreate: dry run, the symlink would be created.
	WouldCreate
	// AlreadyLinked: the target is a symlink to the source.
	AlreadyLinked
	// LinkedElsewhere: the target is a symlink to something else that exists.
	LinkedElsewhere
	// BrokenLink: the target is a symlink to something that doesn't exist.
	BrokenLink
	// ExistsDir: the target is a real directory.
	ExistsDir
	// ExistsFile: the target is a real file.
	ExistsFile
	// ExistsOther: the target is something else (socket, device, ...); never touched.
	ExistsOther
	// Adopted: the local file/folder was moved into the cloud and linked (--adopt).
	Adopted
	// WouldAdopt: dry run of Adopted.
	WouldAdopt
	// Replaced: a broken link or a link to elsewhere was replaced (--force).
	Replaced
	// WouldReplace: dry run of Replaced.
	WouldReplace
	// BackedUp: the local file/folder was renamed to Result.Backup and linked (--force).
	BackedUp
	// WouldBackUp: dry run of BackedUp.
	WouldBackUp
	// Removed: the link to the source was removed (Unlink).
	Removed
	// WouldRemove: dry run of Removed.
	WouldRemove
	// Restored: the link was replaced by a copy of the source (Unlink with Restore).
	Restored
	// WouldRestore: dry run of Restored.
	WouldRestore
	// NothingThere: Unlink found nothing at the target.
	NothingThere
	// Failed: checking or changing failed, see Result.Err.
	Failed
)

// Options for Link.
type Options struct {
	DryRun bool
	// Adopt moves a local file/folder into the cloud when the cloud has none yet.
	Adopt bool
	// Force replaces broken links and links to elsewhere, and renames a local
	// file/folder to <name>.backup-<timestamp> when the cloud already has one.
	Force bool
	// Now is the clock for backup names; time.Now when nil.
	Now func() time.Time
}

// UnlinkOptions for Unlink.
type UnlinkOptions struct {
	DryRun bool
	// Restore replaces the link with a copy of the source instead of just removing it.
	Restore bool
}

// Result describes one entry.
type Result struct {
	State  State
	Source string
	Target string
	// PointsTo is where an existing symlink at Target pointed (absolute).
	PointsTo string
	// Backup is where --force put the local copy.
	Backup string
	Err    error
}

func (r Result) fail(err error) Result {
	r.State, r.Err = Failed, err
	return r
}

// Link makes target a symlink to source. Without Adopt or Force it changes nothing that
// already exists.
func Link(source, target string, opts Options) Result {
	r := Result{Source: source, Target: target}

	info, targetErr := os.Lstat(target)
	if targetErr != nil && !errors.Is(targetErr, fs.ErrNotExist) {
		return r.fail(targetErr)
	}
	targetExists := targetErr == nil

	if _, err := os.Stat(source); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return r.fail(err)
		}
		if opts.Adopt && targetExists && (info.Mode().IsDir() || info.Mode().IsRegular()) {
			return adopt(r, opts.DryRun)
		}
		r.State = SourceMissing
		return r
	}

	if !targetExists {
		if opts.DryRun {
			r.State = WouldCreate
			return r
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return r.fail(err)
		}
		if err := os.Symlink(source, target); err != nil {
			return r.fail(err)
		}
		r.State = Created
		return r
	}

	r = existing(r, info)
	if !opts.Force {
		return r
	}
	switch r.State {
	case BrokenLink, LinkedElsewhere:
		return replaceLink(r, opts.DryRun)
	case ExistsDir, ExistsFile:
		now := time.Now
		if opts.Now != nil {
			now = opts.Now
		}
		return backUp(r, now(), opts.DryRun)
	}
	return r
}

// existing classifies what is at r.Target.
func existing(r Result, info fs.FileInfo) Result {
	switch mode := info.Mode(); {
	case mode&fs.ModeSymlink != 0:
		pointsTo, err := pointsTo(r.Target)
		if err != nil {
			return r.fail(err)
		}
		r.PointsTo = pointsTo
		resolvedTarget, err := filepath.EvalSymlinks(r.Target)
		if err != nil {
			r.State = BrokenLink
			return r
		}
		resolvedSource, err := filepath.EvalSymlinks(r.Source)
		if err == nil && resolvedTarget == resolvedSource {
			r.State = AlreadyLinked
		} else {
			r.State = LinkedElsewhere
		}
	case mode.IsDir():
		r.State = ExistsDir
	case mode.IsRegular():
		r.State = ExistsFile
	default:
		r.State = ExistsOther
	}
	return r
}

// pointsTo returns the absolute destination of the symlink at path.
func pointsTo(path string) (string, error) {
	dest, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(filepath.Dir(path), dest)
	}
	return filepath.Clean(dest), nil
}

// rename is os.Rename with a clear message for the one failure that needs the user:
// source and target on different file systems.
func rename(from, to string) error {
	err := os.Rename(from, to)
	if errors.Is(err, syscall.EXDEV) {
		return fmt.Errorf("%s and %s are on different file systems; move it by hand", from, to)
	}
	return err
}

// adopt moves the local file/folder into the cloud and links it back. If the link can't
// be created, the move is undone.
func adopt(r Result, dryRun bool) Result {
	if dryRun {
		r.State = WouldAdopt
		return r
	}
	if err := os.MkdirAll(filepath.Dir(r.Source), 0o755); err != nil {
		return r.fail(err)
	}
	if err := rename(r.Target, r.Source); err != nil {
		return r.fail(err)
	}
	if err := os.Symlink(r.Source, r.Target); err != nil {
		if undoErr := rename(r.Source, r.Target); undoErr != nil {
			return r.fail(fmt.Errorf("%w; moving it back failed too, it's now at %s: %v", err, r.Source, undoErr))
		}
		return r.fail(err)
	}
	r.State = Adopted
	return r
}

// replaceLink swaps the existing symlink for one to the source in a single rename.
func replaceLink(r Result, dryRun bool) Result {
	if dryRun {
		r.State = WouldReplace
		return r
	}
	tmp := r.Target + ".lhcss-tmp"
	os.Remove(tmp) // leftover of an interrupted run; only ever a symlink of ours
	if err := os.Symlink(r.Source, tmp); err != nil {
		return r.fail(err)
	}
	if err := os.Rename(tmp, r.Target); err != nil {
		os.Remove(tmp)
		return r.fail(err)
	}
	r.State = Replaced
	return r
}

// backUp renames the local file/folder to <target>.backup-<timestamp> and links the
// target. If the link can't be created, the rename is undone.
func backUp(r Result, now time.Time, dryRun bool) Result {
	r.Backup = r.Target + ".backup-" + now.Format("20060102-150405")
	if _, err := os.Lstat(r.Backup); err == nil {
		return r.fail(fmt.Errorf("backup %s already exists", r.Backup))
	}
	if dryRun {
		r.State = WouldBackUp
		return r
	}
	if err := rename(r.Target, r.Backup); err != nil {
		return r.fail(err)
	}
	if err := os.Symlink(r.Source, r.Target); err != nil {
		if undoErr := rename(r.Backup, r.Target); undoErr != nil {
			return r.fail(fmt.Errorf("%w; renaming it back failed too, it's now at %s: %v", err, r.Backup, undoErr))
		}
		return r.fail(err)
	}
	r.State = BackedUp
	return r
}

// Unlink removes target if it is a symlink to source, or with Restore replaces it with a
// copy of source (the cloud copy stays). Real files and folders, and links to anything
// else, are left alone.
func Unlink(source, target string, opts UnlinkOptions) Result {
	r := Result{Source: source, Target: target}

	info, err := os.Lstat(target)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		r.State = NothingThere
		return r
	case err != nil:
		return r.fail(err)
	case info.Mode()&fs.ModeSymlink == 0:
		return existing(r, info)
	}

	dest, err := pointsTo(target)
	if err != nil {
		return r.fail(err)
	}
	r.PointsTo = dest
	if !linksTo(target, dest, source) {
		r.State = LinkedElsewhere
		return r
	}

	if !opts.Restore {
		if opts.DryRun {
			r.State = WouldRemove
			return r
		}
		if err := os.Remove(target); err != nil {
			return r.fail(err)
		}
		r.State = Removed
		return r
	}

	if _, err := os.Stat(source); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			r.State = SourceMissing
			return r
		}
		return r.fail(err)
	}
	if opts.DryRun {
		r.State = WouldRestore
		return r
	}
	// Copy next to the link first, then swap: the link only goes once the copy is complete
	tmp := target + ".lhcss-restore"
	if _, err := os.Lstat(tmp); err == nil {
		return r.fail(fmt.Errorf("%s exists (left over from an interrupted restore?); check and remove it", tmp))
	}
	if err := copyTree(source, tmp); err != nil {
		os.RemoveAll(tmp) // only our own partial copy
		return r.fail(err)
	}
	if err := os.Remove(target); err != nil {
		os.RemoveAll(tmp)
		return r.fail(err)
	}
	if err := os.Rename(tmp, target); err != nil {
		os.Symlink(source, target) // put the link back; the copy stays at tmp
		return r.fail(fmt.Errorf("%w; the copy is at %s", err, tmp))
	}
	r.State = Restored
	return r
}

// linksTo reports whether the symlink at target (pointing to dest) points to source, also
// when source is gone or reached through other links.
func linksTo(target, dest, source string) bool {
	if dest == filepath.Clean(source) {
		return true
	}
	resolvedTarget, err1 := filepath.EvalSymlinks(target)
	resolvedSource, err2 := filepath.EvalSymlinks(source)
	return err1 == nil && err2 == nil && resolvedTarget == resolvedSource
}

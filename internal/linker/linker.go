// Package linker creates one symlink target -> source and never touches anything that
// already exists at the target.
package linker

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// State is what Link found at, or did to, a target.
type State int

const (
	// SourceMissing: the cloud copy doesn't exist, nothing to link to.
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
	// ExistsOther: the target is something else (socket, device, ...).
	ExistsOther
	// Failed: checking or creating the link failed, see Result.Err.
	Failed
)

// Result describes one entry.
type Result struct {
	State  State
	Source string
	Target string
	// PointsTo is where an existing symlink at Target points (absolute).
	PointsTo string
	Err      error
}

// Link makes target a symlink to source, unless something is already at target.
func Link(source, target string, dryRun bool) Result {
	r := Result{Source: source, Target: target}

	if _, err := os.Stat(source); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			r.State = SourceMissing
		} else {
			r.State, r.Err = Failed, err
		}
		return r
	}

	// Lstat, not Stat: Stat follows the link, so a broken link would look "missing"
	info, err := os.Lstat(target)
	switch {
	case err == nil:
		return existing(r, info)
	case !errors.Is(err, fs.ErrNotExist):
		r.State, r.Err = Failed, err
		return r
	}

	if dryRun {
		r.State = WouldCreate
		return r
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		r.State, r.Err = Failed, err
		return r
	}
	if err := os.Symlink(source, target); err != nil {
		r.State, r.Err = Failed, err
		return r
	}
	r.State = Created
	return r
}

func existing(r Result, info fs.FileInfo) Result {
	switch mode := info.Mode(); {
	case mode&fs.ModeSymlink != 0:
		pointsTo, err := os.Readlink(r.Target)
		if err != nil {
			r.State, r.Err = Failed, err
			return r
		}
		if !filepath.IsAbs(pointsTo) {
			pointsTo = filepath.Join(filepath.Dir(r.Target), pointsTo)
		}
		r.PointsTo = filepath.Clean(pointsTo)

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

package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/config"
	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/linker"
)

// kind groups the linker states into what the list shows. Each kind has an icon, a color
// and, if something can be done, a suggested action.
type kind int

const (
	kindLinked     kind = iota // ✓ all good
	kindNotLinked              // ○ cloud has it, $HOME has nothing: plain create-symlinks
	kindLocalOnly              // ● only in $HOME: --adopt
	kindBothExist              // ● real copy in $HOME and in the cloud: --force (backup)
	kindBadLink                // ✗ broken link or link to elsewhere: --force
	kindNotInCloud             // · nowhere, nothing to do
	kindOther                  // ? socket, device, error
)

// entry is one line of the list: a symlinkMap item and what the linker says about it.
type entry struct {
	item   string // as written in the config, e.g. ".config/fish"
	source string // in the cloud
	target string // in $HOME
	// status is a plain dry run: what's there now.
	status linker.Result
	// fix is a dry run with --adopt --force: what those flags would do about it.
	fix  linker.Result
	kind kind
	// what is the real thing at target, for the text: "file" or "folder"
	what string
}

// inspect checks every entry without changing anything: both runs are dry runs.
func inspect(cfg config.Config) []entry {
	entries := make([]entry, 0, len(cfg.SymlinkMap))
	for _, item := range cfg.SymlinkMap {
		source := filepath.Join(cfg.CloudStorageHomeDir, item)
		target := filepath.Join(cfg.LocalHomeDir, item)
		e := entry{
			item:   item,
			source: source,
			target: target,
			status: linker.Link(source, target, linker.Options{DryRun: true}),
			fix:    linker.Link(source, target, linker.Options{DryRun: true, Adopt: true, Force: true}),
		}
		e.kind = classify(e)
		e.what = "file"
		if info, err := os.Lstat(target); err == nil && info.IsDir() {
			e.what = "folder"
		}
		entries = append(entries, e)
	}
	return entries
}

// short writes paths under the home folder as ~/..., so they fit the details box.
func short(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || home == "/" {
		return path
	}
	if path == home {
		return "~"
	}
	if rel, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~/" + rel
	}
	return path
}

func classify(e entry) kind {
	switch e.status.State {
	case linker.AlreadyLinked:
		return kindLinked
	case linker.WouldCreate:
		return kindNotLinked
	case linker.SourceMissing:
		if e.fix.State == linker.WouldAdopt {
			return kindLocalOnly
		}
		return kindNotInCloud
	case linker.ExistsDir, linker.ExistsFile:
		return kindBothExist
	case linker.BrokenLink, linker.LinkedElsewhere:
		return kindBadLink
	default:
		return kindOther
	}
}

// icon, label and hint are the three columns of a row.
func (e entry) icon() string {
	switch e.kind {
	case kindLinked:
		return "✓"
	case kindNotLinked:
		return "○"
	case kindLocalOnly, kindBothExist:
		return "●"
	case kindBadLink:
		return "✗"
	case kindNotInCloud:
		return "·"
	default:
		return "?"
	}
}

func (e entry) label() string {
	switch e.kind {
	case kindLinked:
		return "linked"
	case kindNotLinked:
		return "not linked yet"
	case kindLocalOnly:
		return "only in $HOME"
	case kindBothExist:
		return "local " + e.what + " + cloud"
	case kindBadLink:
		if e.status.State == linker.BrokenLink {
			return "broken link"
		}
		return "links elsewhere"
	case kindNotInCloud:
		return "not in cloud yet"
	default:
		if e.status.State == linker.Failed {
			return "error"
		}
		return "something else"
	}
}

// hint is the suggested fix, empty when there's nothing to do.
func (e entry) hint() string {
	switch e.kind {
	case kindNotLinked:
		return "link"
	case kindLocalOnly:
		return "adopt"
	case kindBothExist:
		return "force (backup)"
	case kindBadLink:
		return "force"
	default:
		return ""
	}
}

// details explains the selected entry in a few lines: what's there, what the fix would
// do, and the key that does it (always the last line, it's never cut).
func (e entry) details() []string {
	switch e.kind {
	case kindLinked:
		return []string{
			fmt.Sprintf("%s is a symlink to %s.", short(e.target), short(e.source)),
			"Key: d deletes the link, D replaces it with a copy.",
		}
	case kindNotLinked:
		return []string{
			fmt.Sprintf("The cloud has %s, but %s doesn't exist yet.", short(e.source), short(e.target)),
			"link: create the symlink.",
			"Key: enter (or l) to link.",
		}
	case kindLocalOnly:
		return []string{
			fmt.Sprintf("%s is a real %s, the cloud has none yet.", short(e.target), e.what),
			fmt.Sprintf("adopt: move it to %s and link it back.", short(e.source)),
			"Key: enter (or a) to adopt.",
		}
	case kindBothExist:
		return []string{
			fmt.Sprintf("%s is a real %s, and the cloud has %s too.", short(e.target), e.what, short(e.source)),
			fmt.Sprintf("force: rename the local one to %s and link the cloud copy.", filepath.Base(e.fix.Backup)),
			"Key: enter (or f) to force.",
		}
	case kindBadLink:
		what := "a symlink to"
		if e.status.State == linker.BrokenLink {
			what = "a broken symlink to"
		}
		return []string{
			fmt.Sprintf("%s is %s %s.", short(e.target), what, short(e.status.PointsTo)),
			fmt.Sprintf("force: replace it with a link to %s (%s itself stays).", short(e.source), short(e.status.PointsTo)),
			"Key: enter (or f) to force.",
		}
	case kindNotInCloud:
		return []string{
			fmt.Sprintf("Neither %s nor a real file or folder at %s.", short(e.source), short(e.target)),
			"Nothing to link. Once the other machine adopted it and Nextcloud synced, it shows up as \"not linked yet\".",
		}
	default:
		if e.status.Err != nil {
			return []string{fmt.Sprintf("%s: %v", short(e.target), e.status.Err)}
		}
		return []string{fmt.Sprintf("%s is neither a file, a folder nor a symlink; it's left alone.", short(e.target))}
	}
}

package tui

import (
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/app"
	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/config"
	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/linker"
)

// Actions, previews and running them come from package app; the TUI adds the words.
type (
	action = app.Action
	step   = app.Step
	plan   = app.Plan
)

const (
	actFix     = app.ActFix     // what each entry needs: link, adopt or force
	actLink    = app.ActLink    // create-symlinks
	actAdopt   = app.ActAdopt   // create-symlinks --adopt
	actForce   = app.ActForce   // create-symlinks --force
	actUnlink  = app.ActUnlink  // delete-symlinks
	actRestore = app.ActRestore // delete-symlinks --restore
	actRemove  = app.ActRemove  // remove from the config only, the files stay
)

// doneMsg comes back from execute when the action has run.
type doneMsg app.Result

// execute runs the confirmed plan in the background: Bubble Tea calls the returned
// function in its own goroutine and passes its result to Update as a message, so the
// screen never freezes while files are moved.
func execute(p plan, entries []entry, cfg config.Config, configPath string) tea.Cmd {
	return func() tea.Msg {
		return doneMsg(app.Apply(p, entries, cfg, configPath))
	}
}

func actionTitle(a action) string {
	return [...]string{
		actFix:     "Fix",
		actLink:    "Link",
		actAdopt:   "Adopt",
		actForce:   "Force",
		actUnlink:  "Delete link",
		actRestore: "Delete link, keep a copy",
		actRemove:  "Remove from config",
	}[a]
}

// explain is the first line of the confirmation dialog.
func actionExplain(a action) string {
	return [...]string{
		actFix:     "Each entry gets what it needs: link, adopt (move into the cloud) or force (backup, then link).",
		actLink:    "Create the symlinks into the cloud.",
		actAdopt:   "Move the local files/folders into the cloud and link them back.",
		actForce:   "Replace broken links and links to elsewhere; rename local copies to *.backup-<time>, then link.",
		actUnlink:  "Remove the symlinks. The cloud copy stays; $HOME has nothing there afterwards.",
		actRestore: "Replace the symlinks with a copy of the cloud content. The cloud copy stays.",
		actRemove:  "Remove the entries from the config. Nothing on disk changes: links and files stay.",
	}[a]
}

// describe is one line for a step, in the dialog and in the results.
func describe(a action, s step) string {
	if a == actRemove {
		return "remove from the config"
	}
	r := s.Result
	switch r.State {
	case linker.WouldCreate:
		return "link to " + short(r.Source)
	case linker.Created:
		return "linked to " + short(r.Source)
	case linker.WouldAdopt:
		return "move to " + short(r.Source) + ", link it back"
	case linker.Adopted:
		return "moved to " + short(r.Source) + ", linked"
	case linker.WouldReplace:
		return "replace the link to " + short(r.PointsTo)
	case linker.Replaced:
		return "replaced the link to " + short(r.PointsTo)
	case linker.WouldBackUp:
		return "rename to " + filepath.Base(r.Backup) + ", link the cloud copy"
	case linker.BackedUp:
		return "renamed to " + filepath.Base(r.Backup) + ", linked"
	case linker.WouldRemove:
		return "remove the link"
	case linker.Removed:
		return "link removed"
	case linker.WouldRestore:
		return "replace the link with a copy"
	case linker.Restored:
		return "replaced the link with a copy"
	case linker.AlreadyLinked:
		return "skip: already linked"
	case linker.SourceMissing:
		return "skip: not in the cloud"
	case linker.ExistsDir, linker.ExistsFile:
		what := "file"
		if r.State == linker.ExistsDir {
			what = "folder"
		}
		if a == actUnlink || a == actRestore {
			return "skip: real " + what + ", not a link"
		}
		return "skip: real " + what + " in $HOME (needs force)"
	case linker.BrokenLink, linker.LinkedElsewhere:
		if a == actUnlink || a == actRestore {
			return "skip: links to " + short(r.PointsTo) + ", not to the cloud"
		}
		return "skip: link to " + short(r.PointsTo) + " (needs force)"
	case linker.NothingThere:
		return "skip: nothing there"
	case linker.ExistsOther:
		return "skip: neither file, folder nor link"
	case linker.Failed:
		return fmt.Sprintf("failed: %v", r.Err)
	default:
		return "skip"
	}
}

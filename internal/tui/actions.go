package tui

import (
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/config"
	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/linker"
)

// action is something the TUI can do to the selected entries. Every action is shown as a
// dry run first (preview) and only runs after confirming.
type action int

const (
	actFix     action = iota // what each entry needs: link, adopt or force
	actLink                  // create-symlinks
	actAdopt                 // create-symlinks --adopt
	actForce                 // create-symlinks --force
	actUnlink                // delete-symlinks
	actRestore               // delete-symlinks --restore
	actRemove                // remove from the config only, the files stay
)

func (a action) title() string {
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
func (a action) explain() string {
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

// linkOptions returns how to call the linker for an entry: Link with opts, or (unlink
// true) Unlink with unlinkOpts. For fix it depends on what the entry needs.
func (a action) linkOptions(e entry) (opts linker.Options, unlink bool, unlinkOpts linker.UnlinkOptions) {
	switch a {
	case actAdopt:
		opts.Adopt = true
	case actForce:
		opts.Force = true
	case actFix:
		switch e.kind {
		case kindLocalOnly:
			opts.Adopt = true
		case kindBothExist, kindBadLink:
			opts.Force = true
		}
	case actUnlink:
		return opts, true, linker.UnlinkOptions{}
	case actRestore:
		return opts, true, linker.UnlinkOptions{Restore: true}
	}
	return opts, false, linker.UnlinkOptions{}
}

// run applies the action to one entry, for real or as a dry run.
func (a action) run(e entry, dryRun bool) linker.Result {
	opts, unlink, unlinkOpts := a.linkOptions(e)
	if unlink {
		unlinkOpts.DryRun = dryRun
		return linker.Unlink(e.source, e.target, unlinkOpts)
	}
	opts.DryRun = dryRun
	return linker.Link(e.source, e.target, opts)
}

// step is one line of a preview or a result: the entry and what happens (or happened).
type step struct {
	item   string
	result linker.Result
	// does: the dry run says something would change. Only these run on confirm.
	does bool
}

// plan is a confirmation dialog waiting for yes or no.
type plan struct {
	action action
	steps  []step
}

func (p plan) count() int {
	n := 0
	for _, s := range p.steps {
		if s.does {
			n++
		}
	}
	return n
}

// preview dry-runs the action for the given entries.
func preview(a action, entries []entry) plan {
	p := plan{action: a}
	for _, e := range entries {
		if a == actRemove {
			p.steps = append(p.steps, step{item: e.item, does: true})
			continue
		}
		r := a.run(e, true)
		p.steps = append(p.steps, step{item: e.item, result: r, does: wouldChange(r.State)})
	}
	return p
}

func wouldChange(s linker.State) bool {
	switch s {
	case linker.WouldCreate, linker.WouldAdopt, linker.WouldReplace, linker.WouldBackUp,
		linker.WouldRemove, linker.WouldRestore:
		return true
	}
	return false
}

// doneMsg comes back from execute when the action has run.
type doneMsg struct {
	action action
	steps  []step
	err    error // only for actRemove: saving the config failed
}

// execute runs the confirmed plan in the background: Bubble Tea calls the returned
// function in its own goroutine and passes its result to Update as a message, so the
// screen never freezes while files are moved.
func execute(p plan, entries map[string]entry, cfg config.Config, configPath string) tea.Cmd {
	return func() tea.Msg {
		done := doneMsg{action: p.action}
		if p.action == actRemove {
			remove := map[string]bool{}
			for _, s := range p.steps {
				remove[s.item] = true
			}
			var keep []string
			for _, item := range cfg.SymlinkMap {
				if !remove[item] {
					keep = append(keep, item)
				}
			}
			done.steps = p.steps
			done.err = config.SaveSymlinkMap(configPath, cfg, keep)
			return done
		}
		for _, s := range p.steps {
			if !s.does {
				continue
			}
			r := p.action.run(entries[s.item], false)
			done.steps = append(done.steps, step{item: s.item, result: r, does: r.State != linker.Failed})
		}
		return done
	}
}

// describe is one line for a step, in the dialog and in the results.
func describe(a action, s step) string {
	if a == actRemove {
		return "remove from the config"
	}
	r := s.result
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

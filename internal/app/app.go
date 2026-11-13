// Package app is what the interactive front ends (the TUI, the Home Sync GUI) share: the
// state of every symlinkMap entry, actions on entries with a dry-run preview, and adding
// and removing entries. It has no wording and no presentation; each front end describes
// states and actions in its own words.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/config"
	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/linker"
)

// Kind groups the linker states into what a front end shows. Each kind suggests at most
// one fix.
type Kind int

const (
	KindLinked     Kind = iota // all good
	KindNotLinked              // cloud has it, $HOME has nothing: link
	KindLocalOnly              // only in $HOME: adopt
	KindBothExist              // real copy in $HOME and in the cloud: force (backup)
	KindBadLink                // broken link or link to elsewhere: force
	KindNotInCloud             // nowhere, nothing to do
	KindOther                  // socket, device, error
)

// Entry is one symlinkMap item and what the linker says about it.
type Entry struct {
	Item   string // as written in the config, e.g. ".config/fish"
	Source string // in the cloud
	Target string // in $HOME
	// Status is a plain dry run: what's there now.
	Status linker.Result
	// Fix is a dry run with adopt and force: what those would do about it.
	Fix  linker.Result
	Kind Kind
	// What is the real thing at Target: "file" or "folder".
	What string
}

// Inspect checks every entry without changing anything: both runs are dry runs.
func Inspect(cfg config.Config) []Entry {
	entries := make([]Entry, 0, len(cfg.SymlinkMap))
	for _, item := range cfg.SymlinkMap {
		source := filepath.Join(cfg.CloudStorageHomeDir, item)
		target := filepath.Join(cfg.LocalHomeDir, item)
		e := Entry{
			Item:   item,
			Source: source,
			Target: target,
			Status: linker.Link(source, target, linker.Options{DryRun: true}),
			Fix:    linker.Link(source, target, linker.Options{DryRun: true, Adopt: true, Force: true}),
		}
		e.Kind = classify(e)
		e.What = "file"
		if info, err := os.Lstat(target); err == nil && info.IsDir() {
			e.What = "folder"
		}
		entries = append(entries, e)
	}
	return entries
}

func classify(e Entry) Kind {
	switch e.Status.State {
	case linker.AlreadyLinked:
		return KindLinked
	case linker.WouldCreate:
		return KindNotLinked
	case linker.SourceMissing:
		if e.Fix.State == linker.WouldAdopt {
			return KindLocalOnly
		}
		return KindNotInCloud
	case linker.ExistsDir, linker.ExistsFile:
		return KindBothExist
	case linker.BrokenLink, linker.LinkedElsewhere:
		return KindBadLink
	default:
		return KindOther
	}
}

// Short writes paths under the home folder as ~/....
func Short(path string) string {
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

// Action is something a front end can do to entries. Every action is previewed as a dry
// run first and only applied after the user confirms.
type Action int

const (
	ActFix     Action = iota // what each entry needs: link, adopt or force
	ActLink                  // create-symlinks
	ActAdopt                 // create-symlinks --adopt
	ActForce                 // create-symlinks --force
	ActUnlink                // delete-symlinks
	ActRestore               // delete-symlinks --restore
	ActRemove                // remove from the config only, the files stay
)

// linkOptions returns how to call the linker for an entry: Link with opts, or (unlink
// true) Unlink with unlinkOpts. For fix it depends on what the entry needs.
func (a Action) linkOptions(e Entry) (opts linker.Options, unlink bool, unlinkOpts linker.UnlinkOptions) {
	switch a {
	case ActAdopt:
		opts.Adopt = true
	case ActForce:
		opts.Force = true
	case ActFix:
		switch e.Kind {
		case KindLocalOnly:
			opts.Adopt = true
		case KindBothExist, KindBadLink:
			opts.Force = true
		}
	case ActUnlink:
		return opts, true, linker.UnlinkOptions{}
	case ActRestore:
		return opts, true, linker.UnlinkOptions{Restore: true}
	}
	return opts, false, linker.UnlinkOptions{}
}

// Run applies the action to one entry, for real or as a dry run. Not for ActRemove,
// which only changes the config (see Apply).
func (a Action) Run(e Entry, dryRun bool) linker.Result {
	opts, unlink, unlinkOpts := a.linkOptions(e)
	if unlink {
		unlinkOpts.DryRun = dryRun
		return linker.Unlink(e.Source, e.Target, unlinkOpts)
	}
	opts.DryRun = dryRun
	return linker.Link(e.Source, e.Target, opts)
}

// Step is one line of a preview or a result: the entry and what happens (or happened).
type Step struct {
	Item   string
	Result linker.Result
	// Does: in a preview, something would change (only these run); in a result, it
	// worked.
	Does bool
}

// Plan is a previewed action waiting for confirmation.
type Plan struct {
	Action Action
	Steps  []Step
}

// Count is how many entries the plan would change.
func (p Plan) Count() int {
	n := 0
	for _, s := range p.Steps {
		if s.Does {
			n++
		}
	}
	return n
}

// Preview dry-runs the action for the given entries.
func Preview(a Action, entries []Entry) Plan {
	p := Plan{Action: a}
	for _, e := range entries {
		if a == ActRemove {
			p.Steps = append(p.Steps, Step{Item: e.Item, Does: true})
			continue
		}
		r := a.Run(e, true)
		p.Steps = append(p.Steps, Step{Item: e.Item, Result: r, Does: wouldChange(r.State)})
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

// Result is what Apply did.
type Result struct {
	Action Action
	Steps  []Step
	// Config is the config afterwards (only ActRemove changes it).
	Config config.Config
	// Err is set when saving the config failed (ActRemove).
	Err error
}

// Failed is how many steps didn't work.
func (r Result) Failed() int {
	n := 0
	for _, s := range r.Steps {
		if !s.Does {
			n++
		}
	}
	return n
}

// Apply runs a confirmed plan for real: only the steps whose preview would change
// something. entries are looked up by item, so pass the current ones. It can take a
// while (moving folders), so front ends run it in the background.
func Apply(p Plan, entries []Entry, cfg config.Config, configPath string) Result {
	res := Result{Action: p.Action, Config: cfg}
	if p.Action == ActRemove {
		remove := map[string]bool{}
		for _, s := range p.Steps {
			remove[s.Item] = true
		}
		keep := slices.DeleteFunc(slices.Clone(cfg.SymlinkMap), func(item string) bool { return remove[item] })
		res.Steps = p.Steps
		if res.Err = config.SaveSymlinkMap(configPath, cfg, keep); res.Err == nil {
			res.Config.SymlinkMap = keep
		}
		return res
	}
	byItem := map[string]Entry{}
	for _, e := range entries {
		byItem[e.Item] = e
	}
	for _, s := range p.Steps {
		if !s.Does {
			continue
		}
		r := p.Action.Run(byItem[s.Item], false)
		res.Steps = append(res.Steps, Step{Item: s.Item, Result: r, Does: r.State != linker.Failed})
	}
	return res
}

// NewItem turns a picked path into a symlinkMap item, or explains why it can't be one:
// it must be inside home, not the cloud folder, and not inside or around an entry.
func NewItem(path, home, cloud string, existing []string) (string, error) {
	path = filepath.Clean(path)
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is not inside %s", Short(path), Short(home))
	}
	if path == cloud || strings.HasPrefix(path, cloud+string(filepath.Separator)) ||
		strings.HasPrefix(cloud, path+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is the cloud folder or contains it", Short(path))
	}
	for _, e := range existing {
		switch {
		case e == rel:
			return "", fmt.Errorf("%s is already an entry", rel)
		case strings.HasPrefix(rel, e+"/"):
			return "", fmt.Errorf("%s is inside the entry %s", rel, e)
		case strings.HasPrefix(e, rel+"/"):
			return "", fmt.Errorf("%s contains the entry %s; adopting it would move that link too", rel, e)
		}
	}
	return filepath.ToSlash(rel), nil
}

// AddItem checks a picked path, adds it to the config and saves it. It returns the
// config afterwards and the new item.
func AddItem(cfg config.Config, configPath, path string) (config.Config, string, error) {
	item, err := NewItem(path, cfg.LocalHomeDir, cfg.CloudStorageHomeDir, cfg.SymlinkMap)
	if err != nil {
		return cfg, "", err
	}
	items := append(slices.Clone(cfg.SymlinkMap), item)
	if err := config.SaveSymlinkMap(configPath, cfg, items); err != nil {
		return cfg, "", fmt.Errorf("saving the config failed: %w", err)
	}
	cfg.SymlinkMap = items
	return cfg, item, nil
}

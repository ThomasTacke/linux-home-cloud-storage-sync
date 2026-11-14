package main

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/app"
	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/config"
	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/linker"
)

// Action is what the user can do to entries. Every action is previewed first (Preview)
// and only run after confirming (Apply).
type Action string

const (
	ActionFix      Action = "fix"      // what each entry needs
	ActionConnect  Action = "connect"  // link: use the cloud copy here
	ActionMove     Action = "move"     // adopt: move this computer's copy into the cloud
	ActionUseCloud Action = "useCloud" // force: keep this computer's copy as a backup, use the cloud's
	ActionStop     Action = "stop"     // unlink and keep a copy here (restore)
	ActionRemove   Action = "remove"   // remove from the list only, nothing on disk changes
)

func (a Action) toApp() (app.Action, error) {
	switch a {
	case ActionFix:
		return app.ActFix, nil
	case ActionConnect:
		return app.ActLink, nil
	case ActionMove:
		return app.ActAdopt, nil
	case ActionUseCloud:
		return app.ActForce, nil
	case ActionStop:
		return app.ActRestore, nil
	case ActionRemove:
		return app.ActRemove, nil
	}
	return 0, fmt.Errorf("unknown action %q", a)
}

// Outcome is what a step does or did, for the frontend to put into words.
type Outcome string

const (
	OutcomeConnect     Outcome = "connect"     // will link / linked
	OutcomeMove        Outcome = "move"        // will move to the cloud and link / did
	OutcomeRepair      Outcome = "repair"      // will replace a wrong link / did
	OutcomeBackup      Outcome = "backup"      // will rename the local copy and link / did
	OutcomeDisconnect  Outcome = "disconnect"  // will remove the link / did
	OutcomeCopyBack    Outcome = "copyBack"    // will replace the link with a copy / did
	OutcomeRemoveEntry Outcome = "removeEntry" // will remove from the list / did
	OutcomeSkipSynced  Outcome = "skipSynced"  // nothing to do: already in sync
	OutcomeSkipNoCloud Outcome = "skipNoCloud" // nothing to do: not in the cloud
	OutcomeSkipLocal   Outcome = "skipLocal"   // nothing to do with this action: a real copy is here
	OutcomeSkipLink    Outcome = "skipLink"    // nothing to do with this action: a wrong link is here
	OutcomeSkipNothing Outcome = "skipNothing" // nothing to do: nothing here
	OutcomeSkipOther   Outcome = "skipOther"   // nothing to do: something unusual is here
	OutcomeFailed      Outcome = "failed"      // it went wrong, see Error
)

// Step is one entry in a preview or a result.
type Step struct {
	Item    string  `json:"item"`
	Outcome Outcome `json:"outcome"`
	// Change: in a preview, this entry would change (only these run); in a result, it
	// worked.
	Change bool `json:"change"`
	// Backup is the name the local copy gets (OutcomeBackup).
	Backup string `json:"backup,omitempty"`
	// PointsTo is where the replaced link pointed (OutcomeRepair, OutcomeSkipLink).
	PointsTo string `json:"pointsTo,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Plan is a preview: what an action would do to the chosen entries.
type Plan struct {
	Action Action `json:"action"`
	Steps  []Step `json:"steps"`
	// Changes is how many entries would change; 0 means there's nothing to confirm.
	Changes int `json:"changes"`
}

// Result is what Apply did.
type Result struct {
	Action Action `json:"action"`
	Steps  []Step `json:"steps"`
	Failed int    `json:"failed"`
	// Error is set when saving the list failed (ActionRemove).
	Error string `json:"error,omitempty"`
}

// load reads the config and the current state of the chosen entries.
func (s *SyncService) load(items []string) (config.Config, []app.Entry, error) {
	cfg, err := config.Load(s.path())
	if err != nil {
		return cfg, nil, err
	}
	var chosen []app.Entry
	for _, e := range app.Inspect(cfg) {
		if slices.Contains(items, e.Item) {
			chosen = append(chosen, e)
		}
	}
	return cfg, chosen, nil
}

// Preview shows what action would do to items. It changes nothing.
func (s *SyncService) Preview(action Action, items []string) (Plan, error) {
	a, err := action.toApp()
	if err != nil {
		return Plan{}, err
	}
	_, entries, err := s.load(items)
	if err != nil {
		return Plan{}, err
	}
	p := app.Preview(a, entries)
	out := Plan{Action: action, Steps: []Step{}, Changes: p.Count()}
	for _, st := range p.Steps {
		out.Steps = append(out.Steps, toStep(a, st))
	}
	return out, nil
}

// Apply runs action on items. It checks every entry again first and does what's needed
// now: the same as the preview, unless something changed on disk in between.
func (s *SyncService) Apply(action Action, items []string) (Result, error) {
	a, err := action.toApp()
	if err != nil {
		return Result{}, err
	}
	cfg, entries, err := s.load(items)
	if err != nil {
		return Result{}, err
	}
	res := app.Apply(app.Preview(a, entries), entries, cfg, s.path())
	out := Result{Action: action, Steps: []Step{}, Failed: res.Failed()}
	if res.Err != nil {
		out.Error = res.Err.Error()
	}
	for _, st := range res.Steps {
		out.Steps = append(out.Steps, toStep(a, st))
	}
	return out, nil
}

func toStep(a app.Action, st app.Step) Step {
	r := st.Result
	out := Step{Item: st.Item, Change: st.Does}
	if a == app.ActRemove {
		out.Outcome = OutcomeRemoveEntry
		return out
	}
	switch r.State {
	case linker.WouldCreate, linker.Created:
		out.Outcome = OutcomeConnect
	case linker.WouldAdopt, linker.Adopted:
		out.Outcome = OutcomeMove
	case linker.WouldReplace, linker.Replaced:
		out.Outcome, out.PointsTo = OutcomeRepair, app.Short(r.PointsTo)
	case linker.WouldBackUp, linker.BackedUp:
		out.Outcome, out.Backup = OutcomeBackup, filepath.Base(r.Backup)
	case linker.WouldRemove, linker.Removed:
		out.Outcome = OutcomeDisconnect
	case linker.WouldRestore, linker.Restored:
		out.Outcome = OutcomeCopyBack
	case linker.AlreadyLinked:
		out.Outcome = OutcomeSkipSynced
	case linker.SourceMissing:
		out.Outcome = OutcomeSkipNoCloud
	case linker.ExistsDir, linker.ExistsFile:
		out.Outcome = OutcomeSkipLocal
	case linker.BrokenLink, linker.LinkedElsewhere:
		out.Outcome, out.PointsTo = OutcomeSkipLink, app.Short(r.PointsTo)
	case linker.NothingThere:
		out.Outcome = OutcomeSkipNothing
	case linker.Failed:
		out.Outcome = OutcomeFailed
		if r.Err != nil {
			out.Error = r.Err.Error()
		}
	default:
		out.Outcome = OutcomeSkipOther
	}
	return out
}

// Added is the result of AddEntry: the new entry's name and how it looks now.
type Added struct {
	Item  string `json:"item"`
	Entry Entry  `json:"entry"`
}

// AddEntry adds a file or folder (a path in the home folder) to the list and saves it.
// It doesn't move or link anything; the frontend offers that next.
func (s *SyncService) AddEntry(path string) (Added, error) {
	cfg, err := config.Load(s.path())
	if err != nil {
		return Added{}, err
	}
	cfg, item, err := app.AddItem(cfg, s.path(), path)
	if err != nil {
		return Added{}, err
	}
	for _, e := range app.Inspect(cfg) {
		if e.Item == item {
			return Added{Item: item, Entry: toEntry(e)}, nil
		}
	}
	return Added{Item: item}, nil
}

// ChooseFolder opens the system's folder dialog in the home folder. An empty result
// means the user cancelled.
func (s *SyncService) ChooseFolder(showHidden bool) (string, error) {
	return s.choose(true, showHidden)
}

// ChooseFile opens the system's file dialog in the home folder (for single settings
// files, an advanced thing).
func (s *SyncService) ChooseFile(showHidden bool) (string, error) {
	return s.choose(false, showHidden)
}

func (s *SyncService) choose(folder, showHidden bool) (string, error) {
	cfg, err := config.Load(s.path())
	if errors.Is(err, fs.ErrNotExist) {
		return "", errors.New("Home Sync isn't set up yet")
	} else if err != nil {
		return "", err
	}
	title := "Choose a folder to keep in sync"
	if !folder {
		title = "Choose a file to keep in sync"
	}
	return application.Get().Dialog.OpenFile().
		SetTitle(title).
		SetDirectory(cfg.LocalHomeDir).
		CanChooseDirectories(folder).
		CanChooseFiles(!folder).
		ShowHiddenFiles(showHidden).
		SetButtonText("Add").
		PromptForSingleSelection()
}

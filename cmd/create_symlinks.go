package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/config"
	"github.com/s3r3nity-labs/linux-home-cloud-storage-sync/internal/linker"
)

var (
	bold    = color.New(color.Bold).SprintFunc()
	green   = color.New(color.Bold, color.FgGreen).SprintFunc()
	red     = color.New(color.Bold, color.FgRed).SprintFunc()
	yellow  = color.New(color.FgYellow).SprintFunc()
	yellowB = color.New(color.Bold, color.FgYellow).SprintFunc()
)

func newCreateSymlinksCmd() *cobra.Command {
	var opts linker.Options
	cmd := &cobra.Command{
		Use:   "create-symlinks [config]",
		Short: "Creates symlinks from CloudStorage/LinuxHome to $HOME",
		Long: `Creates symlinks from CloudStorage/LinuxHome to $HOME, as listed in the config (default: ` + config.DefaultPath() + `).

Without flags nothing that exists is changed. --adopt and --force are for the cases
that are reported otherwise; try them with -n first.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadOrDefaults(cmd.OutOrStdout(), configPath(args))
			if err != nil {
				return err
			}
			return forEach(cmd.OutOrStdout(), cfg, func(source, target string) linker.Result {
				return linker.Link(source, target, opts)
			})
		},
	}
	cmd.Flags().BoolVarP(&opts.DryRun, "dry-run", "n", false, "Show actions without executing")
	cmd.Flags().BoolVar(&opts.Adopt, "adopt", false, "Move a local file/folder that isn't in the cloud yet into it, then link it")
	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false, "Replace broken links and links to elsewhere; rename a local file/folder that's also in the cloud to <name>.backup-<timestamp>, then link it")
	return cmd
}

func newDeleteSymlinksCmd() *cobra.Command {
	var opts linker.UnlinkOptions
	cmd := &cobra.Command{
		Use:   "delete-symlinks [config]",
		Short: "Removes the symlinks into CloudStorage/LinuxHome again",
		Long: `Removes the symlinks in $HOME that point to their entry in CloudStorage/LinuxHome, as
listed in the config (default: ` + config.DefaultPath() + `). Real files and folders, and
links to anything else, are left alone. The cloud copy is never touched.

With --restore each link is replaced by a copy of the cloud content instead, keeping
permissions; the cloud copy stays for the other machines.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadOrDefaults(cmd.OutOrStdout(), configPath(args))
			if err != nil {
				return err
			}
			return forEach(cmd.OutOrStdout(), cfg, func(source, target string) linker.Result {
				return linker.Unlink(source, target, opts)
			})
		},
	}
	cmd.Flags().BoolVarP(&opts.DryRun, "dry-run", "n", false, "Show actions without executing")
	cmd.Flags().BoolVar(&opts.Restore, "restore", false, "Replace each link with a copy of the cloud content")
	return cmd
}

func configPath(args []string) string {
	if len(args) == 1 {
		return config.ExpandHome(args[0])
	}
	return config.DefaultPath()
}

func loadOrDefaults(out io.Writer, path string) (config.Config, error) {
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		fmt.Fprintln(out, "No configuration found/given. Continue with default values.")
		return config.Defaults(), nil
	}
	cfg, err := config.Load(path)
	var invalid *config.ValidationError
	if errors.As(err, &invalid) {
		return cfg, fmt.Errorf("invalid configuration %q:\n  %s", path, strings.Join(invalid.Problems, "\n  "))
	}
	return cfg, err
}

// forEach runs do for every entry, prints each result and fails if any entry failed.
func forEach(out io.Writer, cfg config.Config, do func(source, target string) linker.Result) error {
	failed := 0
	for _, item := range cfg.SymlinkMap {
		r := do(
			filepath.Join(cfg.CloudStorageHomeDir, item),
			filepath.Join(cfg.LocalHomeDir, item),
		)
		fmt.Fprintln(out, describe(r))
		if r.State == linker.Failed {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d entries failed", failed, len(cfg.SymlinkMap))
	}
	return nil
}

func describe(r linker.Result) string {
	target := fmt.Sprintf("The %s %q", yellowB("target"), r.Target)
	switch r.State {
	case linker.SourceMissing:
		return fmt.Sprintf("The %s %q does not exist.", red("source"), r.Source)
	case linker.Created:
		return bold(fmt.Sprintf("Symlink %s -> %s created.", r.Target, r.Source))
	case linker.WouldCreate:
		return yellow(fmt.Sprintf("Dry run: would create symlink %s -> %s", r.Target, r.Source))
	case linker.AlreadyLinked:
		return fmt.Sprintf("%s already exists and is a %s", target, green("symlink"))
	case linker.LinkedElsewhere:
		return fmt.Sprintf("%s is a %s to %q, not %q", target, red("symlink"), r.PointsTo, r.Source)
	case linker.BrokenLink:
		return fmt.Sprintf("%s is a %s to %q, not %q", target, red("broken symlink"), r.PointsTo, r.Source)
	case linker.ExistsDir:
		return fmt.Sprintf("%s already exists and is a %s", target, red("directory"))
	case linker.ExistsFile:
		return fmt.Sprintf("%s already exists and is a %s", target, red("file"))
	case linker.ExistsOther:
		return fmt.Sprintf("%s already exists and is something else", target)
	case linker.Adopted:
		return bold(fmt.Sprintf("Moved %s to %s and linked it.", r.Target, r.Source))
	case linker.WouldAdopt:
		return yellow(fmt.Sprintf("Dry run: would move %s to %s and link it", r.Target, r.Source))
	case linker.Replaced:
		return bold(fmt.Sprintf("Replaced symlink %s (was -> %s) with -> %s.", r.Target, r.PointsTo, r.Source))
	case linker.WouldReplace:
		return yellow(fmt.Sprintf("Dry run: would replace symlink %s (-> %s) with -> %s", r.Target, r.PointsTo, r.Source))
	case linker.BackedUp:
		return bold(fmt.Sprintf("Renamed %s to %s and linked it to %s.", r.Target, r.Backup, r.Source))
	case linker.WouldBackUp:
		return yellow(fmt.Sprintf("Dry run: would rename %s to %s and link it to %s", r.Target, r.Backup, r.Source))
	case linker.Removed:
		return bold(fmt.Sprintf("Removed symlink %s -> %s.", r.Target, r.Source))
	case linker.WouldRemove:
		return yellow(fmt.Sprintf("Dry run: would remove symlink %s -> %s", r.Target, r.Source))
	case linker.Restored:
		return bold(fmt.Sprintf("Replaced symlink %s with a copy of %s.", r.Target, r.Source))
	case linker.WouldRestore:
		return yellow(fmt.Sprintf("Dry run: would replace symlink %s with a copy of %s", r.Target, r.Source))
	case linker.NothingThere:
		return fmt.Sprintf("%s does not exist, nothing to remove.", target)
	default:
		if errors.Is(r.Err, fs.ErrPermission) {
			return fmt.Sprintf("%s: %s (permission denied)", red("failed"), r.Target)
		}
		return fmt.Sprintf("%s: %s: %v", red("failed"), r.Target, r.Err)
	}
}

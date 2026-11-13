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

	"github.com/ThomasTacke/linux-home-cloud-storage-sync/internal/config"
	"github.com/ThomasTacke/linux-home-cloud-storage-sync/internal/linker"
)

var (
	bold    = color.New(color.Bold).SprintFunc()
	green   = color.New(color.Bold, color.FgGreen).SprintFunc()
	red     = color.New(color.Bold, color.FgRed).SprintFunc()
	yellow  = color.New(color.FgYellow).SprintFunc()
	yellowB = color.New(color.Bold, color.FgYellow).SprintFunc()
)

func newCreateSymlinksCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "create-symlinks [config]",
		Short: "Creates symlinks from CloudStorage/LinuxHome to $HOME",
		Long:  "Creates symlinks from CloudStorage/LinuxHome to $HOME, as listed in the config (default: " + config.DefaultPath() + ").",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.DefaultPath()
			if len(args) == 1 {
				path = config.ExpandHome(args[0])
			}
			cfg, err := loadOrDefaults(cmd.OutOrStdout(), path)
			if err != nil {
				return err
			}
			return createSymlinks(cmd.OutOrStdout(), cfg, dryRun)
		},
	}
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Show actions without executing")
	return cmd
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

func createSymlinks(out io.Writer, cfg config.Config, dryRun bool) error {
	failed := 0
	for _, item := range cfg.SymlinkMap {
		r := linker.Link(
			filepath.Join(cfg.CloudStorageHomeDir, item),
			filepath.Join(cfg.LocalHomeDir, item),
			dryRun,
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
	default:
		if errors.Is(r.Err, fs.ErrPermission) {
			return fmt.Sprintf("%s: %s (permission denied)", red("failed"), r.Target)
		}
		return fmt.Sprintf("%s: %s: %v", red("failed"), r.Target, r.Err)
	}
}

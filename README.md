<!-- omit from toc -->
# linux-home-cloud-storage-sync

*A tiny CLI to keep a "cloud-storage home" folder linked into the rest of your Linux home.*

> ⚡️ **What it does** – It creates symlinks inside your `$HOME` that point to a folder in
> your cloud storage (e.g. Nextcloud, Dropbox, …). You give it a config file and the tool
> takes care of the rest. One static binary, written in Go.

<!-- omit from toc -->
## Table of Contents

- [Features](#features)
- [How it works](#how-it-works)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Usage](#usage)
- [Development](#development)
- [Releases](#releases)
- [TODOs \& Roadmap](#todos--roadmap)

---

## Features

| Feature                                                   | Status |
| --------------------------------------------------------- | ------ |
| Dry-run mode (`-n` / `--dry-run`)                         | ✅      |
| Create symlinks                                           | ✅      |
| Report broken links and links to elsewhere                | ✅      |
| `--adopt`: move a local file/folder into the cloud        | ✅      |
| `--force`: replace links, set local copies aside          | ✅      |
| `delete-symlinks` (optionally `--restore` a copy)         | ✅      |
| `tui`: interactive view with actions and new entries      | ✅      |
| `~` in config paths                                       | ✅      |
| Shell completion (fish, bash, zsh)                        | ✅      |
| Unit tests                                                | ✅      |

---

## How it works

For every entry in `symlinkMap` the tool looks at two paths:

- **source** – `cloudStorageHomeDir/<entry>` (the copy living in your cloud storage)
- **target** – `localHomeDir/<entry>` (the location in your `$HOME`)

The direction is **cloud → home**: the cloud copy is the real one, `$HOME` gets a symlink
to it. Paths in the config may start with `~`, the home of the user running the tool.

`create-symlinks` applies these rules. Without flags it **never changes anything that
exists**; `--adopt` and `--force` are for the cases it otherwise only reports:

| Situation                                        | Without flags        | With the flag                                                                 |
| ------------------------------------------------ | -------------------- | ----------------------------------------------------------------------------- |
| source exists, target missing                    | create the symlink   |                                                                               |
| target is a symlink to source                    | nothing (all good)   |                                                                               |
| source missing, target is a real file/folder     | notice               | `--adopt`: move it to the source, then link it                                |
| source missing, nothing (or a link) at target    | notice               |                                                                               |
| target is a broken link or a link to elsewhere   | notice, in red       | `--force`: replace the link (what it pointed to stays)                        |
| source exists **and** target is a real file/folder | notice, in red     | `--force`: rename it to `<target>.backup-<YYYYMMDD-HHMMSS>`, then link it     |
| target is a socket, device, …                    | notice               | never touched                                                                 |

Nothing is ever deleted or merged: a backup is a rename next to the original, and an
existing backup name is never overwritten. Moves are renames, so source and target must
be on the same file system (they usually are, both in your home); otherwise the entry
fails with a message and stays as it was. If linking fails after a move or rename, the
move is undone.

`delete-symlinks` is the way back. It removes only symlinks that point to their entry's
source; real files and folders, and links to anything else, stay. With `--restore` each
link becomes a copy of the cloud content (permissions, times and inner symlinks kept).
The cloud copy is never touched, the other machines still use it.

### Adding a new entry

1. Add it to `symlinkMap` (e.g. `".claude/skills"`).
2. On the machine that has it:

   ```fish
   linux-home-cloud-storage-sync create-symlinks --adopt -n   # dry run first
   linux-home-cloud-storage-sync create-symlinks --adopt
   ```

   It's moved to `~/Nextcloud/LinuxHome/.claude/skills` and linked back. Quit the program
   that uses it first (fish, an editor, …), so it doesn't write in the middle of the move.
3. On the other machines, **once Nextcloud has synced it**:

   ```fish
   linux-home-cloud-storage-sync create-symlinks --force -n
   linux-home-cloud-storage-sync create-symlinks --force
   ```

   A local copy there (e.g. a default config) is renamed to `….backup-<timestamp>`;
   compare and delete it yourself. Don't use `--adopt` on a machine that hasn't synced
   yet: it would move its local copy into the cloud as well, and Nextcloud would end up
   with a conflict file.

---

## Installation

With [Homebrew](https://brew.sh) (e.g. on Bazzite), updated by `brew upgrade`:

```fish
brew install s3r3nity-labs/tap/linux-home-cloud-storage-sync
```

From a release, no Go needed (`amd64`; for ARM replace it with `arm64`):

```fish
curl -sL https://github.com/s3r3nity-labs/linux-home-cloud-storage-sync/releases/latest/download/linux-home-cloud-storage-sync_linux_amd64.tar.gz \
  | tar -xz -C ~/.local/bin linux-home-cloud-storage-sync
```

`checksums.txt` in each release has the SHA-256 of every archive.

With Go 1.26.7 or newer (the TUI libraries need it):

```fish
go install github.com/s3r3nity-labs/linux-home-cloud-storage-sync@latest
```

The binary lands in `~/go/bin` (add it to your `PATH`). Or build it yourself and copy the
single file wherever you like, e.g. to the other machine:

```fish
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ~/.local/bin/linux-home-cloud-storage-sync .
```

For a release build, set the version: `-ldflags "-s -w -X
github.com/s3r3nity-labs/linux-home-cloud-storage-sync/cmd.version=v0.2.0"`. `go install
...@v0.2.0` picks up the tag by itself.

Shell completion for fish:

```fish
linux-home-cloud-storage-sync completion fish > ~/.config/fish/completions/linux-home-cloud-storage-sync.fish
```

---

## Quick start

1. Create a config file (JSON) inside the folder you keep in your cloud storage (e.g.
   `~/Nextcloud/LinuxHome/.config/linux-home-cloud-storage-sync/config.json`), or write the
   template and edit it:

   ```fish
   linux-home-cloud-storage-sync write-default-config
   ```

   ```json
   {
       "cloudStorageHomeDir": "~/Nextcloud/LinuxHome",
       "localHomeDir": "~",
       "symlinkMap": ["Desktop", "Documents", ".config/fish", ".gitconfig"]
   }
   ```

2. Run it, dry run first:

   ```fish
   linux-home-cloud-storage-sync create-symlinks -n
   linux-home-cloud-storage-sync create-symlinks
   ```

---

## Usage

```fish
❯ linux-home-cloud-storage-sync --help
Creates symlinks inside your $HOME that point to a folder in your cloud storage (Nextcloud, Dropbox, ...).
Without --adopt or --force it never moves or renames anything, and it never deletes files or folders.

Usage:
  linux-home-cloud-storage-sync [command]

Available Commands:
  completion            Generate the autocompletion script for the specified shell
  create-symlinks       Creates symlinks from CloudStorage/LinuxHome to $HOME
  delete-symlinks       Removes the symlinks into CloudStorage/LinuxHome again
  help                  Help about any command
  print-template-config Prints the template configuration
  tui                   Interactive view: status, fix, adopt, force, delete, add entries
  write-default-config  Writes the template configuration

Flags:
  -h, --help      help for linux-home-cloud-storage-sync
  -v, --version   version for linux-home-cloud-storage-sync
```

`create-symlinks [config]` and `delete-symlinks [config]` read
`~/.config/linux-home-cloud-storage-sync/config.json` unless given a path, and go through
every entry. Flags: `-n` (dry run) for both, `--adopt` and `-f`/`--force` for
`create-symlinks`, `--restore` for `delete-symlinks`.

```fish
❯ linux-home-cloud-storage-sync create-symlinks
The target "/home/kiwi/.config/fish" already exists and is a symlink
Symlink /home/kiwi/.config/MangoHud -> /home/kiwi/Nextcloud/LinuxHome/.config/MangoHud created.
The target "/home/kiwi/Music" is a broken symlink to "/home/kiwi/Old/Music", not "/home/kiwi/Nextcloud/LinuxHome/Music"
The source "/home/kiwi/Nextcloud/LinuxHome/Templates" does not exist.
```

`tui [config]` is the same as a full-screen view: every entry with an icon, its status
and what would fix it, details for the selected one, and actions on the selection. Every
action shows its dry run in a dialog first and only runs after `y`/`enter`.

| Key | |
| --- | --- |
| `↑`/`↓` (`k`/`j`), `g`/`G` | move |
| `space`, `ctrl+a`, `esc` | select (and move on), select all shown, clear selection (then filter) |
| `enter` | **fix**: what each entry needs (link, adopt or force) |
| `l` / `a` / `f` | link / adopt / force, like the `create-symlinks` flags |
| `d` / `D` | delete the link / replace it with a copy (`delete-symlinks`, `--restore`) |
| `n` | **new entry**: pick a file or folder in `$HOME`, it's added to the config and adopted (or linked) after confirming |
| `x` | remove from the config (nothing on disk changes) |
| `/`, `r`, `?`, `q` | filter, check again, all keys, quit |

Actions apply to the selected entries, or to the one under the cursor when nothing is
selected. Saving the config only rewrites `symlinkMap`; `~` in its paths and other keys
stay, and when the config is a symlink into the cloud, the link stays too.

```
linux-home-cloud-storage-sync  ~/Nextcloud/LinuxHome                    ✓ 1  ○ 1  ● 2  ✗ 1  · 1
╭──────────────────────────────────────────────────────────────────────────────────────────────────╮
│ ▸ ✓  .config/fish  linked                                                                        │
│   ○  Pictures      not linked yet        → link                                                  │
│   ●  .gitconfig    only in $HOME         → adopt                                                 │
│   ●  Documents     local folder + cloud  → force (backup)                                        │
│   ✗  Music         broken link           → force                                                 │
│   ·  Templates     not in cloud yet                                                              │
╰──────────────────────────────────────────────────────────────────────────────────────────────────╯
╭──────────────────────────────────────────────────────────────────────────────────────────────────╮
│ .gitconfig  only in $HOME                                                                        │
│ ~/.gitconfig is a real file, the cloud has none yet.                                             │
│ adopt: move it to ~/Nextcloud/LinuxHome/.gitconfig and link it back.                             │
│ Key: enter (or a) to adopt.                                                                      │
╰──────────────────────────────────────────────────────────────────────────────────────────────────╯
space select • enter fix • n new entry • / filter • ? more keys • q quit
```

With entries selected, `enter` opens the dialog on top of the list:

```
╭──────────────────────────────────────────────────────────────────────────────────╮
│ Fix 4 entries?                                                                   │
│                                                                                  │
│ Each entry gets what it needs: link, adopt (move into the cloud) or force        │
│ (backup, then link). 2 have nothing to do and are skipped.                       │
│                                                                                  │
│ · .config/fish  skip: already linked                                             │
│ → Pictures      link to ~/Nextcloud/LinuxHome/Pictures                           │
│ → .gitconfig    move to ~/Nextcloud/LinuxHome/.gitconfig, link it back           │
│ → Documents     rename to Documents.backup-20261114-000617, link the cloud copy  │
│ → Music         replace the link to ~/Old/Music                                  │
│ · Templates     skip: not in the cloud                                           │
╰──────────────────────────────────────────────────────────────────────────────────╯
y/enter do it • n/esc cancel
```

Both exit with 1 on an invalid config (listing every problem) or when an entry failed
(e.g. permission denied); the other entries are still done.

---

## Development

```fish
go test ./...
go vet ./...
go run . create-symlinks -n
```

- `main.go` – entry point
- `cmd/` – the Cobra commands and their output
- `internal/config/` – reading, validating and writing the JSON config
- `internal/linker/` – deciding what to do for one entry and creating the link
- `internal/app/` – what the interactive front ends share: state of every entry, actions
  with dry-run preview and apply, adding and removing entries (no wording)
- `internal/tui/` – the `tui` view ([Bubble Tea](https://github.com/charmbracelet/bubbletea) v2:
  `model.go` state, modes and keys, `view.go` drawing and dialogs, `entry.go` and
  `actions.go` the TUI's words for states and actions, `picker.go` the file picker)

---

## Releases

GitHub Actions runs the tests on every push and pull request (`.github/workflows/ci.yml`).
To release:

1. *Releases → Draft a new release*, new tag (e.g. `v0.3.0`) on `main`, *Generate release
   notes*, *Publish release*.
2. `.github/workflows/release.yml` runs the tests and GoReleaser (`.goreleaser.yaml`):
   static binaries for `linux/amd64` and `linux/arm64`, as `.tar.gz` with README and
   LICENSE, plus `checksums.txt`, attached to the release. The text from step 1 stays.
   It also commits the Homebrew cask to
   [s3r3nity-labs/homebrew-tap](https://github.com/s3r3nity-labs/homebrew-tap), with the
   repository secret `HOMEBREW_TAP_GITHUB_TOKEN` (fine-grained, Contents read/write on
   that repository only; expires 2027-11-14, then regenerate it and update the secret).

Local test build, nothing published: `goreleaser release --snapshot --clean` (output in
`dist/`).

---

## TODOs & Roadmap

- `CONTRIBUTING.md`.

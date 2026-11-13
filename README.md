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
- [TODOs \& Roadmap](#todos--roadmap)

---

## Features

| Feature                                        | Status |
| ---------------------------------------------- | ------ |
| Dry-run mode (`-n` / `--dry-run`)              | ✅      |
| Create symlinks                                | ✅      |
| Report broken links and links to elsewhere     | ✅      |
| `~` in config paths                            | ✅      |
| Shell completion (fish, bash, zsh)             | ✅      |
| Unit tests                                     | ✅      |
| Delete symlinks                                | ⚙️      |
| Force recreation of existing links             | ⚙️      |

---

## How it works

The tool **only creates symlinks** – it never moves or copies your data. For every entry
in `symlinkMap` it looks at two paths:

- **source** – `cloudStorageHomeDir/<entry>` (the copy living in your cloud storage)
- **target** – `localHomeDir/<entry>` (the location in your `$HOME`)

and then applies this rule:

| Situation                            | Action                                             |
| ------------------------------------ | -------------------------------------------------- |
| source exists **and** target missing | create symlink `target → source`                   |
| source does **not** exist            | do nothing (just print a notice)                   |
| target is a symlink to source        | do nothing (reported as symlink)                   |
| target is a symlink to somewhere else, or a broken one | do nothing (reported in red, with where it points) |
| target is a real file or directory   | do nothing (just print a notice)                   |

Paths in the config may start with `~`, which means the home of the user running the
tool.

So the direction is **cloud → home**: the authoritative copy must already live inside
`cloudStorageHomeDir`. The tool will **not** migrate an existing file for you, and it will
**not** overwrite anything that is already present in your `$HOME`.

### First‑time migration

Because the tool leaves existing files untouched, adding a *new* entry to `symlinkMap`
takes one manual step first – move the real file/folder into your cloud storage, keeping
the same relative path:

```fish
# Example: back up ~/.claude/skills
mkdir -p ~/Nextcloud/LinuxHome/.claude
mv ~/.claude/skills ~/Nextcloud/LinuxHome/.claude/

# then let the tool link it back into $HOME
linux-home-cloud-storage-sync create-symlinks -n   # dry run first
linux-home-cloud-storage-sync create-symlinks
```

After that the file lives in the cloud and `~/.claude/skills` is a symlink pointing to it.

---

## Installation

With Go 1.25 or newer:

```fish
go install github.com/ThomasTacke/linux-home-cloud-storage-sync@latest
```

The binary lands in `~/go/bin` (add it to your `PATH`). Or build it yourself and copy the
single file wherever you like, e.g. to the other machine:

```fish
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ~/.local/bin/linux-home-cloud-storage-sync .
```

For a release build, set the version: `-ldflags "-s -w -X
github.com/ThomasTacke/linux-home-cloud-storage-sync/cmd.version=v0.2.0"`. `go install
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
It never moves, copies or overwrites anything.

Usage:
  linux-home-cloud-storage-sync [command]

Available Commands:
  completion            Generate the autocompletion script for the specified shell
  create-symlinks       Creates symlinks from CloudStorage/LinuxHome to $HOME
  help                  Help about any command
  print-template-config Prints the template configuration
  write-default-config  Writes the template configuration

Flags:
  -h, --help      help for linux-home-cloud-storage-sync
  -v, --version   version for linux-home-cloud-storage-sync
```

`create-symlinks [config]` reads `~/.config/linux-home-cloud-storage-sync/config.json`
unless given a path, and goes through every entry:

```fish
❯ linux-home-cloud-storage-sync create-symlinks
The target "/home/kiwi/.config/fish" already exists and is a symlink
Symlink /home/kiwi/.config/MangoHud -> /home/kiwi/Nextcloud/LinuxHome/.config/MangoHud created.
The target "/home/kiwi/Music" is a broken symlink to "/home/kiwi/Old/Music", not "/home/kiwi/Nextcloud/LinuxHome/Music"
The source "/home/kiwi/Nextcloud/LinuxHome/Templates" does not exist.
```

It exits with 1 on an invalid config (listing every problem) or when an entry couldn't
be linked (e.g. permission denied); everything else is just reported.

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

---

## TODOs & Roadmap

- `--force` option – replace broken links and links pointing elsewhere.
- `delete-symlinks` command – remove stale links.
- GitHub Actions: tests on every push, release binaries for tags (GoReleaser).
- `CONTRIBUTING.md`.

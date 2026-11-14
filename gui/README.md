# Home Sync

The desktop app of linux-home-cloud-storage-sync, for people who'd rather not use a
terminal. Same logic as the CLI and the TUI (`internal/app` in the parent module), shown
in plain words.

Built with [Wails 3](https://v3.wails.io) (Go backend, system WebView), Svelte 5,
Tailwind CSS 4 and [shadcn-svelte](https://shadcn-svelte.com).

**Status:** phase 3. Without a config, a wizard sets Home Sync up: it finds the cloud
folder (Nextcloud client settings, ~/Nextcloud, ~/Dropbox, …), offers the usual folders
under the names the desktop uses (Dokumente, Bilder, … on a German system, from
`~/.config/user-dirs.dirs`), shows what will happen and sets up. Afterwards it shows every
entry and whether it's in sync, and fixes it: a button per item for what it needs, a menu
for Stop syncing and Remove from list, a selection bar for several items, and Add folder…
(Add file… with hidden settings shown). Every action shows what will happen first and only
runs after confirming. It's installed from the s3r3nity-labs Flatpak repository.

## Development

Needs Go 1.26.7+, Node 22+ (26 tested), and GTK4 and WebKitGTK 6.0 with headers
(Ubuntu: `libgtk-4-dev libwebkitgtk-6.0-dev`, Arch: `gtk4 webkitgtk-6.0`). Then:

```fish
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
wails3 doctor      # checks the setup
wails3 dev         # runs the app with hot reload
wails3 build       # bin/homesync
```

To try the buttons without touching your real files, use the demo setup (one item in
every state, in /tmp):

```fish
scripts/demo-setup.sh
HOME_SYNC_CONFIG=/tmp/homesync-demo/config.json wails3 dev
```

For the setup wizard, a home without a config (German folder names, empty Nextcloud):

```fish
scripts/demo-setup.sh --fresh
env HOME_SYNC_CONFIG=/tmp/homesync-fresh/config.json HOME_SYNC_HOME=/tmp/homesync-fresh/home wails3 dev
```

`HOME_SYNC_HOME` is the home folder the wizard works in.
`HOME_SYNC_CONFIG` points Home Sync at any config instead of
`~/.config/linux-home-cloud-storage-sync/config.json`. `go test .` covers the Go side.

- `service.go` – `SyncService`, the Go methods the frontend calls; `wails3 generate
  bindings -ts` turns them into `frontend/bindings` (done by `wails3 build`/`dev` too)
- `actions.go` – Preview, Apply, AddEntry and the file dialogs; `setup.go` – the wizard's
  side: finding cloud folders and the desktop's folder names, preview, setting up
- `frontend/src/App.svelte` – the window; `frontend/src/lib/ActionDialog.svelte` – preview,
  confirm and result of every action; `frontend/src/lib/SetupWizard.svelte` – first run; `frontend/src/lib/status.ts` and `actions.ts` – the
  words for each state, action and step; `frontend/src/lib/components/ui` – shadcn-svelte components
- `go.mod` uses the parent module through `replace … => ../`, so the CLI's `go.mod`
  stays free of Wails and cgo

## Flatpak

Install from the s3r3nity-labs Flatpak repository (https://s3r3nity-labs.github.io/flatpak/),
which also brings updates through the app store:

```fish
flatpak install --user https://s3r3nity-labs.github.io/flatpak/org.s3r3nity.HomeSync.flatpakref
```

Every release (and "Run workflow" on `.github/workflows/flatpak.yml`) builds
`build/flatpak/org.s3r3nity.HomeSync.yml` into that repository, signed with the key in the
secret `FLATPAK_GPG_PRIVATE_KEY`, and pushes it with `FLATPAK_REPO_TOKEN`. Releases also
carry `Home-Sync-x86_64.flatpak` as a single-file download (no updates); pull requests
touching what Home Sync is built from keep it as a workflow artifact.

It runs on the GNOME 51 runtime (GTK4 and WebKitGTK 6.0 included, fetched from Flathub on
install) with access to the home folder only. A cloud folder outside the home folder isn't
reachable from inside the Flatpak. Building locally needs `flatpak-builder`:

```fish
flatpak-builder --user --install-deps-from=flathub --force-clean --repo=flatpak-repo flatpak-build gui/build/flatpak/org.s3r3nity.HomeSync.yml
flatpak build-bundle flatpak-repo Home-Sync-x86_64.flatpak org.s3r3nity.HomeSync
```

(from the repository root, branch "master" unless `--default-branch=stable`).

**Ubuntu: not from VSCode's terminal.** Ubuntu ships an AppArmor profile for VSCode
(`/etc/apparmor.d/code`) that every process started from its integrated terminal inherits;
WebKitGTK's web process can't start under it, so the window stays empty (`WebProcess
CRASHED`, `EGL_BAD_PARAMETER`). Run `wails3 dev` from a normal terminal instead. Systems
without AppArmor (CachyOS, Arch) aren't affected, and neither is Home Sync started from
the app menu.

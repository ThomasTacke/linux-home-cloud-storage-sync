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
runs after confirming. AppImage releases come next.

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

**Ubuntu 24.04:** WebKitGTK's web process can't start there with the default AppArmor
restriction on unprivileged user namespaces (`kernel.apparmor_restrict_unprivileged_userns=1`):
the window stays empty, `WebProcess CRASHED`. Arch-based systems (CachyOS, Bazzite's
Fedora base) don't have this restriction.

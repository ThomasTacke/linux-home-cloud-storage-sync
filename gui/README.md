# Home Sync

The desktop app of linux-home-cloud-storage-sync, for people who'd rather not use a
terminal. Same logic as the CLI and the TUI (`internal/app` in the parent module), shown
in plain words.

Built with [Wails 3](https://v3.wails.io) (Go backend, system WebView), Svelte 5,
Tailwind CSS 4 and [shadcn-svelte](https://shadcn-svelte.com).

**Status:** phase 1, read-only. It shows every entry of the config and whether it's in
sync. Actions, a first-run wizard and AppImage releases come next.

## Development

Needs Go 1.26.7+, Node 22+ (26 tested), and GTK4 and WebKitGTK 6.0 with headers
(Ubuntu: `libgtk-4-dev libwebkitgtk-6.0-dev`, Arch: `gtk4 webkitgtk-6.0`). Then:

```fish
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
wails3 doctor      # checks the setup
wails3 dev         # runs the app with hot reload
wails3 build       # bin/homesync
```

- `service.go` – `SyncService`, the Go methods the frontend calls; `wails3 generate
  bindings -ts` turns them into `frontend/bindings` (done by `wails3 build`/`dev` too)
- `frontend/src/App.svelte` – the window; `frontend/src/lib/status.ts` – the words for
  each state; `frontend/src/lib/components/ui` – shadcn-svelte components
- `go.mod` uses the parent module through `replace … => ../`, so the CLI's `go.mod`
  stays free of Wails and cgo

**Ubuntu 24.04:** WebKitGTK's web process can't start there with the default AppArmor
restriction on unprivileged user namespaces (`kernel.apparmor_restrict_unprivileged_userns=1`):
the window stays empty, `WebProcess CRASHED`. Arch-based systems (CachyOS, Bazzite's
Fedora base) don't have this restriction.

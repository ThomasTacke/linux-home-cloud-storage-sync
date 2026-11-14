module github.com/s3r3nity-labs/linux-home-cloud-storage-sync/gui

go 1.26.7

require github.com/wailsapp/wails/v3 v3.1.0

require github.com/s3r3nity-labs/linux-home-cloud-storage-sync v0.0.0

require (
	github.com/coder/websocket v1.8.14 // indirect
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/term v0.46.0 // indirect
)

// The CLI module in the parent folder: internal/app, internal/config, internal/linker
replace github.com/s3r3nity-labs/linux-home-cloud-storage-sync => ../

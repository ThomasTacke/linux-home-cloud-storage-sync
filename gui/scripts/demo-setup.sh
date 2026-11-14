#!/usr/bin/env bash
# Creates a throwaway setup with one item in every state, for trying Home Sync's buttons
# without touching your real files:
#
#   gui/scripts/demo-setup.sh
#   HOME_SYNC_CONFIG=/tmp/homesync-demo/config.json wails3 dev
#
# Running it again starts over.
#
# With --fresh it builds a home without a config instead, for the first-run wizard: German
# folder names (like on a German desktop) and an empty Nextcloud folder:
#
#   gui/scripts/demo-setup.sh --fresh
#   env HOME_SYNC_CONFIG=/tmp/homesync-fresh/config.json HOME_SYNC_HOME=/tmp/homesync-fresh/home wails3 dev
set -euo pipefail

if [[ "${1:-}" == "--fresh" ]]; then
  fresh=/tmp/homesync-fresh
  home=$fresh/home
  rm -rf "$fresh"
  mkdir -p "$home/.config" "$home/Nextcloud"
  cat > "$home/.config/user-dirs.dirs" <<'DIRS'
XDG_DESKTOP_DIR="$HOME/Schreibtisch"
XDG_DOWNLOAD_DIR="$HOME/Downloads"
XDG_TEMPLATES_DIR="$HOME/Vorlagen"
XDG_DOCUMENTS_DIR="$HOME/Dokumente"
XDG_MUSIC_DIR="$HOME/Musik"
XDG_PICTURES_DIR="$HOME/Bilder"
XDG_VIDEOS_DIR="$HOME/Videos"
DIRS
  for d in Schreibtisch Downloads Dokumente Bilder Musik; do mkdir -p "$home/$d"; done
  echo "Brief an Oma" > "$home/Dokumente/brief.txt"
  echo "photo" > "$home/Bilder/urlaub.jpg"
  echo "Fresh home ready (no config yet):"
  echo "  env HOME_SYNC_CONFIG=$fresh/config.json HOME_SYNC_HOME=$home wails3 dev"
  exit 0
fi
demo=/tmp/homesync-demo
cloud=$demo/Nextcloud
home=$demo/home
rm -rf "$demo"
mkdir -p "$cloud" "$home"

mkdir -p "$cloud/Documents" && echo "letter" > "$cloud/Documents/letter.txt"   # in sync
ln -s "$cloud/Documents" "$home/Documents"
mkdir -p "$cloud/Pictures" && echo "photo" > "$cloud/Pictures/cat.jpg"         # ready to connect
mkdir -p "$home/Projects" && echo "code" > "$home/Projects/main.go"            # only on this computer
mkdir -p "$cloud/Music" "$home/Music"                                         # two versions
echo "cloud song" > "$cloud/Music/a.mp3"
echo "local song" > "$home/Music/b.mp3"
mkdir -p "$cloud/Videos" && ln -s "$demo/old-videos" "$home/Videos"           # needs repair
echo "[user]" > "$home/.gitconfig"                                            # hidden, only here
# "Templates" is in neither place: not found

cat > "$demo/config.json" <<JSON
{
    "cloudStorageHomeDir": "$cloud",
    "localHomeDir": "$home",
    "symlinkMap": ["Documents", "Pictures", "Projects", "Music", "Videos", "Templates", ".gitconfig"]
}
JSON
echo "Demo ready: HOME_SYNC_CONFIG=$demo/config.json"

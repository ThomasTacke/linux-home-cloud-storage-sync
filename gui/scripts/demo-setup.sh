#!/usr/bin/env bash
# Creates a throwaway setup with one item in every state, for trying Home Sync's buttons
# without touching your real files:
#
#   gui/scripts/demo-setup.sh
#   HOME_SYNC_CONFIG=/tmp/homesync-demo/config.json wails3 dev
#
# Running it again starts over.
set -euo pipefail
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

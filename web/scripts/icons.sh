#!/usr/bin/env bash
# Regenerates the PNG and ICO icons in web/public from the favicon.svg master.
# Needs rsvg-convert (librsvg) and ImageMagick 7 (magick). Run from anywhere.
set -euo pipefail
cd "$(dirname "$0")/../public"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
# iOS and Android round the corners themselves: give them a full square.
sed 's/ rx="14"//' favicon.svg >"$tmp/square.svg"
for s in 16 32 48; do rsvg-convert -w $s -h $s favicon.svg -o "$tmp/$s.png"; done
magick "$tmp/16.png" "$tmp/32.png" "$tmp/48.png" favicon.ico
rsvg-convert -w 180 -h 180 "$tmp/square.svg" -o apple-touch-icon.png
rsvg-convert -w 192 -h 192 "$tmp/square.svg" -o icon-192.png
rsvg-convert -w 512 -h 512 "$tmp/square.svg" -o icon-512.png

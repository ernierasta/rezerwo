#!/bin/sh
# Odtwarza fonty Red Hat Text / Red Hat Display w css/fonts/ (są w repozytorium,
# skrypt potrzebny tylko przy aktualizacji fontów).
#
# Interfejs (scss/ui.scss) używa ich przez @font-face z lokalnych plików,
# a nie z Google Fonts (RODO + działanie bez zewnętrznych zasobów).
# Z plików zmiennej grubości (google/fonts, licencja SIL OFL 1.1) robimy
# statyczne grubości z @font-face: Text 400/500/600, Display 500.
#
# wymaga: curl, fonttools z brotli (pacman -S python-fonttools python-brotli)
# usage (z katalogu repozytorium): tools/download-fonts.sh

set -e

OUT=css/fonts
BASE=https://raw.githubusercontent.com/google/fonts/main/ofl
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$OUT"

curl -fsSL "$BASE/redhattext/RedHatText%5Bwght%5D.ttf" -o "$TMP/RedHatText.ttf"
curl -fsSL "$BASE/redhatdisplay/RedHatDisplay%5Bwght%5D.ttf" -o "$TMP/RedHatDisplay.ttf"
curl -fsSL "$BASE/redhattext/OFL.txt" -o "$OUT/OFL.txt"

for spec in "RedHatText 400 Regular" "RedHatText 500 Medium" "RedHatText 600 SemiBold" "RedHatDisplay 500 Medium"; do
	set -- $spec
	fonttools varLib.instancer -q "$TMP/$1.ttf" wght="$2" -o "$TMP/$1-$3.ttf"
	fonttools ttLib --flavor woff2 -o "$OUT/$1-$3.woff2" "$TMP/$1-$3.ttf"
	echo "gotowe: $OUT/$1-$3.woff2"
done

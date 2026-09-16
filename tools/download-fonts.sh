#!/bin/sh
# Pobiera fonty Red Hat Text / Red Hat Display do media/fonts/.
#
# Nowy interfejs (scss/ui.scss) używa ich przez @font-face z lokalnych plików,
# a nie z Google Fonts (RODO + działanie bez zewnętrznych zasobów). Bez tych
# plików strony działają dalej, tylko w foncie systemowym.
#
# usage (z katalogu repozytorium): tools/download-fonts.sh

set -e

DIR=media/fonts
mkdir -p "$DIR"

# Pliki woff2 z repozytorium google/fonts (licencja SIL OFL 1.1).
BASE=https://raw.githubusercontent.com/google/fonts/main/ofl

fetch() {
	url="$1"
	out="$2"
	if [ -f "$out" ]; then
		echo "jest już: $out"
		return
	fi
	echo "pobieram: $out"
	curl -fsSL "$url" -o "$out"
}

# Red Hat Text i Display są w google/fonts jako pliki zmiennej grubości (ttf).
# Zamieniamy je na woff2 narzędziem woff2_compress, jeżeli jest dostępne;
# inaczej zostawiamy ttf i trzeba poprawić rozszerzenia w scss/ui.scss.
fetch "$BASE/redhattext/RedHatText%5Bwght%5D.ttf" "$DIR/RedHatText.ttf"
fetch "$BASE/redhatdisplay/RedHatDisplay%5Bwght%5D.ttf" "$DIR/RedHatDisplay.ttf"

if command -v woff2_compress >/dev/null 2>&1; then
	woff2_compress "$DIR/RedHatText.ttf"
	woff2_compress "$DIR/RedHatDisplay.ttf"
	echo "gotowe: $DIR/*.woff2 (zmienna grubość - popraw @font-face w scss/ui.scss na jeden plik z font-weight 400 500 600)"
else
	echo "uwaga: brak woff2_compress (pakiet woff2), zostały pliki .ttf"
fi

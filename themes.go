package main

import (
	"html/template"
	"strings"
)

// Tematy graficzne stron dla gości (rezerwacja, zamówienie, formularz).
// Temat wybiera administrator w ustawieniach imprezy albo formularza
// (kolumna theme w events i formtemplates, tools/migration-themes.sql).
// Kolory są w scss/_themes.scss jako zestawy zmiennych pod
// [data-theme="<klucz>"]; pusty klucz = domyślny temat "klasyczny".
// Panel admina i strony bez imprezy/formularza (logowanie, błędy) mają
// kolory z :root, czyli "pastel".

// Tematy organizacji (Logo: true) mają też znak z logo - przed nazwą imprezy
// i blady w rogu tła. Znak jest opcjonalny i domyślnie wyłączony: włączony
// zapisuje się w kolumnie theme jako "<klucz>;logo" (bez migracji), na stronie
// gościa daje atrybut data-logo na <html>.

// GuestTheme - pozycja w wyborze tematu w panelu admina (szablon "themepick"
// w n_base.html; próbki malują się zmiennymi CSS samego tematu).
type GuestTheme struct {
	Key  string
	Dark bool
	Logo bool // temat ma znak organizacji do włączenia
}

const themeLogoSuffix = ";logo"

const DEFAULTTHEME = "klasyczny"

var guestThemes = []GuestTheme{
	{Key: "klasyczny"},
	{Key: "pastel"},
	{Key: "piasek"},
	{Key: "szalwia"},
	{Key: "macierz", Logo: true},
	{Key: "pzko", Logo: true},
	{Key: "noc", Dark: true},
	{Key: "grafit", Dark: true},
	{Key: "sliwka", Dark: true},
}

// ThemeKey zwraca znany klucz tematu (bez ";logo") albo domyślny.
func ThemeKey(key string) string {
	key = strings.TrimSuffix(strings.TrimSpace(key), themeLogoSuffix)
	for _, t := range guestThemes {
		if t.Key == key {
			return key
		}
	}
	return DEFAULTTHEME
}

// ThemeLogo - czy zapisany temat ma włączony znak organizacji (i go ma).
func ThemeLogo(value string) bool {
	value = strings.TrimSpace(value)
	if !strings.HasSuffix(value, themeLogoSuffix) {
		return false
	}
	key := ThemeKey(value)
	for _, t := range guestThemes {
		if t.Key == key {
			return t.Logo
		}
	}
	return false
}

// ThemeValue - wartość kolumny theme: znany klucz i ";logo", gdy znak jest
// włączony, a temat go ma.
func ThemeValue(key string, logo bool) string {
	key = ThemeKey(key)
	if logo && ThemeLogo(key+themeLogoSuffix) {
		return key + themeLogoSuffix
	}
	return key
}

// ThemeNorm - zapisana wartość po sprawdzeniu (kopiowanie imprezy/formularza).
func ThemeNorm(value string) string {
	return ThemeValue(value, ThemeLogo(value))
}

// themeAttr - wartość atrybutu data-theme strony imprezy/formularza.
func themeAttr(key string) string {
	return ThemeKey(key)
}

// ThemeName - nazwa tematu w języku panelu (locales: theme.<klucz>).
func ThemeName(lang, key string) string {
	return T(lang, "theme."+ThemeKey(key))
}

// GuestFuncs - funkcje szablonów strony gościa: TmplFuncs + temat.
func GuestFuncs(lang, theme string) template.FuncMap {
	f := TmplFuncs(lang)
	f["theme"] = func() string { return themeAttr(theme) }
	f["themelogo"] = func() bool { return ThemeLogo(theme) }
	return f
}

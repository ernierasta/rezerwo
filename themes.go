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

// GuestTheme - pozycja w wyborze tematu w panelu admina (szablon "themepick"
// w n_base.html; próbki malują się zmiennymi CSS samego tematu).
type GuestTheme struct {
	Key  string
	Dark bool
}

const DEFAULTTHEME = "klasyczny"

var guestThemes = []GuestTheme{
	{Key: "klasyczny"},
	{Key: "pastel"},
	{Key: "piasek"},
	{Key: "szalwia"},
	{Key: "noc", Dark: true},
	{Key: "grafit", Dark: true},
	{Key: "sliwka", Dark: true},
}

// ThemeKey zwraca znany klucz tematu albo domyślny.
func ThemeKey(key string) string {
	key = strings.TrimSpace(key)
	for _, t := range guestThemes {
		if t.Key == key {
			return key
		}
	}
	return DEFAULTTHEME
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
	return f
}

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Teksty panelu wołane z JS (t / RZ.t / RZ.tN) muszą istnieć we wszystkich
// językach. Brak klucza to nie pusty napis, tylko sam klucz na ekranie - tak
// w panelu wychodziło "count.seat.many", bo skrypt sam doklejał końcówkę
// ".many", której w katalogu angielskim nie ma (jest one/other), a po polsku
// psuło się "1 miejsc" zamiast "1 miejsce".
//
// Test obejmuje wszystkie skrypty panelu, nie tylko strony gościa (tam
// sprawdza to TestGuestTextsInAllLanguages).

var (
	jsTextKey  = regexp.MustCompile(`\b(?:RZ\.)?t\(\s*'([a-z0-9_.]+)'`)
	jsCountKey = regexp.MustCompile(`\b(?:RZ\.)?tN\(\s*'([a-z0-9_.]+)'`)
)

// pluralForms zwraca końcówki, jakie może wybrać pluralForm() dla danego
// języka (ta sama funkcja jest w js/n_ui.js). Sprawdzamy tylko te, które ten
// język faktycznie używa.
func pluralForms(lang string) []string {
	switch lang {
	case "pl", "cs", "sk":
		return []string{"one", "few", "many"}
	default:
		return []string{"one", "other"}
	}
}

func TestPanelScriptTextsInAllLanguages(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	scripts, err := filepath.Glob("js/n_*.js")
	if err != nil || len(scripts) == 0 {
		t.Fatalf("no panel scripts found: %v", err)
	}

	for _, f := range scripts {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range jsTextKey.FindAllStringSubmatch(string(src), -1) {
			key := m[1]
			// klucz budowany w locie: t('status.' + s), t('newmail.draft_' + x)
			// - rozwinięcia nie da się sprawdzić statycznie
			if strings.HasSuffix(key, "_") || strings.HasSuffix(key, ".") {
				continue
			}
			for _, lang := range allLangs {
				checkKey(t, f, lang, key)
			}
		}
		// tN: sprawdzamy każdą końcówkę, jaką pluralForm() może wybrać
		for _, m := range jsCountKey.FindAllStringSubmatch(string(src), -1) {
			for _, lang := range allLangs {
				for _, suf := range pluralForms(lang) {
					checkKey(t, f, lang, m[1]+"."+suf)
				}
			}
		}
	}
}

func checkKey(t *testing.T, file, lang, key string) {
	t.Helper()
	localesMu.RLock()
	_, ok := locales[lang][key]
	localesMu.RUnlock()
	if !ok {
		t.Errorf("%s: key %q missing in %s", file, key, lang)
	}
}

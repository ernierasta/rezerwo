package main

// i18n.go: katalogi tekstów interfejsu (locales/*.toml).
//
// Teksty interfejsu tłumaczymy my, raz; treści organizacji (opisy imprez, maile)
// zostają w bazie w języku imprezy. Katalog jest płaską mapą kluczy
// ("event.sec.basic"), powstałą ze sklejenia nazw tabel TOML kropką.
//
// Ten sam katalog jest wystawiany dla JS pod /locales/{lang}.json.

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
)

// LOCALESDIR - katalog z plikami tłumaczeń.
const LOCALESDIR = "locales"

// DEFAULTLANG - język używany, gdy klucza nie ma w wybranym katalogu.
const DEFAULTLANG = "pl"

// PANELLANGCOOKIE - ciasteczko z językiem panelu admina.
const PANELLANGCOOKIE = "rezerwo-panel-lang"

var (
	localesMu sync.RWMutex
	locales   = map[string]map[string]string{}
	localesJS = map[string][]byte{}
)

// LoadLocales wczytuje wszystkie locales/*.toml. Woła się raz przy starcie.
func LoadLocales() error {
	files, err := filepath.Glob(filepath.Join(LOCALESDIR, "*.toml"))
	if err != nil {
		return err
	}
	loaded := map[string]map[string]string{}
	loadedJS := map[string][]byte{}

	for _, f := range files {
		lang := strings.TrimSuffix(filepath.Base(f), ".toml")
		var raw map[string]interface{}
		if _, err := toml.DecodeFile(f, &raw); err != nil {
			return fmt.Errorf("i18n: %s: %w", f, err)
		}
		flat := map[string]string{}
		flatten("", raw, flat)
		loaded[lang] = flat

		js, err := json.Marshal(flat)
		if err != nil {
			return fmt.Errorf("i18n: %s: %w", f, err)
		}
		loadedJS[lang] = js
	}

	if _, ok := loaded[DEFAULTLANG]; !ok {
		return fmt.Errorf("i18n: missing %s/%s.toml", LOCALESDIR, DEFAULTLANG)
	}

	localesMu.Lock()
	locales, localesJS = loaded, loadedJS
	localesMu.Unlock()

	log.Printf("i18n: loaded %d catalogues from %s/", len(loaded), LOCALESDIR)
	return nil
}

func flatten(prefix string, in map[string]interface{}, out map[string]string) {
	for k, v := range in {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case string:
			out[key] = val
		case map[string]interface{}:
			flatten(key, val, out)
		default:
			out[key] = fmt.Sprint(val)
		}
	}
}

// KnownLang zwraca język, jeżeli mamy dla niego katalog, inaczej DEFAULTLANG.
func KnownLang(lang string) string {
	localesMu.RLock()
	defer localesMu.RUnlock()
	if _, ok := locales[lang]; ok {
		return lang
	}
	return DEFAULTLANG
}

// T zwraca przetłumaczony tekst. Jeżeli podano argumenty, działa jak Sprintf.
// Brakujący klucz zwraca sam klucz - widać go na stronie i łatwo znaleźć.
func T(lang, key string, args ...interface{}) string {
	localesMu.RLock()
	s, ok := locales[lang][key]
	if !ok {
		s, ok = locales[DEFAULTLANG][key]
	}
	localesMu.RUnlock()

	if !ok {
		return key
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

// TN zwraca tekst z liczbą w poprawnej formie: klucz + ".one"/".few"/".many"/".other".
// Reguły CLDR dla pl i cs (few dla 2-4, many dla reszty), inne języki: one/other.
func TN(lang, key string, n int64) string {
	return T(lang, key+"."+pluralForm(lang, n), n)
}

func pluralForm(lang string, n int64) string {
	if n < 0 {
		n = -n
	}
	switch lang {
	case "pl", "cs", "sk":
		switch {
		case n == 1:
			return "one"
		case n >= 2 && n <= 4:
			return "few"
		default:
			return "many"
		}
	default:
		if n == 1 {
			return "one"
		}
		return "other"
	}
}

// TmplFuncs to funkcje szablonu dla stron w danym języku:
// {{T "klucz"}}, {{TN "count.seat" 5}}, {{money 1400 "Kč"}}.
func TmplFuncs(lang string) template.FuncMap {
	return template.FuncMap{
		"T": func(key string, args ...interface{}) string {
			return T(lang, key, args...)
		},
		"TN": func(key string, n int64) string {
			return TN(lang, key, n)
		},
		"lang":  func() string { return lang },
		"money": FormatMoney,
		"date":  func(unix int64) string { return FormatDate(lang, unix) },
	}
}

// FormatMoney formatuje kwotę z twardą spacją jako separatorem tysięcy: "1 400 Kč".
func FormatMoney(amount int64, currency string) string {
	neg := amount < 0
	if neg {
		amount = -amount
	}
	s := fmt.Sprintf("%d", amount)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteString(" ")
		}
		b.WriteRune(r)
	}
	out := b.String()
	if neg {
		out = "-" + out
	}
	if currency != "" {
		out += " " + currency
	}
	return out
}

// LocalesJSONHandler wystawia katalog dla JS: /locales/pl.json.
func LocalesJSONHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := KnownLang(strings.TrimSuffix(filepath.Base(r.URL.Path), ".json"))

		localesMu.RLock()
		js := localesJS[lang]
		localesMu.RUnlock()

		if js == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(js)
	}
}

// PanelLang zwraca język panelu admina: ciasteczko, potem Accept-Language, potem pl.
// (Kolumny admins.language jeszcze nie ma - dojdzie razem z ustawieniami organizacji.)
func PanelLang(r *http.Request) string {
	if c, err := r.Cookie(PANELLANGCOOKIE); err == nil && c.Value != "" {
		return KnownLang(c.Value)
	}
	for _, part := range strings.Split(r.Header.Get("Accept-Language"), ",") {
		tag := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if tag == "" {
			continue
		}
		base := strings.ToLower(strings.SplitN(tag, "-", 2)[0])
		localesMu.RLock()
		_, ok := locales[base]
		localesMu.RUnlock()
		if ok {
			return base
		}
	}
	return DEFAULTLANG
}

// SetPanelLangHandler zapisuje wybrany język panelu w ciasteczku: /admin/lang?l=cs.
func SetPanelLangHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := KnownLang(r.URL.Query().Get("l"))
		http.SetCookie(w, &http.Cookie{
			Name:     PANELLANGCOOKIE,
			Value:    lang,
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 365,
			HttpOnly: false,
		})
		back := r.Referer()
		if back == "" {
			back = "/admin"
		}
		http.Redirect(w, r, back, http.StatusSeeOther)
	}
}

// localeFileExists - drobna pomoc przy starcie (jasny komunikat zamiast pustych stron).
func localesDirExists() bool {
	st, err := os.Stat(LOCALESDIR)
	return err == nil && st.IsDir()
}

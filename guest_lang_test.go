package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// Zasada: strona gościa jest ZAWSZE w języku imprezy/formularza, bez wyjątku.
// Język przeglądarki (Accept-Language, ciasteczko panelu admina) nie ma
// znaczenia, a przeglądarka nie może pokazać gościowi niczego od siebie
// (dymki walidacji, kalendarz, okna alert/confirm, "Opuścić stronę?").
// Wielu gości ma czeską przeglądarkę, a imprezy są po polsku.

var allLangs = []string{"pl", "cs", "en"}

// browserHeaders udaje gościa z przeglądarką w innym języku niż strona
// (i z ciasteczkiem języka panelu - np. admin oglądający stronę gościa).
func browserHeaders(r *http.Request, pageLang string) {
	other := "cs"
	if pageLang == "cs" {
		other = "en"
	}
	r.Header.Set("Accept-Language", other+","+other+"-CZ;q=0.9")
	r.AddCookie(&http.Cookie{Name: PANELLANGCOOKIE, Value: other})
}

type guestFixture struct {
	db      *DB
	userID  int64
	eventID int64
	roomID  int64
	router  *mux.Router
}

func newGuestFixture(t *testing.T) *guestFixture {
	t.Helper()
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	db := newTestDB(t)
	f := &guestFixture{db: db}
	f.userID = addUser(t, db, "org@example.com", "org")
	f.roomID = addRoom(t, db, "Sala", 3)
	f.eventID = addEvent(t, db, f.userID, "Bal")
	if err := db.EventAddRoom(f.eventID, f.roomID); err != nil {
		t.Fatal(err)
	}
	content := `[
	 {"type":"text","subtype":"email","name":"email-REQUIRED","label":"E-mail"},
	 {"type":"text","subtype":"text","name":"name-REQUIRED","label":"Imię"},
	 {"type":"date","name":"termin","label":"Termin","value":"2026-12-18","min":"2026-01-01","required":true},
	 {"type":"text","subtype":"color","name":"kolor","label":"Kolor"},
	 {"type":"number","name":"ile","label":"Ile","min":"0","step":"50"}
	]`
	must(t)(db.FormTemplateAdd(&FormTemplate{Name: "Zgłoszenie", URL: "zgloszenie",
		Content: sql.NullString{String: content, Valid: true}, UserID: f.userID, CreatedDate: 1}))

	cs := sessions.NewCookieStore([]byte("0123456789abcdef0123456789abcdef"))
	f.router = mux.NewRouter()
	f.router.HandleFunc("/res/{user}", GuestReservationPage(db, cs))
	f.router.HandleFunc("/order", GuestOrderPage(db))
	f.router.HandleFunc("/form/{userurl}/{formurl}", GuestFormPage(db, cs))
	return f
}

func (f *guestFixture) setLang(t *testing.T, lang string) {
	t.Helper()
	if _, err := f.db.DB.Exec(`UPDATE events SET language=$1 WHERE id=$2`, lang, f.eventID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.DB.Exec(`UPDATE formtemplates SET language=$1, status='open'`, lang); err != nil {
		t.Fatal(err)
	}
}

func (f *guestFixture) do(t *testing.T, r *http.Request, pageLang string) string {
	t.Helper()
	browserHeaders(r, pageLang)
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, r)
	return w.Body.String()
}

func (f *guestFixture) orderPage(t *testing.T, lang string) string {
	t.Helper()
	form := url.Values{
		"event-id": {fmt.Sprint(f.eventID)}, "sits": {"1"}, "rooms": {fmt.Sprint(f.roomID)},
		"prices": {"100"}, "total-price": {"100"}, "default-currency": {"Kč"},
	}
	r := httptest.NewRequest("POST", "/order", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body := f.do(t, r, lang)
	// zwolnij miejsce dla następnego języka
	if _, err := f.db.DB.Exec(`DELETE FROM reservations`); err != nil {
		t.Fatal(err)
	}
	return body
}

// wantLang sprawdza, że strona jest w języku lang: atrybut <html lang> i
// teksty z katalogu tego języka, a nie innych.
func wantLang(t *testing.T, page, body, lang string, keys ...string) {
	t.Helper()
	if !strings.Contains(body, `<html lang="`+lang+`"`) {
		t.Errorf("%s [%s]: missing <html lang=%q>", page, lang, lang)
	}
	for _, key := range keys {
		want := T(lang, key)
		if want == key {
			t.Fatalf("%s: key %q missing in locales", page, key)
		}
		if !strings.Contains(body, want) {
			t.Errorf("%s [%s]: missing %q (%s)", page, lang, want, key)
		}
		for _, other := range allLangs {
			if s := T(other, key); other != lang && s != want && strings.Contains(body, s) {
				t.Errorf("%s [%s]: contains %s text %q (%s)", page, lang, other, s, key)
			}
		}
	}
}

func TestGuestPagesInEventLanguage(t *testing.T) {
	f := newGuestFixture(t)
	for _, lang := range allLangs {
		f.setLang(t, lang)

		r := httptest.NewRequest("GET", fmt.Sprintf("/res/org?event-id=%d", f.eventID), nil)
		wantLang(t, "reservation", f.do(t, r, lang), lang, "guest.nothing_selected")

		wantLang(t, "order", f.orderPage(t, lang), lang, "order.submit", "order.email", "order.cancel")

		r = httptest.NewRequest("GET", "/form/org/zgloszenie", nil)
		wantLang(t, "form", f.do(t, r, lang), lang, "gform.submit", "gform.date_ph")
	}
}

func TestGuestErrorsInEventLanguage(t *testing.T) {
	f := newGuestFixture(t)
	if _, err := f.db.DB.Exec(`DELETE FROM events_rooms`); err != nil {
		t.Fatal(err)
	}
	for _, lang := range allLangs {
		f.setLang(t, lang)
		r := httptest.NewRequest("GET", fmt.Sprintf("/res/org?event-id=%d", f.eventID), nil)
		wantLang(t, "no rooms", f.do(t, r, lang), lang, "guest.no_rooms_title")

		// bez otwartej imprezy: język ostatniej imprezy organizacji
		r = httptest.NewRequest("GET", "/res/org?event-id=999", nil)
		wantLang(t, "no event", f.do(t, r, lang), lang, "guest.no_event_title")
	}
}

// Formularze gościa: novalidate (własne komunikaty zamiast dymków
// przeglądarki) i żadnych pól, które przeglądarka rysuje po swojemu.
func TestGuestFormsNoBrowserUI(t *testing.T) {
	f := newGuestFixture(t)
	f.setLang(t, "pl")
	pages := map[string]string{
		"order": f.orderPage(t, "pl"),
		"form":  f.do(t, httptest.NewRequest("GET", "/form/org/zgloszenie", nil), "pl"),
	}
	formTag := regexp.MustCompile(`<form[^>]*>`)
	nativeInput := regexp.MustCompile(`<input[^>]*\stype="(date|time|datetime-local|month|week|color|file|password)"`)
	for name, body := range pages {
		tags := formTag.FindAllString(body, -1)
		if len(tags) == 0 {
			t.Errorf("%s: no <form>", name)
		}
		for _, tag := range tags {
			if !strings.Contains(tag, "novalidate") {
				t.Errorf("%s: %s without novalidate", name, tag)
			}
		}
		if m := nativeInput.FindString(body); m != "" {
			t.Errorf("%s: browser-drawn input %s", name, m)
		}
	}
	form := pages["form"]
	if !strings.Contains(form, `data-date`) || !strings.Contains(form, `value="18.12.2026"`) {
		t.Errorf("form: date field should be a text input with dd.mm.yyyy value")
	}
}

// Skrypty stron gościa nie mogą wołać okien przeglądarki - ich treść
// i przyciski są w języku przeglądarki.
func TestGuestScriptsNoBrowserDialogs(t *testing.T) {
	banned := regexp.MustCompile(`\b(alert|confirm|prompt)\(|beforeunload|reportValidity\(|navigator\.languages?\b|toLocale\w*String\(\)`)
	for _, f := range guestScripts(t) {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			code := strings.TrimSpace(line)
			if strings.HasPrefix(code, "//") {
				continue
			}
			if m := banned.FindString(code); m != "" {
				t.Errorf("%s:%d: %s - shows browser-language UI to guests", f, i+1, m)
			}
		}
	}
}

// Każdy tekst używany na stronach gościa istnieje we wszystkich językach
// (brak klucza = tekst domyślny po polsku na czeskiej stronie).
func TestGuestTextsInAllLanguages(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	tmplKey := regexp.MustCompile(`\{\{\s*TN? "([a-z0-9_.]+)"`)
	jsKey := regexp.MustCompile(`\b(?:RZ\.)?t\('([a-z0-9_.]+)'`)
	files := append(guestScripts(t), guestTemplates...)
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		re := tmplKey
		if strings.HasSuffix(f, ".js") {
			re = jsKey
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			key := m[1]
			for _, lang := range allLangs {
				localesMu.RLock()
				_, ok := locales[lang][key]
				if !ok { // TN: klucz z końcówką liczby mnogiej
					_, ok = locales[lang][key+".other"]
				}
				localesMu.RUnlock()
				if !ok {
					t.Errorf("%s: key %q missing in %s", f, key, lang)
				}
			}
		}
	}
}

var guestTemplates = []string{
	"tmpl/n_guest_base.html", "tmpl/n_reservation.html", "tmpl/n_order.html",
	"tmpl/n_formpage.html", "tmpl/n_formfield.html", "tmpl/n_form_done.html",
	"tmpl/n_error.html",
}

// guestScripts to skrypty ładowane przez szablony stron gościa.
func guestScripts(t *testing.T) []string {
	t.Helper()
	src := regexp.MustCompile(`src="(?:\{\{asset ")?/(js/n_[a-z_]+\.js)`)
	seen := map[string]bool{}
	var out []string
	for _, f := range guestTemplates {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range src.FindAllStringSubmatch(string(b), -1) {
			if p := filepath.Clean(m[1]); !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	if len(out) < 4 {
		t.Fatalf("found only %v guest scripts", out)
	}
	return out
}

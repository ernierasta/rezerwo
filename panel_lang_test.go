package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
)

// Zasada: panel admina jest po polsku, dopóki admin sam go nie przełączy.
// Język przeglądarki (Accept-Language) nie ma wpływu na PanelLang - czeska
// przeglądarka nie może przełączyć panelu na czeski bez jego woli.
// Wybór (PL/CZ/EN w n_base.html) zapisujemy w ciasteczku i na koncie admina.

func panelReq(accept, cookie string) *http.Request {
	r := httptest.NewRequest("GET", "/admin", nil)
	if accept != "" {
		r.Header.Set("Accept-Language", accept)
	}
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: PANELLANGCOOKIE, Value: cookie})
	}
	return r
}

// PanelRenderDoesNotStoreLanguage: samo wejście do panelu (tu strona logowania,
// którą admin z czeską przeglądarką widzi jako pierwszą) nie może zapisać
// ciasteczka języka. Ciasteczko powstaje wyłącznie od kliknięcia PL/CZ/EN
// (SetPanelLangHandler) albo z języka zapisanego na koncie admina - inaczej
// czeska przeglądarka "zapamiętałaby" sobie czeski i panel już nigdy nie
// wróciłby do polskiego.
func TestPanelRenderDoesNotStoreLanguage(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	r := panelReq("cs-CZ,cs;q=0.9", "")
	w := httptest.NewRecorder()
	AdminLoginHTML()(w, r)

	res := w.Result()
	for _, c := range res.Cookies() {
		if c.Name == PANELLANGCOOKIE {
			t.Errorf("panel set %s=%q just by being opened (Accept-Language cs), want no cookie", c.Name, c.Value)
		}
	}
	body := w.Body.String()
	if !strings.Contains(body, `<html lang="pl"`) {
		t.Errorf("login page not in pl, got lang=%q", regexp.MustCompile(`<html lang="(\w+)"`).FindStringSubmatch(body))
	}
	if want, other := T("pl", "login.title"), T("cs", "login.title"); want != other && !strings.Contains(body, want) {
		t.Errorf("login page missing Polish title %q", want)
	}
}

// PanelLangIgnoresBrowserLanguage: czeska przeglądarka = polski panel.
func TestPanelLangIgnoresBrowserLanguage(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	for _, accept := range []string{"", "cs", "cs-CZ,cs;q=0.9", "en-US,en;q=0.9", "sk-SK,cs;q=0.8,en;q=0.6"} {
		if got := PanelLang(panelReq(accept, "")); got != DEFAULTLANG {
			t.Errorf("PanelLang with Accept-Language %q = %q, want %q", accept, got, DEFAULTLANG)
		}
	}
	// ciasteczko z wyboru admina ma pierwszeństwo przed językiem przeglądarki
	if got := PanelLang(panelReq("cs-CZ,cs;q=0.9", "en")); got != "en" {
		t.Errorf("PanelLang with cookie en = %q, want en", got)
	}
	// nieznane albo uszkodzone ciasteczko to po prostu brak wyboru: pl
	for _, c := range []string{"xx", "pl-PL", " "} {
		if got := PanelLang(panelReq("", c)); got != DEFAULTLANG {
			t.Errorf("PanelLang with cookie %q = %q, want %q", c, got, DEFAULTLANG)
		}
	}
}

// /admin/lang?l=cs zapisuje wybór w ciasteczku (bez sesji) i na koncie admina,
// a kolejne żądania z tym ciasteczkiem są już po czesku.
func TestSetPanelLangRemembersChoice(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	db := newTestDB(t)
	userID := addUser(t, db, "org@example.com", "org")
	email := "admin@example.com"
	must(t)(db.AdminAdd(&Admin{Type: "admin", Email: email, UserID: userID}))

	cs := sessions.NewCookieStore([]byte("0123456789abcdef0123456789abcdef"))
	h := SetPanelLangHandler(db, cs)

	w := httptest.NewRecorder()
	h(w, httptest.NewRequest("GET", "/admin/lang?l=cs", nil))

	cookies := w.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name != PANELLANGCOOKIE {
			continue
		}
		found = true
		if c.Value != "cs" || c.MaxAge <= 0 {
			t.Errorf("cookie %s = %q (MaxAge %d), want cs remembered for a year", c.Name, c.Value, c.MaxAge)
		}
		r := httptest.NewRequest("GET", "/admin", nil)
		r.AddCookie(c)
		if got := PanelLang(r); got != "cs" {
			t.Errorf("PanelLang after choosing cs = %q, want cs", got)
		}
	}
	if !found {
		t.Fatalf("no %s cookie set: %v", PANELLANGCOOKIE, cookies)
	}

	// nieznany język w URL nie zostawia śmieci w ciasteczku
	w = httptest.NewRecorder()
	h(w, httptest.NewRequest("GET", "/admin/lang?l=de", nil))
	r := httptest.NewRequest("GET", "/admin", nil)
	for _, c := range w.Result().Cookies() {
		if c.Name == PANELLANGCOOKIE {
			r.AddCookie(c)
		}
	}
	if got := PanelLang(r); got != DEFAULTLANG {
		t.Errorf("PanelLang after /admin/lang?l=de = %q, want %q", got, DEFAULTLANG)
	}
}

// Po zalogowaniu język z konta (admins.language) ląduje w ciasteczku, więc
// panel jest taki sam na innym komputerze, mimo że przeglądarka jest czeska.
func TestApplyAdminLanguageFromAccount(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	db := newTestDB(t)
	userID := addUser(t, db, "org@example.com", "org")
	email := "admin@example.com"
	must(t)(db.AdminAdd(&Admin{Type: "admin", Email: email, UserID: userID}))

	w := httptest.NewRecorder()
	ApplyAdminLanguage(w, db, email)
	if cookies := w.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("admin without saved language got cookies %v, want none", cookies)
	}

	if err := db.AdminSetLanguage(email, "cs"); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	ApplyAdminLanguage(w, db, email)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != PANELLANGCOOKIE || cookies[0].Value != "cs" {
		t.Fatalf("ApplyAdminLanguage = %v, want %s=cs", cookies, PANELLANGCOOKIE)
	}
	r := panelReq("cs-CZ,cs;q=0.9", cookies[0].Value)
	if got := PanelLang(r); got != "cs" {
		t.Errorf("PanelLang after login = %q, want cs", got)
	}
}

// Nigdzie w kodzie nie wolno czytać języka przeglądarki (Accept-Language):
// panel startuje po polsku i zmienia język tylko wyborem admina.
func TestNoAcceptLanguageUse(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no sources found: %v", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			code := strings.TrimSpace(line)
			if strings.HasPrefix(code, "//") {
				continue // komentarz wyjaśniający, nie kod
			}
			if strings.Contains(code, `Get("Accept-Language")`) || strings.Contains(code, `["Accept-Language"]`) {
				t.Errorf("%s:%d: %s - panel language must not follow the browser", f, i+1, code)
			}
		}
	}
}

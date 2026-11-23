package main

// Strony z linkami (linkpages.go): zapis z panelu, sprawdzanie przycisków,
// strona gościa, kod QR i dostęp menedżerów.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

func TestCleanLinkURL(t *testing.T) {
	for in, want := range map[string]string{
		"https://www.zori.cz/a?b=1": "https://www.zori.cz/a?b=1",
		"http://example.com":        "http://example.com",
		"www.zori.cz":               "https://www.zori.cz",
		"/form/org/zgloszenie":      "/form/org/zgloszenie",
		"javascript:alert(1)":       "",
		"//evil.example.com":        "",
		"ftp://example.com":         "",
		"mailto:a@example.com":      "",
		"https://":                  "",
		"https://a b.cz":            "",
		"":                          "",
	} {
		if got := cleanLinkURL(in); got != want {
			t.Errorf("cleanLinkURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLinkPages(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	db := newTestDB(t)
	h, _ := hashPassword("owner-pass")
	orgID := must(t)(db.UserAdd(&User{Email: "org@example.com", URL: "org", Passwd: h}))
	ev := addEvent(t, db, orgID, "Bal Polski")
	draft := addEvent(t, db, orgID, "Piknik")
	if _, err := db.DB.Exec(`UPDATE events SET status='draft' WHERE id=$1`, draft); err != nil {
		t.Fatal(err)
	}
	form := must(t)(db.FormTemplateAdd(&FormTemplate{Name: "Zgłoszenie", URL: "zgloszenie", Content: ToNS(`[]`), UserID: orgID, CreatedDate: 1}))
	other := addUser(t, db, "x@example.com", "x")
	foreign := addEvent(t, db, other, "Cudza")

	cs := sessions.NewCookieStore([]byte("0123456789abcdef0123456789abcdef"))
	rtr := mux.NewRouter()
	rtr.HandleFunc("/admin", AdminStartPage(db, cs))
	rtr.HandleFunc("/admin/pages/{id:[0-9]+}", LinkPagePage(db, cs))
	rtr.HandleFunc("/api/pages", LinkPageCreateAPI(db, cs)).Methods("POST")
	rtr.HandleFunc("/api/pages/{id:[0-9]+}", LinkPageDeleteAPI(db, cs)).Methods("DELETE")
	rtr.HandleFunc("/api/pages/{id:[0-9]+}/section", LinkPageSectionSave(db, cs)).Methods("POST")
	rtr.HandleFunc("/api/pages/{id:[0-9]+}/qr.{ext:png|svg}", LinkPageQRAPI(db, cs)).Methods("GET")
	rtr.HandleFunc("/p/{userurl}/{pageurl}", GuestLinkPage(db, cs))
	rtr.HandleFunc("/api/managers", ManagerCreateAPI(db, cs)).Methods("POST")
	rtr.HandleFunc("/api/login", LoginAPI(db, cs))

	do := func(method, path, body string, cookies []*http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		rtr.ServeHTTP(rec, req)
		return rec
	}
	login := func(email, pass string) []*http.Cookie {
		t.Helper()
		rec := do("POST", "/api/login", `{"email":"`+email+`","password":"`+pass+`"}`, nil)
		if rec.Code != 200 {
			t.Fatalf("login %s: %d %s", email, rec.Code, rec.Body)
		}
		return rec.Result().Cookies()
	}
	owner := login("org@example.com", "owner-pass")

	// nowa strona: adres z nazwy, drugi raz z -2
	create := func(name string) int64 {
		t.Helper()
		rec := do("POST", "/api/pages", `{"name":"`+name+`"}`, owner)
		if rec.Code != 200 {
			t.Fatalf("create: %d %s", rec.Code, rec.Body)
		}
		var resp struct{ ID int64 }
		json.Unmarshal(rec.Body.Bytes(), &resp)
		return resp.ID
	}
	id := create("Bal Polski 2027")
	id2 := create("Bal Polski 2027")
	p, err := db.LinkPageGetByID(id, orgID)
	if err != nil || p.URL != "bal-polski-2027" {
		t.Fatalf("page: %+v %v", p, err)
	}
	if p2, _ := db.LinkPageGetByID(id2, orgID); p2.URL != "bal-polski-2027-2" {
		t.Errorf("second url %q", p2.URL)
	}
	pid := itoa(id)
	section := func(sec string, values map[string]interface{}) *httptest.ResponseRecorder {
		t.Helper()
		b, _ := json.Marshal(map[string]interface{}{"section": sec, "values": values})
		return do("POST", "/api/pages/"+pid+"/section", string(b), owner)
	}
	buttons := func(bb ...LinkButton) map[string]interface{} {
		b, _ := json.Marshal(bb)
		return map[string]interface{}{"buttons": string(b)}
	}

	// adres zajęty przez drugą stronę
	if rec := section("basic", map[string]interface{}{"name": "Bal", "url": "bal-polski-2027-2", "language": "cs"}); rec.Code != http.StatusBadRequest {
		t.Errorf("taken url: %d", rec.Code)
	}
	if rec := section("basic", map[string]interface{}{"name": "Bal", "url": "bal", "language": "cs"}); rec.Code != 200 {
		t.Fatalf("basic: %d %s", rec.Code, rec.Body)
	}

	// złe przyciski
	for name, vals := range map[string]map[string]interface{}{
		"foreign event": buttons(LinkButton{Kind: LINKEVENT, ID: foreign}),
		"no target":     buttons(LinkButton{Kind: ""}),
		"js url":        buttons(LinkButton{Kind: LINKURL, URL: "javascript:alert(1)", Label: "X"}),
		"url no label":  buttons(LinkButton{Kind: LINKURL, URL: "https://zori.cz"}),
	} {
		if rec := section("buttons", vals); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
	if _, err := db.DB.Exec(`UPDATE events SET date=$1 WHERE id IN ($2, $3)`, 1893456000, ev, draft); err != nil {
		t.Fatal(err)
	}
	rec := section("buttons", buttons(
		LinkButton{Kind: LINKEVENT, ID: ev, Color: "red", Size: "l", NoDate: true},
		LinkButton{Kind: LINKFORM, ID: form, Label: "Zapisz się", Color: "#f00", Size: "xxl", NoDate: true, ImgOnly: true},
		LinkButton{Kind: LINKEVENT, ID: draft, Img: "../../etc/passwd"},
		LinkButton{Kind: LINKURL, URL: "www.zori.cz", Label: "Strona Zori", Img: "plakaty/zori.png;50%", ImgOnly: true},
	))
	if rec.Code != 200 {
		t.Fatalf("buttons: %d %s", rec.Code, rec.Body)
	}
	// wygląd: tylko znane kolory i wielkości, obrazek w katalogu organizacji,
	// "bez daty" tylko przy imprezie, "sam obrazek" tylko z obrazkiem
	p, _ = db.LinkPageGetByID(id, orgID)
	saved := ParseLinkButtons(p.Buttons)
	if b := saved[0]; b.Color != "red" || b.Size != "l" || !b.NoDate {
		t.Errorf("button 1 look: %+v", b)
	}
	if b := saved[1]; b.Color != "" || b.Size != "" || b.NoDate || b.ImgOnly {
		t.Errorf("button 2 look not cleaned: %+v", b)
	}
	if b := saved[2]; b.Img != "" {
		t.Errorf("button 3 image outside media accepted: %+v", b)
	}
	if b := saved[3]; b.Img != "plakaty/zori.png" || !b.ImgOnly {
		t.Errorf("button 4 image: %+v", b)
	}
	rec = section("look", map[string]interface{}{"theme": "noc", "banner": "../../etc/passwd", "text": "<p>Zapraszamy!</p>", "head_name": "Bal"})
	if rec.Code != 200 {
		t.Fatalf("look: %d %s", rec.Code, rec.Body)
	}
	p, _ = db.LinkPageGetByID(id, orgID)
	if p.Theme != "noc" || p.Banner != "etc/passwd" || p.Language != "cs" {
		t.Errorf("saved: %+v", p)
	}

	// strona gościa: w języku strony, bez szkicu, z poprawionym adresem
	r := httptest.NewRequest("GET", "/p/org/bal", nil)
	browserHeaders(r, "cs")
	w := httptest.NewRecorder()
	rtr.ServeHTTP(w, r)
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, `<html lang="cs"`) {
		t.Fatalf("guest: %d %s", w.Code, body)
	}
	for _, want := range []string{"/res/org?event-id=" + itoa(ev), "Bal Polski", "/form/org/zgloszenie", "Zapisz się", "https://www.zori.cz", `target="_blank"`, "Zapraszamy!", `data-theme="noc"`} {
		if !strings.Contains(body, want) {
			t.Errorf("guest page: missing %q", want)
		}
	}
	if strings.Contains(body, "Piknik") {
		t.Errorf("guest page shows draft event")
	}
	for _, want := range []string{`class="lpage-btn c-red sz-l"`, `class="lpage-btn with-img img-only"`,
		`src="/media/org/plakaty/zori.png" alt="Strona Zori"`} {
		if !strings.Contains(body, want) {
			t.Errorf("guest page look: missing %q", want)
		}
	}
	if strings.Contains(body, FormatDate("cs", 1893456000)) {
		t.Errorf("guest page shows date of a button with nodate")
	}
	if strings.Contains(body, "<b>Strona Zori</b>") {
		t.Errorf("image-only button shows its label")
	}
	// admin widzi szkic z dopiskiem
	r = httptest.NewRequest("GET", "/p/org/bal", nil)
	for _, c := range owner {
		r.AddCookie(c)
	}
	w = httptest.NewRecorder()
	rtr.ServeHTTP(w, r)
	if !strings.Contains(w.Body.String(), "Piknik") || !strings.Contains(w.Body.String(), T("cs", "lpage.draft_guest")) {
		t.Errorf("admin preview: no draft button")
	}
	if rec := do("GET", "/p/org/nie-ma", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("missing page: %d", rec.Code)
	}

	// panel, strona startowa, QR
	if rec := do("GET", "/admin/pages/"+pid, "", owner); rec.Code != 200 || !strings.Contains(rec.Body.String(), "/p/org/bal") {
		t.Errorf("admin page: %d", rec.Code)
	}
	if rec := do("GET", "/admin", "", owner); !strings.Contains(rec.Body.String(), `href="/admin/pages/`+pid+`"`) {
		t.Errorf("home: no link page row")
	}
	if rec := do("GET", "/api/pages/"+pid+"/qr.png", "", owner); rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("\x89PNG")) {
		t.Errorf("qr: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	rec = do("GET", "/api/pages/"+pid+"/qr.svg?download=1", "", owner)
	svg := rec.Body.String()
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/svg+xml" || !strings.Contains(svg, "<svg") ||
		!strings.Contains(svg, `<path fill="#000" d="M4 4h7`) || strings.Contains(svg, "<rect") ||
		!strings.Contains(rec.Header().Get("Content-Disposition"), `qr-org-bal.svg`) {
		t.Errorf("qr svg: %d %q %.200s", rec.Code, rec.Header(), svg)
	}
	if rec := do("GET", "/api/pages/"+pid+"/qr.svg", "", nil); rec.Code == 200 {
		t.Errorf("qr svg without login: %d", rec.Code)
	}

	// usunięta impreza znika z przycisków gościa
	if err := db.EventDelete(ev, orgID); err != nil {
		t.Fatal(err)
	}
	if rec := do("GET", "/p/org/bal", "", nil); strings.Contains(rec.Body.String(), "event-id="+itoa(ev)) {
		t.Errorf("deleted event still linked")
	}

	// menedżer bez zarządzania organizacją: ani strony, ani listy
	perms := `{"all":true,"org":false,"items":[]}`
	if rec := do("POST", "/api/managers", `{"email":"m@example.com","password":"haslo-menedzera","perms":`+perms+`}`, owner); rec.Code != 200 {
		t.Fatalf("create manager: %d %s", rec.Code, rec.Body)
	}
	mgr := login("m@example.com", "haslo-menedzera")
	for _, c := range []struct{ method, path, body string }{
		{"GET", "/admin/pages/" + pid, ""},
		{"POST", "/api/pages", `{"name":"X"}`},
		{"POST", "/api/pages/" + pid + "/section", `{"section":"basic","values":{"name":"X"}}`},
		{"DELETE", "/api/pages/" + pid, ""},
	} {
		if rec := do(c.method, c.path, c.body, mgr); rec.Code != http.StatusForbidden {
			t.Errorf("manager %s %s: %d", c.method, c.path, rec.Code)
		}
	}
	if rec := do("GET", "/admin", "", mgr); strings.Contains(rec.Body.String(), `href="/admin/pages/`) || strings.Contains(rec.Body.String(), `id="new-page"`) {
		t.Errorf("manager home lists link pages")
	}

	// cudza organizacja nie widzi strony
	h2, _ := hashPassword("other-pass")
	db.DB.Exec(`UPDATE users SET passwd=$1 WHERE id=$2`, h2, other)
	if rec := do("GET", "/admin/pages/"+pid, "", login("x@example.com", "other-pass")); rec.Code != http.StatusNotFound {
		t.Errorf("foreign admin page: %d", rec.Code)
	}

	if rec := do("DELETE", "/api/pages/"+pid, "", owner); rec.Code != 200 {
		t.Errorf("delete: %d", rec.Code)
	}
	if _, err := db.LinkPageGetByID(id, orgID); err == nil {
		t.Errorf("page not deleted")
	}
}

package main

// Uprawnienia menedżerów (access.go) i dopisywanie osób przez menedżera
// (form_fill.go).

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

func TestRouteNeed(t *testing.T) {
	for _, c := range []struct {
		method, path, need, kind string
		id                       int64
	}{
		{"GET", "/admin", needAny, "", 0},
		{"GET", "/admin/events/12", needResults, ACCEVENT, 12},
		{"POST", "/api/events/12/section", needFull, ACCEVENT, 12},
		{"DELETE", "/api/events/12", needFull, ACCEVENT, 12},
		{"POST", "/api/events/12/reservations/status", needResults, ACCEVENT, 12},
		{"POST", "/api/events/12/orders/3/items/4/status", needResults, ACCEVENT, 12},
		{"GET", "/admin/forms/7/answers", needResults, ACCFORM, 7},
		{"GET", "/admin/forms/7/fields", needFull, ACCFORM, 7},
		{"POST", "/api/forms/7/fill", needResults, ACCFORM, 7},
		{"POST", "/api/forms/7/section", needFull, ACCFORM, 7},
		{"POST", "/api/forms", needOrg, "", 0},
		{"GET", "/admin/catalogs", needOrg, "", 0},
		{"GET", "/admin/events/12/rooms/3", needOrg, "", 0},
		{"POST", "/api/eventcopy", needOrg, "", 0},
		{"GET", "/admin/something-new", needOrg, "", 0},
	} {
		r := httptest.NewRequest(c.method, c.path, nil)
		need, kind, id := routeNeed(r)
		if need != c.need || kind != c.kind || id != c.id {
			t.Errorf("%s %s: %s %s %d, want %s %s %d", c.method, c.path, need, kind, id, c.need, c.kind, c.id)
		}
	}
}

// fillForm - formularz jak "funkcje na bal": dane rodzica (ukryte przed
// menedżerem), klasa i funkcje zależne od klasy.
const fillForm = `[
 {"type":"text","name":"name-REQUIRED","label":"Imię","required":true},
 {"type":"text","name":"surname-REQUIRED","label":"Nazwisko","required":true},
 {"type":"text","subtype":"email","name":"email-REQUIRED","label":"E-mail","required":true},
 {"type":"text","name":"telefon","label":"Telefon","required":true,"mgr_hidden":true},
 {"type":"select","name":"klasa","label":"Klasa","required":true,"values":[{"label":"1","value":"1"},{"label":"2","value":"2"}]},
 {"type":"checkbox-group","name":"k1_sala","label":"Sala 1","mgr_people":true,"panel_goal":2,"show_if":{"field":"klasa","values":["1"]},"values":[{"label":"Zgłaszam się","value":"Tak"}]},
 {"type":"checkbox-group","name":"k2_sala","label":"Sala 2","mgr_people":true,"show_if":{"field":"klasa","values":["2"]},"values":[{"label":"Zgłaszam się","value":"Tak"}]},
 {"type":"textarea","name":"uwagi","label":"Uwagi"}
]`

func TestManagerPermsAndFill(t *testing.T) {
	db := newTestDB(t)
	h, _ := hashPassword("owner-pass")
	orgID := must(t)(db.UserAdd(&User{Email: "org@example.com", URL: "org", Passwd: h}))
	ev1 := addEvent(t, db, orgID, "Bal")
	ev2 := addEvent(t, db, orgID, "Piknik")
	f1 := must(t)(db.FormTemplateAdd(&FormTemplate{Name: "Funkcje", URL: "funkcje", Content: ToNS(fillForm), UserID: orgID, CreatedDate: 1}))
	f2 := must(t)(db.FormTemplateAdd(&FormTemplate{Name: "Tajny formularz", URL: "tajny", Content: ToNS(`[]`), UserID: orgID, CreatedDate: 1}))
	for _, f := range []string{"name-REQUIRED", "surname-REQUIRED", "email-REQUIRED", "telefon", "klasa", "k1_sala", "k2_sala", "uwagi"} {
		must(t)(db.FormFieldAdd(&FormField{Name: f, Display: f, Type: "text", FormTemplateID: f1}))
	}

	cs := sessions.NewCookieStore([]byte("0123456789abcdef0123456789abcdef"))
	rtr := mux.NewRouter()
	rtr.HandleFunc("/admin", AdminStartPage(db, cs))
	rtr.HandleFunc("/admin/catalogs", CatalogsPage(db, cs))
	rtr.HandleFunc("/admin/events/{id:[0-9]+}", EventPage(db, cs))
	rtr.HandleFunc("/admin/forms/{id:[0-9]+}", FormPage(db, cs))
	rtr.HandleFunc("/admin/forms/{id:[0-9]+}/fill", FormFillPage(db, cs))
	rtr.HandleFunc("/api/forms/{id:[0-9]+}/fill", FormFillAPI(db, MailConfig{}, cs)).Methods("POST")
	rtr.HandleFunc("/api/forms/{id:[0-9]+}/section", FormSectionSave(db, cs)).Methods("POST")
	rtr.HandleFunc("/api/forms", FormCreateAPI(db, cs)).Methods("POST")
	rtr.HandleFunc("/api/managers", ManagerCreateAPI(db, cs)).Methods("POST")
	rtr.HandleFunc("/api/managers/{id:[0-9]+}", ManagerSaveAPI(db, cs)).Methods("POST")
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

	// cudza pozycja w uprawnieniach = błąd
	other := addUser(t, db, "x@example.com", "x")
	foreign := addEvent(t, db, other, "Cudza")
	bad := `{"email":"m@example.com","password":"haslo-menedzera","perms":{"all":false,"org":false,"items":[{"kind":"event","id":` + itoa(foreign) + `,"level":"full"}]}}`
	if rec := do("POST", "/api/managers", bad, owner); rec.Code != http.StatusBadRequest {
		t.Fatalf("foreign item: %d %s", rec.Code, rec.Body)
	}

	perms := `{"all":false,"org":false,"items":[{"kind":"form","id":` + itoa(f1) + `,"level":"results"},{"kind":"event","id":` + itoa(ev1) + `,"level":"full"}]}`
	if rec := do("POST", "/api/managers", `{"email":"m@example.com","password":"haslo-menedzera","perms":`+perms+`}`, owner); rec.Code != 200 {
		t.Fatalf("create manager: %d %s", rec.Code, rec.Body)
	}
	if rec := do("POST", "/api/managers", `{"email":"m2@example.com","password":"haslo-menedzera","perms":`+perms+`}`, owner); rec.Code != 200 {
		t.Fatalf("create manager 2: %d %s", rec.Code, rec.Body)
	}
	mgr := login("m@example.com", "haslo-menedzera")
	mgr2 := login("m2@example.com", "haslo-menedzera")

	// --- uprawnienia ---
	for _, c := range []struct {
		method, path, body string
		code               int
	}{
		{"GET", "/admin/forms/" + itoa(f1), "", 200},
		{"GET", "/admin/forms/" + itoa(f2), "", http.StatusForbidden},
		{"POST", "/api/forms/" + itoa(f1) + "/section", `{"section":"basic","values":{"name":"X"}}`, http.StatusForbidden},
		{"GET", "/admin/events/" + itoa(ev1), "", 200},
		{"GET", "/admin/events/" + itoa(ev2), "", http.StatusForbidden},
		{"GET", "/admin/catalogs", "", http.StatusForbidden},
		{"POST", "/api/forms", `{"name":"Nowy"}`, http.StatusForbidden},
	} {
		if rec := do(c.method, c.path, c.body, mgr); rec.Code != c.code {
			t.Errorf("%s %s: %d, want %d", c.method, c.path, rec.Code, c.code)
		}
	}
	if body := do("GET", "/admin/forms/"+itoa(f1), "", mgr).Body.String(); !strings.Contains(body, "data-ro") || !strings.Contains(body, "/admin/forms/"+itoa(f1)+"/fill") {
		t.Errorf("form page for 'results' manager: no data-ro or fill link")
	}
	if body := do("GET", "/admin/events/"+itoa(ev1), "", mgr).Body.String(); strings.Contains(body, "data-ro") {
		t.Errorf("event page for 'full' manager is read-only")
	}
	home := do("GET", "/admin", "", mgr).Body.String()
	if !strings.Contains(home, "Funkcje") || strings.Contains(home, "Tajny formularz") || strings.Contains(home, "Piknik") {
		t.Errorf("home of restricted manager shows wrong items")
	}
	if strings.Contains(home, `href="/admin/catalogs"`) {
		t.Errorf("home of manager without org_manage links to catalogs")
	}
	if strings.Contains(home, "simple-go") {
		t.Errorf("home without simple view shows the big button")
	}
	// uproszczony widok: wielki przycisk do dopisywania osób nad listami
	simplePerms := strings.Replace(perms, `"org":false`, `"org":false,"simple":true`, 1)
	if rec := do("POST", "/api/managers", `{"email":"m3@example.com","password":"haslo-menedzera","perms":`+simplePerms+`}`, owner); rec.Code != 200 {
		t.Fatalf("create simple manager: %d %s", rec.Code, rec.Body)
	}
	home3 := do("GET", "/admin", "", login("m3@example.com", "haslo-menedzera")).Body.String()
	big := strings.Index(home3, `class="btn strong huge" href="/admin/forms/`+itoa(f1)+`/fill"`)
	if big < 0 || big > strings.Index(home3, "Funkcje") {
		t.Errorf("simple view: no big fill button above the lists")
	}
	// konto organizacji widzi wszystko
	if rec := do("GET", "/admin/forms/"+itoa(f2), "", owner); rec.Code != 200 {
		t.Errorf("owner: form 2: %d", rec.Code)
	}

	// --- strona dopisywania: bez pól osoby, z polami funkcji ---
	page := do("GET", "/admin/forms/"+itoa(f1)+"/fill", "", mgr)
	if page.Code != 200 {
		t.Fatalf("fill page: %d %s", page.Code, page.Body)
	}
	pb := page.Body.String()
	for _, hidden := range []string{`data-name="telefon"`, `data-name="name-REQUIRED"`, `data-name="email-REQUIRED"`} {
		if strings.Contains(pb, hidden) {
			t.Errorf("fill page shows %s", hidden)
		}
	}
	if !strings.Contains(pb, `class="ff ff-people w12" data-name="k1_sala"`) || !strings.Contains(pb, `data-name="klasa"`) {
		t.Errorf("fill page: no people field or no klasa")
	}

	fill := func(c []*http.Cookie, body string) (fillResp, *httptest.ResponseRecorder) {
		t.Helper()
		rec := do("POST", "/api/forms/"+itoa(f1)+"/fill", body, c)
		var r fillResp
		json.Unmarshal(rec.Body.Bytes(), &r)
		return r, rec
	}
	klasa1 := `[{"name":"klasa","userData":["1"]},{"name":"uwagi","userData":["od menedżera"]},{"name":"telefon","userData":["ukryte"]}]`

	// bez klasy: wymagane pole wspólne
	if _, rec := fill(mgr, `{"data":[],"add":[{"field":"k1_sala","name":"Jan","surname":"Nowak","email":"jan@example.com"}]}`); rec.Code != http.StatusBadRequest {
		t.Errorf("no klasa: %d", rec.Code)
	}
	// funkcja niewidoczna przy klasie 1
	if _, rec := fill(mgr, `{"data":`+klasa1+`,"add":[{"field":"k2_sala","name":"Jan","surname":"Nowak","email":"jan@example.com"}]}`); rec.Code != http.StatusBadRequest {
		t.Errorf("hidden field: %d", rec.Code)
	}
	// zły e-mail
	if _, rec := fill(mgr, `{"data":`+klasa1+`,"add":[{"field":"k1_sala","name":"Jan","surname":"Nowak","email":"jan"}]}`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad email: %d", rec.Code)
	}

	// gość zgłosił się sam (bez funkcji)
	guest := must(t)(db.FormAdd(&Form{Name: ToNS("Ewa"), Surname: ToNS("Gość"), Email: ToNS("ewa@example.com"), CreatedDate: 1, UserID: orgID, FormTemplateID: f1}))

	r, rec := fill(mgr, `{"data":`+klasa1+`,"add":[
		{"field":"k1_sala","name":" Jan ","surname":"Nowak","email":"JAN@example.com"},
		{"field":"k1_sala","name":"Ewa","surname":"Inna","email":"ewa@example.com"}]}`)
	if rec.Code != 200 || r.Added != 2 {
		t.Fatalf("fill: %d %s", rec.Code, rec.Body)
	}
	if len(r.People["k1_sala"]) != 2 {
		t.Fatalf("k1_sala people: %+v", r.People["k1_sala"])
	}
	// Ewa dostała funkcję do swojego zgłoszenia, Jan - nowe zgłoszenie z klasą
	if v, _ := db.FormAnswerGetByFieldName(guest, f1, "k1_sala"); v != "Tak" {
		t.Errorf("guest form k1_sala = %q", v)
	}
	jans, _ := db.FormGetAllByEmail("jan@example.com", f1, orgID)
	if len(jans) != 1 || jans[0].Name.String != "Jan" || !jans[0].AddedBy.Valid {
		t.Fatalf("Jan's form: %+v", jans)
	}
	jan := jans[0].ID
	for field, want := range map[string]string{"klasa": "1", "k1_sala": "Tak", "email-REQUIRED": "jan@example.com", "uwagi": "od menedżera", "telefon": ""} {
		if v, _ := db.FormAnswerGetByFieldName(jan, f1, field); v != want {
			t.Errorf("Jan %s = %q, want %q", field, v, want)
		}
	}
	// podpowiedzi menedżera
	if cc, _ := db.ContactGetAll(orgID, mustManagerID(t, db, "m@example.com")); len(cc) != 2 {
		t.Errorf("contacts: %+v", cc)
	}
	if len(r.Contacts) != 2 {
		t.Errorf("contacts in response: %d", len(r.Contacts))
	}
	// drugi raz ta sama osoba przy tej samej funkcji - bez zmian
	if r, _ := fill(mgr, `{"data":`+klasa1+`,"add":[{"field":"k1_sala","name":"Jan","surname":"Nowak","email":"jan@example.com"}]}`); r.Added != 0 {
		t.Errorf("duplicate add counted: %d", r.Added)
	}

	// drugi menedżer nie może usunąć osób dopisanych przez pierwszego
	if _, rec := fill(mgr2, `{"data":[],"remove":[{"field":"k1_sala","form":`+itoa(jan)+`}]}`); rec.Code != http.StatusForbidden {
		t.Errorf("other manager remove: %d", rec.Code)
	}
	r2, _ := fill(mgr2, `{"data":[],"add":[]}`)
	for _, p := range r2.People["k1_sala"] {
		if p.Mine || p.By != "other" {
			t.Errorf("manager 2 sees %+v", p)
		}
	}

	// usunięcie: zgłoszenie Jana znika, Ewa zostaje bez funkcji
	r, rec = fill(mgr, `{"data":[],"remove":[{"field":"k1_sala","form":`+itoa(jan)+`},{"field":"k1_sala","form":`+itoa(guest)+`}]}`)
	if rec.Code != 200 || r.Removed != 2 || len(r.People["k1_sala"]) != 0 {
		t.Fatalf("remove: %d %s", rec.Code, rec.Body)
	}
	if _, err := db.FormGet(jan, f1); err == nil {
		t.Errorf("Jan's form not deleted")
	}
	if _, err := db.FormGet(guest, f1); err != nil {
		t.Errorf("guest form deleted: %v", err)
	}
	// samodzielnego zgłoszenia gościa menedżer nie zdejmie
	db.FormAnswerSet(&FormAnswer{Value: ToNS("Tak"), FormFieldID: fieldID(t, db, f1, "k1_sala"), FormID: guest})
	if _, rec := fill(mgr, `{"data":[],"remove":[{"field":"k1_sala","form":`+itoa(guest)+`}]}`); rec.Code != http.StatusForbidden {
		t.Errorf("remove guest's own answer: %d", rec.Code)
	}

	// menedżer z obsługą formularza nie może go wypełniać za cudzy formularz
	if _, rec := fill(mgr, `{"data":[]}`); rec.Code != 200 {
		t.Errorf("empty save: %d", rec.Code)
	}
	if rec := do("POST", "/api/forms/"+itoa(f2)+"/fill", `{"data":[]}`, mgr); rec.Code != http.StatusForbidden {
		t.Errorf("fill of forbidden form: %d", rec.Code)
	}
}

func mustManagerID(t *testing.T, db *DB, email string) int64 {
	t.Helper()
	a, err := db.ManagerGetByEmail(email)
	if err != nil {
		t.Fatal(err)
	}
	return a.ID
}

func fieldID(t *testing.T, db *DB, tmplID int64, name string) int64 {
	t.Helper()
	id, err := db.FormFieldGetIDByName(name, tmplID)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

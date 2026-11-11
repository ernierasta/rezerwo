package main

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

// Zapamiętane osoby: edycja poprawia zgłoszenia dopisującego, nie rusza
// zgłoszeń gości, menedżer zmienia tylko swoje, usuwanie - tylko z listy.
func TestContactsEdit(t *testing.T) {
	db := newTestDB(t)
	h, _ := hashPassword("owner-pass")
	orgID := must(t)(db.UserAdd(&User{Email: "org@example.com", URL: "org", Passwd: h}))
	f1 := must(t)(db.FormTemplateAdd(&FormTemplate{Name: "Funkcje", URL: "funkcje", Content: ToNS(fillForm), UserID: orgID, CreatedDate: 1}))
	f2 := must(t)(db.FormTemplateAdd(&FormTemplate{Name: "Drugi", URL: "drugi", Content: ToNS(fillForm), UserID: orgID, CreatedDate: 1}))
	for _, ft := range []int64{f1, f2} {
		for _, f := range []string{"name-REQUIRED", "surname-REQUIRED", "email-REQUIRED", "telefon", "klasa", "k1_sala", "k2_sala", "uwagi"} {
			must(t)(db.FormFieldAdd(&FormField{Name: f, Display: f, Type: "text", FormTemplateID: ft}))
		}
	}

	cs := sessions.NewCookieStore([]byte("0123456789abcdef0123456789abcdef"))
	rtr := mux.NewRouter()
	rtr.HandleFunc("/admin", AdminStartPage(db, cs))
	rtr.HandleFunc("/admin/catalogs", CatalogsPage(db, cs))
	rtr.HandleFunc("/api/forms/{id:[0-9]+}/fill", FormFillAPI(db, MailConfig{}, cs)).Methods("POST")
	rtr.HandleFunc("/api/managers", ManagerCreateAPI(db, cs)).Methods("POST")
	rtr.HandleFunc("/api/contacts/{id:[0-9]+}", ContactSaveAPI(db, cs)).Methods("POST")
	rtr.HandleFunc("/api/contacts/{id:[0-9]+}", ContactDelAPI(db, cs)).Methods("DELETE")
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
	for _, m := range []string{"m@example.com", "m2@example.com"} {
		if rec := do("POST", "/api/managers", `{"email":"`+m+`","password":"haslo-menedzera","perms":{"all":true,"org":false,"items":[]}}`, owner); rec.Code != 200 {
			t.Fatalf("create manager: %d %s", rec.Code, rec.Body)
		}
	}
	mgr := login("m@example.com", "haslo-menedzera")
	mgr2 := login("m2@example.com", "haslo-menedzera")
	mgrID := mustManagerID(t, db, "m@example.com")

	klasa1 := `[{"name":"klasa","userData":["1"]}]`
	fill := func(c []*http.Cookie, form int64, adds string) {
		t.Helper()
		if rec := do("POST", "/api/forms/"+itoa(form)+"/fill", `{"data":`+klasa1+`,"add":[`+adds+`]}`, c); rec.Code != 200 {
			t.Fatalf("fill: %d %s", rec.Code, rec.Body)
		}
	}
	// Jan dopisany w dwóch formularzach; w drugim jest już gość z nowym
	// e-mailem Jana, więc tam zgłoszenia nie da się poprawić
	fill(mgr, f1, `{"field":"k1_sala","name":"Jn","surname":"Nowak","email":"jan@exmple.com"},{"field":"k1_sala","name":"Ola","surname":"Lis","email":"ola@example.com"}`)
	fill(mgr, f2, `{"field":"k1_sala","name":"Jn","surname":"Nowak","email":"jan@exmple.com"}`)
	must(t)(db.FormAdd(&Form{Name: ToNS("Jan"), Surname: ToNS("Gość"), Email: ToNS("jan@example.com"), CreatedDate: 1, UserID: orgID, FormTemplateID: f2}))
	// gość z e-mailem Oli zgłosił się sam do f2 - jego zgłoszenia nie ruszamy
	guestOla := must(t)(db.FormAdd(&Form{Name: ToNS("Ola"), Surname: ToNS("Lis"), Email: ToNS("ola@example.com"), CreatedDate: 1, UserID: orgID, FormTemplateID: f2}))
	fill(mgr2, f1, `{"field":"k1_sala","name":"Piotr","surname":"Kot","email":"piotr@example.com"}`)

	cc, _ := db.ContactGetAll(orgID, mgrID)
	ids := map[string]int64{}
	for _, c := range cc {
		ids[c.Email] = c.ID
	}
	cc2, _ := db.ContactGetAll(orgID, mustManagerID(t, db, "m2@example.com"))
	if len(cc) != 2 || len(cc2) != 1 {
		t.Fatalf("contacts: %+v / %+v", cc, cc2)
	}
	piotr := cc2[0].ID

	// strona startowa menedżera: tylko jego osoby; katalogi organizacji - wszystkie
	home := do("GET", "/admin", "", mgr).Body.String()
	if !strings.Contains(home, `id="contacts"`) || !strings.Contains(home, "ola@example.com") || strings.Contains(home, "piotr@example.com") {
		t.Errorf("manager home: contacts section wrong")
	}
	if strings.Contains(do("GET", "/admin", "", owner).Body.String(), `id="contacts"`) {
		t.Errorf("owner home shows contacts")
	}
	cat := do("GET", "/admin/catalogs", "", owner).Body.String()
	if !strings.Contains(cat, "piotr@example.com") || !strings.Contains(cat, "m2@example.com") || !strings.Contains(cat, "ola@example.com") {
		t.Errorf("catalogs: all contacts with author expected")
	}

	save := func(c []*http.Cookie, id int64, body string) (contactResp, *httptest.ResponseRecorder) {
		t.Helper()
		rec := do("POST", "/api/contacts/"+itoa(id), body, c)
		var r contactResp
		json.Unmarshal(rec.Body.Bytes(), &r)
		return r, rec
	}

	// cudza osoba: menedżer 2 nie zmieni ani nie usunie osoby menedżera 1
	if _, rec := save(mgr2, ids["jan@exmple.com"], `{"name":"X","surname":"Y","email":"x@example.com"}`); rec.Code != http.StatusNotFound {
		t.Errorf("other manager edit: %d", rec.Code)
	}
	if rec := do("DELETE", "/api/contacts/"+itoa(ids["jan@exmple.com"]), "", mgr2); rec.Code != http.StatusNotFound {
		t.Errorf("other manager delete: %d", rec.Code)
	}
	// błędne dane
	if _, rec := save(mgr, ids["jan@exmple.com"], `{"name":"Jan","surname":"Nowak","email":"zly"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad email: %d", rec.Code)
	}
	// e-mail innej zapamiętanej osoby
	if _, rec := save(mgr, ids["jan@exmple.com"], `{"name":"Jan","surname":"Nowak","email":"ola@example.com"}`); rec.Code != http.StatusConflict {
		t.Errorf("duplicate email: %d", rec.Code)
	}

	// poprawka: f1 poprawione, f2 pominięte (gość z tym e-mailem)
	r, rec := save(mgr, ids["jan@exmple.com"], `{"name":" Jan ","surname":"Nowak","email":"JAN@example.com"}`)
	if rec.Code != 200 || r.Forms != 1 || len(r.Skipped) != 1 || r.Skipped[0] != "Drugi" {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	jans, _ := db.FormGetAllByEmail("jan@example.com", f1, orgID)
	if len(jans) != 1 || jans[0].Name.String != "Jan" {
		t.Fatalf("f1 form not corrected: %+v", jans)
	}
	for field, want := range map[string]string{"name-REQUIRED": "Jan", "email-REQUIRED": "jan@example.com", "k1_sala": "Tak"} {
		if v, _ := db.FormAnswerGetByFieldName(jans[0].ID, f1, field); v != want {
			t.Errorf("f1 %s = %q, want %q", field, v, want)
		}
	}
	if old, _ := db.FormGetAllByEmail("jan@exmple.com", f2, orgID); len(old) != 1 {
		t.Errorf("f2 form should stay: %+v", old)
	}
	if c, _ := db.ContactGet(ids["jan@exmple.com"], orgID); c.Email != "jan@example.com" || c.Name != "Jan" {
		t.Errorf("contact not saved: %+v", c)
	}

	// konto organizacji poprawia osobę menedżera; zgłoszenie gościa bez zmian
	if r, rec := save(owner, ids["ola@example.com"], `{"name":"Aleksandra","surname":"Lis","email":"ola@example.com"}`); rec.Code != 200 || r.Forms != 1 {
		t.Fatalf("owner save: %d %s", rec.Code, rec.Body)
	}
	if g, _ := db.FormGet(guestOla, f2); g.Name.String != "Ola" {
		t.Errorf("guest form changed: %+v", g)
	}

	// usunięcie: znika z listy, zgłoszenie zostaje
	if rec := do("DELETE", "/api/contacts/"+itoa(piotr), "", owner); rec.Code != 200 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if _, err := db.ContactGet(piotr, orgID); err == nil {
		t.Errorf("contact not deleted")
	}
	if pp, _ := db.FormGetAllByEmail("piotr@example.com", f1, orgID); len(pp) != 1 {
		t.Errorf("form deleted with contact")
	}
}

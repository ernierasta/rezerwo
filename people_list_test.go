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

// Lista zapisanych: pod opcjami pola z people_list są imiona i nazwiska
// (gości i osób dopisanych przez menedżera), nigdy e-maile; pole bez
// ustawienia nie zdradza nikogo; /live oddaje listy na bieżąco.
func TestGuestPeopleList(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	db := newTestDB(t)
	userID := addUser(t, db, "org@example.com", "org")
	content := `[
	 {"type":"text","name":"name-REQUIRED","label":"Imię"},
	 {"type":"text","name":"surname-REQUIRED","label":"Nazwisko"},
	 {"type":"text","subtype":"email","name":"email-REQUIRED","label":"E-mail"},
	 {"type":"checkbox-group","name":"funkcje","label":"Pomoc","people_list":true,"people_slots":true,"panel_goal":3,"values":[{"label":"Szatnia","value":"szatnia"},{"label":"Bufet","value":"bufet"},{"label":"Kuchnia","value":"kuchnia"}]},
	 {"type":"checkbox-group","name":"inne","label":"Inne","people_list":true,"values":[{"label":"Tak","value":"tak"}]},
	 {"type":"checkbox-group","name":"zgoda","label":"Zgoda","values":[{"label":"Tak","value":"tak"}]}
	]`
	templID := must(t)(db.FormTemplateAdd(&FormTemplate{Name: "Bal", URL: "bal", Content: ToNS(content), UserID: userID, CreatedDate: 1}))
	fieldIDs := map[string]int64{}
	for _, n := range []string{"name-REQUIRED", "surname-REQUIRED", "email-REQUIRED", "funkcje", "inne", "zgoda"} {
		fieldIDs[n] = must(t)(db.FormFieldAdd(&FormField{Name: n, Display: n, Type: "text", FormTemplateID: templID}))
	}
	if _, err := db.DB.Exec(`UPDATE formtemplates SET language='en' WHERE id=$1`, templID); err != nil {
		t.Fatal(err)
	}
	cs := sessions.NewCookieStore([]byte("0123456789abcdef0123456789abcdef"))
	router := mux.NewRouter()
	router.HandleFunc("/form/{userurl}/{formurl}/send", GuestFormSendAPI(db, MailConfig{}))
	router.HandleFunc("/form/{userurl}/{formurl}/live", GuestFormLiveAPI(db, cs))
	router.HandleFunc("/form/{userurl}/{formurl}", GuestFormPage(db, cs))

	send := func(name, surname, email string, funkcje ...string) {
		t.Helper()
		body, _ := json.Marshal(gformSendMsg{Data: []FormAnswersJson{
			{Name: "name-REQUIRED", UserData: []string{name}}, {Name: "surname-REQUIRED", UserData: []string{surname}},
			{Name: "email-REQUIRED", UserData: []string{email}}, {Name: "funkcje", UserData: funkcje},
			{Name: "zgoda", UserData: []string{"tak"}},
		}})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("POST", "/form/org/bal/send", bytes.NewReader(body)))
		if w.Code != http.StatusOK {
			t.Fatalf("send: %d %s", w.Code, w.Body)
		}
	}
	send("Jan", "Nowak", "jan@example.com", "szatnia", "bufet")
	send("Anna", "Kowalska", "anna@example.com", "szatnia")

	// osoba dopisana przez menedżera
	fid := must(t)(db.FormAdd(&Form{Name: ToNS("Ewa"), Surname: ToNS("Zając"), Email: ToNS("ewa@example.com"), CreatedDate: 1,
		UserID: userID, FormTemplateID: templID, AddedBy: ToNI(7)}))
	if err := db.FormAnswerSet(&FormAnswer{Value: ToNS("bufet"), FormFieldID: fieldIDs["funkcje"], FormID: fid, AddedBy: ToNI(7)}); err != nil {
		t.Fatal(err)
	}

	w := liveGet(t, router, "/form/org/bal/live", "")
	if w.Code != http.StatusOK {
		t.Fatalf("live: %d %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "@example.com") {
		t.Errorf("live leaks e-mails: %s", w.Body)
	}
	var r struct {
		Fields map[string]liveField `json:"fields"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	p := r.Fields["funkcje"].People
	if got := strings.Join(p["szatnia"], "|"); got != "Anna Kowalska|Jan Nowak" {
		t.Errorf("szatnia = %q", got)
	}
	if got := strings.Join(p["bufet"], "|"); got != "Jan Nowak|Ewa Zając" {
		t.Errorf("bufet = %q", got)
	}
	if _, ok := r.Fields["zgoda"]; ok {
		t.Error("field without people_list must not list people")
	}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/form/org/bal", nil))
	page := w.Body.String()
	if !strings.Contains(page, `data-live="/form/org/bal/live"`) {
		t.Error("form page with a people list does not poll its live state")
	}
	if !strings.Contains(page, "ff-people-list") || !strings.Contains(page, "<li>Ewa Zając</li>") {
		t.Error("people list not rendered on the guest page")
	}
	// cel 3: szatnia 2 osoby + 1 puste, bufet 2 + 1, kuchnia 3 puste
	if n := strings.Count(page, `class="fpl-slot"`); n != 5 {
		t.Errorf("empty places = %d, want 5", n)
	}
	// pusta lista bez pustych miejsc się nie pokazuje
	if !strings.Contains(page, `class="fpl empty" data-v="tak"`) {
		t.Error("empty list without places must be hidden")
	}
	if strings.Contains(page, "jan@example.com") {
		t.Error("guest page leaks e-mails")
	}
}

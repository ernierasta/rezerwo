package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

func liveGet(t *testing.T, router *mux.Router, url, etag string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("GET", url, nil)
	if etag != "" {
		r.Header.Set("If-None-Match", etag)
	}
	browserHeaders(r, "pl")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

// Stan miejsc na żywo: zmiana rezerwacji zmienia odpowiedź, ten sam stan
// daje 304, szkic i cudza impreza nic nie zdradzają.
func TestGuestRoomLive(t *testing.T) {
	f := newGuestFixture(t)
	f.router.HandleFunc("/res/{user}/live", GuestRoomLiveAPI(f.db, sessions.NewCookieStore([]byte("0123456789abcdef0123456789abcdef"))))
	f.setLang(t, "cs")
	url := fmt.Sprintf("/res/org/live?event-id=%d", f.eventID)

	type resp struct {
		Rooms []liveRoom `json:"rooms"`
	}
	read := func(w *httptest.ResponseRecorder) resp {
		t.Helper()
		if w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body)
		}
		var r resp
		if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		return r
	}

	w := liveGet(t, f.router, url, "")
	r := read(w)
	if len(r.Rooms) != 1 || r.Rooms[0].Free != 3 || r.Rooms[0].Total != 3 || len(r.Rooms[0].Seats) != 3 {
		t.Fatalf("rooms = %+v", r.Rooms)
	}
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag")
	}
	if w := liveGet(t, f.router, url, etag); w.Code != http.StatusNotModified || w.Body.Len() != 0 {
		t.Errorf("same state: status %d, body %q; want 304", w.Code, w.Body)
	}

	cust := must(t)(f.db.CustomerAdd(&Customer{Email: "guest@example.com"}))
	must(t)(f.db.ReservationAdd(&Reservation{Status: "marked", FurnitureID: chair(t, f.db, f.roomID, 2), EventID: f.eventID, CustomerID: cust}))
	w = liveGet(t, f.router, url, etag)
	r = read(w)
	if r.Rooms[0].Free != 2 {
		t.Errorf("free = %d, want 2", r.Rooms[0].Free)
	}
	for _, s := range r.Rooms[0].Seats {
		want := "free"
		if s.N == 2 {
			want = "taken"
			if s.St != "marked" {
				t.Errorf("seat 2 state %q, want marked", s.St)
			}
			// dymek w języku imprezy
			if !strings.Contains(s.T, T("cs", "guest.taken")) {
				t.Errorf("seat 2 title %q not in Czech", s.T)
			}
		}
		if s.S != want {
			t.Errorf("seat %d: %q, want %q", s.N, s.S, want)
		}
	}

	if _, err := f.db.DB.Exec(`UPDATE events SET status='draft' WHERE id=$1`, f.eventID); err != nil {
		t.Fatal(err)
	}
	if w := liveGet(t, f.router, url, ""); w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "seats") {
		t.Errorf("draft event: status %d, body %s", w.Code, w.Body)
	}
	if _, err := f.db.DB.Exec(`UPDATE events SET status='' WHERE id=$1`, f.eventID); err != nil {
		t.Fatal(err)
	}
	addUser(t, f.db, "other@example.com", "other")
	if w := liveGet(t, f.router, fmt.Sprintf("/res/other/live?event-id=%d", f.eventID), ""); w.Code != http.StatusNotFound {
		t.Errorf("event of another organization: status %d", w.Code)
	}

	page := f.do(t, httptest.NewRequest("GET", fmt.Sprintf("/res/org?event-id=%d", f.eventID), nil), "cs")
	if !strings.Contains(page, `data-live="/res/org/live?event-id=`) {
		t.Error("reservation page does not point to its live state")
	}
}

// Formularz na żywo: panel i liczby przy opcjach rosną z każdym
// zgłoszeniem, zamknięty formularz nic nie oddaje.
func TestGuestFormLive(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	db := newTestDB(t)
	userID := addUser(t, db, "org@example.com", "org")
	content := `[
	 {"type":"text","subtype":"email","name":"email-REQUIRED","label":"E-mail"},
	 {"type":"radio-group","name":"szatnia","label":"Szatnia","panel_place":"field","panel_goal":1,"values":[{"label":"Tak","value":"tak"},{"label":"Nie","value":"nie"}]},
	 {"type":"select","name":"klasa","label":"Klasa","panel_place":"field","values":[{"label":"1A","value":"1a"},{"label":"1B","value":"1b"}]},
	 {"type":"number","name":"ile","label":"Ile osób","panel_place":"field"},
	 {"type":"number","name":"ciasta","label":"Ciasta"}
	]`
	templID := must(t)(db.FormTemplateAdd(&FormTemplate{Name: "Bal", URL: "bal", Content: ToNS(content), UserID: userID, CreatedDate: 1}))
	for _, n := range []string{"email-REQUIRED", "szatnia", "klasa", "ile", "ciasta"} {
		must(t)(db.FormFieldAdd(&FormField{Name: n, Display: n, Type: "text", FormTemplateID: templID}))
	}
	if _, err := db.DB.Exec(`UPDATE formtemplates SET infopanel_mode='auto', language='en' WHERE id=$1`, templID); err != nil {
		t.Fatal(err)
	}
	cs := sessions.NewCookieStore([]byte("0123456789abcdef0123456789abcdef"))
	router := mux.NewRouter()
	router.HandleFunc("/form/{userurl}/{formurl}/send", GuestFormSendAPI(db, MailConfig{}))
	router.HandleFunc("/form/{userurl}/{formurl}/live", GuestFormLiveAPI(db, cs))
	router.HandleFunc("/form/{userurl}/{formurl}", GuestFormPage(db, cs))

	send := func(email, szatnia, klasa, ile string) {
		t.Helper()
		body, _ := json.Marshal(gformSendMsg{Data: []FormAnswersJson{
			{Name: "email-REQUIRED", UserData: []string{email}}, {Name: "szatnia", UserData: []string{szatnia}},
			{Name: "klasa", UserData: []string{klasa}}, {Name: "ile", UserData: []string{ile}},
			{Name: "ciasta", UserData: []string{"1"}},
		}})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("POST", "/form/org/bal/send", bytes.NewReader(body)))
		if w.Code != http.StatusOK {
			t.Fatalf("send: %d %s", w.Code, w.Body)
		}
	}
	type resp struct {
		Info   string               `json:"info"`
		Fields map[string]liveField `json:"fields"`
	}
	get := func() resp {
		t.Helper()
		w := liveGet(t, router, "/form/org/bal/live", "")
		if w.Code != http.StatusOK {
			t.Fatalf("live: %d %s", w.Code, w.Body)
		}
		var r resp
		if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		return r
	}

	send("a@example.com", "tak", "1a", "2")
	send("b@example.com", "tak", "1a", "3")
	r := get()
	if got := r.Fields["szatnia"].Opts["|tak"]; got.T != "2 / 1" || !got.D {
		t.Errorf("szatnia tak = %+v, want 2 / 1 done (goal never blocks)", got)
	}
	if got := r.Fields["szatnia"].Opts["|nie"]; got.T != "0 / 1" || got.D {
		t.Errorf("szatnia nie = %+v", got)
	}
	if got := r.Fields["klasa"].Opts["|1a"].T; got != "2" {
		t.Errorf("klasa 1a = %q, want 2", got)
	}
	if st := r.Fields["ile"].Stat; !strings.Contains(st, "ff-stat") || !strings.Contains(st, "5") {
		t.Errorf("ile stat = %q", st)
	}
	if _, ok := r.Fields["ciasta"]; ok {
		t.Error("field shown in the panel must not get a stat at the field")
	}
	if !strings.Contains(r.Info, T("en", "panel.title")) || strings.Contains(r.Info, T("pl", "panel.title")) {
		t.Errorf("panel not in form language: %s", r.Info)
	}

	send("c@example.com", "nie", "1b", "1")
	r = get()
	if got := r.Fields["szatnia"].Opts["|nie"].T; got != "1 / 1" {
		t.Errorf("after 3rd submission szatnia nie = %q", got)
	}
	if !strings.Contains(r.Fields["ile"].Stat, "6") {
		t.Errorf("after 3rd submission ile = %q", r.Fields["ile"].Stat)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/form/org/bal", nil))
	if !strings.Contains(w.Body.String(), `data-live="/form/org/bal/live"`) {
		t.Error("form page does not point to its live state")
	}

	if _, err := db.DB.Exec(`UPDATE formtemplates SET status='closed' WHERE id=$1`, templID); err != nil {
		t.Fatal(err)
	}
	if w := liveGet(t, router, "/form/org/bal/live", ""); w.Code != http.StatusGone || strings.Contains(w.Body.String(), "info") {
		t.Errorf("closed form: %d %s", w.Code, w.Body)
	}
}

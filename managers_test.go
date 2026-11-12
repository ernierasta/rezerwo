package main

// Loginy menedżerów: dodawanie przez konto organizacji, logowanie menedżera,
// brak uprawnień menedżera do zarządzania loginami, unieważnianie sesji.

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

func managersRouter(db *DB, cs *sessions.CookieStore) http.Handler {
	rtr := mux.NewRouter()
	rtr.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		if u, ok := adminUser(w, r, db, cs); ok {
			w.Write([]byte("org:" + u.URL))
		}
	})
	rtr.HandleFunc("/admin/managers", ManagersPage(db, cs))
	rtr.HandleFunc("/api/managers", ManagerCreateAPI(db, cs)).Methods("POST")
	rtr.HandleFunc("/api/managers/{id:[0-9]+}", ManagerSaveAPI(db, cs)).Methods("POST")
	rtr.HandleFunc("/api/managers/{id:[0-9]+}", ManagerDeleteAPI(db, cs)).Methods("DELETE")
	rtr.HandleFunc("/api/login", LoginAPI(db, cs))
	return rtr
}

func TestManagers(t *testing.T) {
	db := newTestDB(t)
	h, _ := hashPassword("owner-pass")
	orgID := must(t)(db.UserAdd(&User{Email: "org@example.com", URL: "org", Passwd: h}))
	otherID := addUser(t, db, "other@example.com", "other")
	cs := sessions.NewCookieStore([]byte("0123456789abcdef0123456789abcdef"))
	rtr := managersRouter(db, cs)

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
	canAdmin := func(c []*http.Cookie) bool {
		rec := do("GET", "/admin", "", c)
		return rec.Code == 200 && rec.Body.String() == "org:org"
	}

	owner := login("org@example.com", "owner-pass")

	// walidacja
	for _, body := range []string{
		`{"email":"zly","password":"dlugiehaslo"}`,
		`{"email":"m@example.com","password":"krotkie"}`,
		`{"email":"ORG@example.com","password":"dlugiehaslo"}`, // login organizacji
		`{"email":"other@example.com","password":"dlugiehaslo"}`,
	} {
		if rec := do("POST", "/api/managers", body, owner); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", body, rec.Code)
		}
	}

	if rec := do("POST", "/api/managers", `{"email":" Jan@Example.com ","notes":"Jan - kasa","password":"haslo-jana"}`, owner); rec.Code != 200 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	mm, _ := db.ManagerGetAll(orgID)
	if len(mm) != 1 || mm[0].Email != "jan@example.com" || mm[0].Notes.String != "Jan - kasa" || !isBcryptHash(mm[0].Passwd.String) {
		t.Fatalf("stored manager: %+v", mm)
	}
	mid := mm[0].ID
	if rec := do("POST", "/api/managers", `{"email":"JAN@example.com","password":"haslo-jana"}`, owner); rec.Code != http.StatusBadRequest {
		t.Fatalf("duplicate e-mail accepted: %d", rec.Code)
	}

	// menedżer nie dostaje powiadomień o zamówieniach
	if emails, _ := db.AdminGetEmails(orgID); len(emails) != 0 {
		t.Fatalf("manager in order notification recipients: %v", emails)
	}

	// logowanie menedżera (bez względu na wielkość liter), panel jego organizacji
	if rec := do("POST", "/api/login", `{"email":"jan@example.com","password":"zle-haslo"}`, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong manager password: %d", rec.Code)
	}
	mgr := login("Jan@example.com", "haslo-jana")
	if !canAdmin(mgr) {
		t.Fatal("manager can not open panel")
	}
	if a, _ := db.ManagerGetByID(mid, orgID); !a.LastLogin.Valid {
		t.Fatal("last login not stored")
	}

	// menedżer nie zarządza loginami
	if rec := do("POST", "/api/managers", `{"email":"x@example.com","password":"dlugiehaslo"}`, mgr); rec.Code != http.StatusForbidden {
		t.Fatalf("manager created manager: %d", rec.Code)
	}
	if rec := do("DELETE", "/api/managers/"+itoa(mid), "", mgr); rec.Code != http.StatusForbidden {
		t.Fatalf("manager deleted manager: %d", rec.Code)
	}
	if rec := do("GET", "/admin/managers", "", mgr); rec.Code != 200 || strings.Contains(rec.Body.String(), `id="manager-form"`) {
		t.Fatalf("manager page for manager: %d, form shown: %v", rec.Code, strings.Contains(rec.Body.String(), `id="manager-form"`))
	}
	if rec := do("GET", "/admin/managers", "", owner); rec.Code != 200 || !strings.Contains(rec.Body.String(), "jan@example.com") || !strings.Contains(rec.Body.String(), `id="manager-form"`) {
		t.Fatalf("manager page for owner: %d", rec.Code)
	}
	// zakładka "Dostęp" tylko dla konta organizacji
	if rec := do("GET", "/admin/managers", "", mgr); strings.Contains(rec.Body.String(), `href="/admin/managers"`) {
		t.Fatal("manager sees the access tab")
	}
	if rec := do("GET", "/admin/managers", "", owner); !strings.Contains(rec.Body.String(), `href="/admin/managers"`) {
		t.Fatal("owner does not see the access tab")
	}

	// inna organizacja nie zmieni cudzego menedżera
	db.UserSetPass("other@example.com", h)
	other := login("other@example.com", "owner-pass")
	if rec := do("POST", "/api/managers/"+itoa(mid), `{"email":"jan@example.com","password":"przejete-haslo"}`, other); rec.Code != http.StatusNotFound {
		t.Fatalf("other org changed manager: %d", rec.Code)
	}
	_ = otherID

	// zmiana opisu bez hasła nie wylogowuje
	if rec := do("POST", "/api/managers/"+itoa(mid), `{"email":"jan@example.com","notes":"Jan","password":""}`, owner); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	if !canAdmin(mgr) {
		t.Fatal("manager logged out by notes change")
	}
	// nowe hasło wylogowuje
	if rec := do("POST", "/api/managers/"+itoa(mid), `{"email":"jan@example.com","notes":"Jan","password":"nowe-haslo-jana"}`, owner); rec.Code != 200 {
		t.Fatalf("save password: %d", rec.Code)
	}
	if canAdmin(mgr) {
		t.Fatal("old session valid after password change")
	}
	mgr = login("jan@example.com", "nowe-haslo-jana")
	if !canAdmin(mgr) {
		t.Fatal("new password does not work")
	}

	// usunięcie wylogowuje
	if rec := do("DELETE", "/api/managers/"+itoa(mid), "", owner); rec.Code != 200 {
		t.Fatalf("delete: %d", rec.Code)
	}
	if canAdmin(mgr) {
		t.Fatal("deleted manager still logged in")
	}
	if rec := do("POST", "/api/login", `{"email":"jan@example.com","password":"nowe-haslo-jana"}`, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("deleted manager can log in: %d", rec.Code)
	}
	if !canAdmin(owner) {
		t.Fatal("owner session broken")
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

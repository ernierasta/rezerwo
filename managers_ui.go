package main

// managers_ui.go: loginy menedżerów organizacji - /admin/managers.
//
// Menedżer (admins, type=MANAGER) loguje się własnym e-mailem i hasłem
// i ma dostęp do całego panelu swojej organizacji. Loginy dodaje, zmienia
// i usuwa tylko konto organizacji (users) - menedżer widzi listę bez edycji.
// Zmiana e-maila albo hasła i usunięcie od razu wylogowują menedżera
// (sessionLogin w auth.go).

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// MINPASSLEN - najkrótsze hasło menedżera.
const MINPASSLEN = 8

type nManager struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Notes     string `json:"notes"`
	LastLogin string `json:"-"`
	// Me - to jest zalogowany menedżer
	Me bool `json:"-"`
}

type nManagersVars struct {
	NPage
	OwnerEmail string
	// CanEdit - zalogowane konto organizacji (menedżer tylko ogląda)
	CanEdit  bool
	Managers []nManager
}

// ManagersPage - /admin/managers
func ManagersPage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		login, ok := sessionLogin(r, db, cs)
		if !ok {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		lang := PanelLang(r)
		user := login.User

		vars := nManagersVars{
			NPage:      NPage{Title: T(lang, "managers.title"), OrgName: orgName(user)},
			OwnerEmail: user.Email,
			CanEdit:    !login.Manager,
		}
		mm, err := db.ManagerGetAll(user.ID)
		if err != nil {
			log.Printf("ManagersPage: %v", err)
		}
		for _, m := range mm {
			vars.Managers = append(vars.Managers, nManager{
				ID:        m.ID,
				Email:     m.Email,
				Notes:     m.Notes.String,
				LastLogin: FormatDate(lang, m.LastLogin.Int64),
				Me:        login.Manager && m.Email == login.Email,
			})
		}
		nRender(w, lang, vars, "tmpl/n_managers.html")
	}
}

// adminOwner - jak adminUser, ale tylko dla konta organizacji; menedżer
// dostaje 403 (JSON).
func adminOwner(w http.ResponseWriter, r *http.Request, db *DB, cs *sessions.CookieStore) (User, bool) {
	login, ok := sessionLogin(r, db, cs)
	if !ok {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return User{}, false
	}
	if login.Manager {
		jsonMsg(w, http.StatusForbidden, map[string]string{"msg": T(PanelLang(r), "managers.owner_only")})
		return User{}, false
	}
	return login.User, true
}

type nManagerMsg struct {
	Email    string `json:"email"`
	Notes    string `json:"notes"`
	Password string `json:"password"`
}

// validEmail - prosta kontrola adresu logowania: coś@domena.tld, bez spacji.
func validEmail(s string) bool {
	at := strings.LastIndex(s, "@")
	return at > 0 && at < len(s)-1 && strings.Contains(s[at+1:], ".") &&
		!strings.ContainsAny(s, " \t\r\n,;<>\"") && utf8.RuneCountInString(s) <= 254
}

// managerFromMsg sprawdza dane z formularza i przenosi je do a. Puste hasło
// przy zmianie (a.ID != 0) zostawia dotychczasowe. Zwraca komunikat błędu
// i nazwę pola, którego dotyczy.
func managerFromMsg(db *DB, lang string, m nManagerMsg, a *Admin) (string, string) {
	email := strings.ToLower(strings.TrimSpace(m.Email))
	if !validEmail(email) {
		return T(lang, "managers.bad_email"), "email"
	}
	taken, err := db.LoginEmailTaken(email, a.ID)
	if err != nil {
		log.Printf("managerFromMsg: %v", err)
		return T(lang, "common.save_error"), ""
	}
	if taken {
		return T(lang, "managers.email_taken"), "email"
	}
	if m.Password != "" || a.ID == 0 {
		if utf8.RuneCountInString(m.Password) < MINPASSLEN {
			return T(lang, "managers.short_password", MINPASSLEN), "password"
		}
		h, err := hashPassword(m.Password)
		if err != nil {
			log.Printf("managerFromMsg: hash: %v", err)
			return T(lang, "common.save_error"), ""
		}
		a.Passwd = ToNS(h)
	}
	a.Email = email
	a.Notes = ToNS(strings.TrimSpace(m.Notes))
	return "", ""
}

func readManagerMsg(w http.ResponseWriter, r *http.Request) (nManagerMsg, bool) {
	var m nManagerMsg
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&m); err != nil {
		jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "bad request"})
		return m, false
	}
	return m, true
}

// ManagerCreateAPI - POST /api/managers {email, notes, password}
func ManagerCreateAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminOwner(w, r, db, cs)
		if !ok {
			return
		}
		m, ok := readManagerMsg(w, r)
		if !ok {
			return
		}
		lang := PanelLang(r)
		a := Admin{UserID: user.ID}
		if msg, field := managerFromMsg(db, lang, m, &a); msg != "" {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": msg, "field": field})
			return
		}
		id, err := db.ManagerAdd(&a)
		if err != nil {
			log.Printf("ManagerCreateAPI: %v", err)
			jsonMsg(w, http.StatusInternalServerError, map[string]string{"msg": T(lang, "common.save_error")})
			return
		}
		log.Printf("ManagerCreateAPI: org %d added manager %q", user.ID, a.Email)
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "created", "id": id})
	}
}

func ownedManager(w http.ResponseWriter, r *http.Request, db *DB, user User) (Admin, bool) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "bad manager id"})
		return Admin{}, false
	}
	a, err := db.ManagerGetByID(id, user.ID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("ownedManager %d: %v", id, err)
		}
		jsonMsg(w, http.StatusNotFound, map[string]string{"msg": "manager not found"})
		return Admin{}, false
	}
	return a, true
}

// ManagerSaveAPI - POST /api/managers/{id} {email, notes, password}; puste
// hasło = bez zmiany.
func ManagerSaveAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminOwner(w, r, db, cs)
		if !ok {
			return
		}
		a, ok := ownedManager(w, r, db, user)
		if !ok {
			return
		}
		m, ok := readManagerMsg(w, r)
		if !ok {
			return
		}
		lang := PanelLang(r)
		if msg, field := managerFromMsg(db, lang, m, &a); msg != "" {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": msg, "field": field})
			return
		}
		if err := db.ManagerMod(&a); err != nil {
			log.Printf("ManagerSaveAPI: %v", err)
			jsonMsg(w, http.StatusInternalServerError, map[string]string{"msg": T(lang, "common.save_error")})
			return
		}
		log.Printf("ManagerSaveAPI: org %d changed manager %d (%q), new password: %v", user.ID, a.ID, a.Email, m.Password != "")
		jsonMsg(w, http.StatusOK, map[string]string{"msg": "saved"})
	}
}

// ManagerDeleteAPI - DELETE /api/managers/{id}
func ManagerDeleteAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminOwner(w, r, db, cs)
		if !ok {
			return
		}
		a, ok := ownedManager(w, r, db, user)
		if !ok {
			return
		}
		if err := db.ManagerDel(a.ID, user.ID); err != nil {
			log.Printf("ManagerDeleteAPI: %v", err)
			jsonMsg(w, http.StatusInternalServerError, map[string]string{"msg": T(PanelLang(r), "del.failed")})
			return
		}
		log.Printf("ManagerDeleteAPI: org %d removed manager %q", user.ID, a.Email)
		jsonMsg(w, http.StatusOK, map[string]string{"msg": "deleted", "next": "/admin/managers"})
	}
}

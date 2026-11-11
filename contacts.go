package main

// contacts.go: zapamiętane osoby (manager_contacts) - te, które menedżer albo
// konto organizacji dopisali do formularzy na stronie zgłaszania osób
// (form_fill.go). Menedżer widzi swoje na stronie startowej, konto
// organizacji - wszystkie w Katalogach (menedżer z zarządzaniem organizacją
// tylko swoje).
//
// Zmiana imienia, nazwiska albo e-maila poprawia też zgłoszenia, które ten sam
// dopisujący założył za tę osobę (forms.added_by), we wszystkich formularzach;
// zgłoszeń złożonych przez osobę samą nie ruszamy. Zgłoszenie pomijamy, gdy
// w tym formularzu jest już zgłoszenie z nowym e-mailem (jeden e-mail = jedno
// zgłoszenie). Usunięcie zabiera osobę tylko z listy - przy funkcjach zostaje.

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// contactFold - ile osób widać od razu, reszta pod "Starsze".
const contactFold = 8

// nContactRow - wiersz listy zapamiętanych osób (tmpl/n_contacts.html).
type nContactRow struct {
	ID      int64
	Name    string
	Surname string
	Email   string
	// By - kto ją wpisał (tylko w liście całej organizacji)
	By  string
	Old bool
}

// nContacts - sekcja "Zapamiętane osoby".
type nContacts struct {
	Show  bool
	Rows  []nContactRow
	Older int
	// All - lista całej organizacji (kolumna "wpisał(a)")
	All bool
	// Next - adres strony, na którą wracamy po usunięciu
	Next string
}

// contactsSection - zapamiętane osoby zalogowanego; konto organizacji
// (all) widzi osoby wszystkich menedżerów.
func contactsSection(db *DB, user User, login Login, lang, next string, all bool) nContacts {
	var cc []Contact
	var err error
	all = all && !login.Manager
	if all {
		cc, err = db.ContactGetAllOrg(user.ID)
	} else {
		cc, err = db.ContactGetAll(user.ID, login.AdderID())
	}
	if err != nil {
		log.Printf("contactsSection: %v", err)
	}
	s := nContacts{Show: len(cc) > 0, All: all, Next: next}
	for _, c := range cc {
		row := nContactRow{ID: c.ID, Name: c.Name, Surname: c.Surname, Email: c.Email, Old: len(s.Rows) >= contactFold}
		if all {
			row.By = c.By
			if c.AdminID == 0 {
				row.By = T(lang, "contacts.by_org")
			} else if row.By == "" {
				row.By = "?"
			}
		}
		if row.Old {
			s.Older++
		}
		s.Rows = append(s.Rows, row)
	}
	return s
}

// adminContact - zapamiętana osoba {id} z adresu, jeśli zalogowany może ją
// zmieniać: menedżer tylko swoje, konto organizacji wszystkie.
func adminContact(w http.ResponseWriter, r *http.Request, db *DB, cs *sessions.CookieStore) (User, Login, Contact, bool) {
	user, ok := adminUser(w, r, db, cs)
	if !ok {
		return user, Login{}, Contact{}, false
	}
	login := requestLogin(r)
	lang := PanelLang(r)
	id, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	c, err := db.ContactGet(id, user.ID)
	if err != nil || (login.Manager && c.AdminID != login.AdderID()) {
		jsonMsg(w, http.StatusNotFound, map[string]string{"msg": T(lang, "contacts.err.not_found")})
		return user, login, c, false
	}
	return user, login, c, true
}

// contactResp - odpowiedź na zmianę osoby.
type contactResp struct {
	Msg string `json:"msg"`
	// Forms - ile zgłoszeń poprawiono
	Forms int `json:"forms"`
	// Skipped - formularze, w których jest już zgłoszenie z nowym e-mailem
	Skipped []string `json:"skipped"`
}

// ContactSaveAPI - POST /api/contacts/{id} {name, surname, email}
func ContactSaveAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, login, c, ok := adminContact(w, r, db, cs)
		if !ok {
			return
		}
		lang := PanelLang(r)
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		if err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		var p fillAdd
		if err := json.Unmarshal(body, &p); err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		if bad := cleanPerson(lang, &p); bad != "" {
			field := "name"
			if !validEmail(p.Email) {
				field = "email"
			}
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": bad, "field": field})
			return
		}
		resp, status, msg := updateContact(db, user, c, Contact{ID: c.ID, Name: p.Name, Surname: p.Surname, Email: p.Email, AdminID: c.AdminID}, lang)
		if status != http.StatusOK {
			jsonMsg(w, status, map[string]string{"msg": msg, "field": "email"})
			return
		}
		log.Printf("ContactSaveAPI: %q contact %d: %q -> %q, forms %d, skipped %d", login.Email, c.ID, c.Email, p.Email, resp.Forms, len(resp.Skipped))
		jsonMsg(w, http.StatusOK, resp)
	}
}

// updateContact zapisuje osobę i poprawia zgłoszenia założone przez tego
// samego dopisującego. Zwraca status HTTP i komunikat błędu.
func updateContact(db *DB, user User, old, c Contact, lang string) (contactResp, int, string) {
	resp := contactResp{Msg: "saved", Skipped: []string{}}
	newEmail := !strings.EqualFold(old.Email, c.Email)
	if newEmail {
		cc, err := db.ContactGetAll(user.ID, old.AdminID)
		if err != nil {
			log.Printf("updateContact: %v", err)
			return resp, http.StatusTeapot, T(lang, "common.save_error")
		}
		for _, x := range cc {
			if x.ID != old.ID && strings.EqualFold(x.Email, c.Email) {
				return resp, http.StatusConflict, T(lang, "contacts.err.dup", c.Email)
			}
		}
	}
	if err := db.ContactUpdate(c, user.ID); err != nil {
		log.Printf("updateContact: contact %d: %v", c.ID, err)
		return resp, http.StatusTeapot, T(lang, "common.save_error")
	}

	ff, err := db.FormGetAllAddedBy(user.ID, old.AdminID, old.Email)
	if err != nil {
		log.Printf("updateContact: forms: %v", err)
		return resp, http.StatusTeapot, T(lang, "common.save_error")
	}
	for _, f := range ff {
		if newEmail {
			other, err := db.FormGetAllByEmail(c.Email, f.FormTemplateID, user.ID)
			if err != nil || len(other) > 0 {
				name := strconv.FormatInt(f.FormTemplateID, 10)
				if t, err := db.FormTemplateGetByID(f.FormTemplateID, user.ID); err == nil {
					name = t.Name
				}
				resp.Skipped = append(resp.Skipped, name)
				continue
			}
		}
		f.Name, f.Surname, f.Email = ToNS(c.Name), ToNS(c.Surname), ToNS(strings.ToLower(c.Email))
		if err := db.FormSetPerson(f); err != nil {
			log.Printf("updateContact: form %d: %v", f.ID, err)
			resp.Skipped = append(resp.Skipped, strconv.FormatInt(f.ID, 10))
			continue
		}
		for role, v := range map[string]string{ROLENAME: c.Name, ROLESURNAME: c.Surname, ROLEEMAIL: f.Email.String} {
			fid, err := db.FormFieldGetIDByName(role, f.FormTemplateID)
			if err != nil || fid == 0 {
				continue
			}
			if err := db.FormAnswerSet(&FormAnswer{Value: ToNS(v), FormFieldID: fid, FormID: f.ID, AddedBy: f.AddedBy}); err != nil {
				log.Printf("updateContact: form %d answer %s: %v", f.ID, role, err)
			}
		}
		resp.Forms++
	}
	return resp, http.StatusOK, ""
}

// ContactDelAPI - DELETE /api/contacts/{id}: tylko z listy podpowiedzi.
func ContactDelAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, login, c, ok := adminContact(w, r, db, cs)
		if !ok {
			return
		}
		if err := db.ContactDelByID(c.ID, user.ID); err != nil {
			log.Printf("ContactDelAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(PanelLang(r), "common.save_error")})
			return
		}
		log.Printf("ContactDelAPI: %q deleted contact %d (%q)", login.Email, c.ID, c.Email)
		jsonMsg(w, http.StatusOK, map[string]string{"msg": "deleted"})
	}
}

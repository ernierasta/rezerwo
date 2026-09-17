package main

// catalog_ui.go: nowe edytory katalogów - mail i konto bankowe (etap 6).
//
// Stare /admin/maileditor i /admin/bankacceditor zostają do czasu wyłączenia
// starego panelu. Nowe strony mają adresy z ID (/admin/mails/{id},
// /admin/accounts/{id}) i zapisują się jednym żądaniem JSON.

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// --- mail ------------------------------------------------------------------

type nMailVars struct {
	NPage
	ID        int64
	Name      string
	Subject   string
	Text      template.HTML
	RelatedTo string
	Sharable  bool
	UsedBy    []string
	// Attachments i Images pokazujemy tylko jako informację - pliki wgrywa
	// się na razie w starym edytorze.
	Attachments string
	Images      string
}

// MailPage - /admin/mails/{id}
func MailPage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, mail, ok := adminMail(w, r, db, cs, true)
		if !ok {
			return
		}
		lang := PanelLang(r)

		vars := nMailVars{
			NPage: NPage{
				Title:   mail.Name,
				Crumb:   mail.Name,
				OrgName: orgName(user),
			},
			ID:          mail.ID,
			Name:        mail.Name,
			Subject:     mail.Title.String,
			Text:        template.HTML(mail.Text),
			RelatedTo:   mail.RelatedTo,
			Sharable:    mail.Sharable,
			UsedBy:      mailUsedBy(db, user, mail.ID),
			Attachments: mail.AttachedFilesDelimited.String,
			Images:      mail.EmbeddedImgsDelimited.String,
		}
		nRender(w, lang, vars, "tmpl/n_mail.html")
	}
}

func adminMail(w http.ResponseWriter, r *http.Request, db *DB, cs *sessions.CookieStore, htmlErrors bool) (User, Notification, bool) {
	user, ok := adminUser(w, r, db, cs)
	if !ok {
		return User{}, Notification{}, false
	}
	fail := func(status int, msg string) {
		if htmlErrors {
			http.NotFound(w, r)
			return
		}
		jsonMsg(w, status, map[string]string{"msg": msg})
	}

	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		fail(http.StatusBadRequest, "bad mail id")
		return User{}, Notification{}, false
	}
	mail, err := db.NotificationGetByID(id, user.ID)
	if err != nil {
		fail(http.StatusNotFound, "mail not found")
		return User{}, Notification{}, false
	}
	return user, mail, true
}

// mailUsedBy mówi, które imprezy i formularze używają tego maila.
// Współdzielone maile zmienia się w jednym miejscu, więc admin powinien
// widzieć, czego jeszcze dotknie zmiana (rozdz. 5.3 HARMONIJKA.md).
func mailUsedBy(db *DB, user User, mailID int64) []string {
	var used []string

	events, err := db.EventGetAllByUserID(user.ID)
	if err != nil {
		log.Printf("mailUsedBy: events: %v", err)
	}
	for _, e := range events {
		if e.ThankYouNotificationsID.Int64 == mailID || e.AdminNotificationsID.Int64 == mailID {
			used = append(used, e.Name)
		}
	}

	templs, err := db.FormTemplateGetAll(user.ID)
	if err != nil {
		log.Printf("mailUsedBy: form templates: %v", err)
	}
	for _, t := range templs {
		if t.NotificationID.Int64 == mailID || t.AdminNotificationID.Int64 == mailID {
			used = append(used, t.Name)
		}
	}
	return used
}

type nMailMsg struct {
	Name      string `json:"name"`
	Subject   string `json:"subject"`
	Text      string `json:"text"`
	RelatedTo string `json:"related_to"`
	Sharable  bool   `json:"sharable"`
}

// MailSaveAPI - POST /api/mails/{id}
func MailSaveAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, mail, ok := adminMail(w, r, db, cs, false)
		if !ok {
			return
		}
		lang := PanelLang(r)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		var m nMailMsg
		if err := json.Unmarshal(body, &m); err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}

		if name := strings.TrimSpace(m.Name); name != "" {
			mail.Name = name
		}
		mail.Title = ToNS(strings.TrimSpace(m.Subject))
		mail.Text = removeTrailingEmptyParagraphs(m.Text)
		mail.Sharable = m.Sharable
		if m.RelatedTo == "events" || m.RelatedTo == "forms" {
			mail.RelatedTo = m.RelatedTo
		}
		mail.UpdatedDate = ToNI(time.Now().Unix())

		if err := db.NotificationModByID(&mail, user.ID); err != nil {
			log.Printf("MailSaveAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(lang, "common.save_error") + ": " + err.Error()})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "saved", "title": mail.Name})
	}
}

type nNewNamedMsg struct {
	Name string `json:"name"`
}

// MailCreateAPI - POST /api/mails
func MailCreateAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		name, ok := namedBody(w, r)
		if !ok {
			return
		}

		id, err := db.NotificationAdd(&Notification{
			Name:        name,
			Type:        "email",
			RelatedTo:   "events",
			Title:       ToNS(name),
			Text:        "",
			CreatedDate: time.Now().Unix(),
			UserID:      user.ID,
		})
		if err != nil {
			log.Printf("MailCreateAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "created", "id": id})
	}
}

func namedBody(w http.ResponseWriter, r *http.Request) (string, bool) {
	if r.Method != http.MethodPost {
		jsonMsg(w, http.StatusMethodNotAllowed, map[string]string{"msg": "POST only"})
		return "", false
	}
	body, _ := io.ReadAll(r.Body)
	var m nNewNamedMsg
	if err := json.Unmarshal(body, &m); err != nil {
		jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
		return "", false
	}
	name := strings.TrimSpace(m.Name)
	if name == "" {
		jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "empty name"})
		return "", false
	}
	return name, true
}

// --- konto bankowe ---------------------------------------------------------

type nAccountVars struct {
	NPage
	ID          int64
	Name        string
	IBAN        string
	Recipient   string
	Bank        string
	Currency    string
	Message     string
	VarSymbol   string
	AmountField string
	UsedBy      []string
}

// AccountPage - /admin/accounts/{id}
func AccountPage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, acc, ok := adminAccount(w, r, db, cs, true)
		if !ok {
			return
		}
		lang := PanelLang(r)

		varsym := ""
		if acc.VarSymbol.Valid && acc.VarSymbol.Int64 != 0 {
			varsym = strconv.FormatInt(acc.VarSymbol.Int64, 10)
		}

		vars := nAccountVars{
			NPage: NPage{
				Title:   acc.Name,
				Crumb:   acc.Name,
				OrgName: orgName(user),
			},
			ID:          acc.ID,
			Name:        acc.Name,
			IBAN:        acc.IBAN,
			Recipient:   acc.RecipientName.String,
			Bank:        acc.BankID.String,
			Currency:    acc.Currency,
			Message:     acc.Message.String,
			VarSymbol:   varsym,
			AmountField: acc.AmountField.String,
			UsedBy:      accountUsedBy(db, user, acc.ID),
		}
		nRender(w, lang, vars, "tmpl/n_account.html")
	}
}

func adminAccount(w http.ResponseWriter, r *http.Request, db *DB, cs *sessions.CookieStore, htmlErrors bool) (User, BankAccount, bool) {
	user, ok := adminUser(w, r, db, cs)
	if !ok {
		return User{}, BankAccount{}, false
	}
	fail := func(status int, msg string) {
		if htmlErrors {
			http.NotFound(w, r)
			return
		}
		jsonMsg(w, status, map[string]string{"msg": msg})
	}

	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		fail(http.StatusBadRequest, "bad account id")
		return User{}, BankAccount{}, false
	}
	acc, err := db.BankAccountGetByID(id, user.ID)
	if err != nil {
		fail(http.StatusNotFound, "account not found")
		return User{}, BankAccount{}, false
	}
	return user, acc, true
}

func accountUsedBy(db *DB, user User, accID int64) []string {
	var used []string

	events, err := db.EventGetAllByUserID(user.ID)
	if err != nil {
		log.Printf("accountUsedBy: events: %v", err)
	}
	for _, e := range events {
		if e.BankAccountsID.Int64 == accID {
			used = append(used, e.Name)
		}
	}

	templs, err := db.FormTemplateGetAll(user.ID)
	if err != nil {
		log.Printf("accountUsedBy: form templates: %v", err)
	}
	for _, t := range templs {
		if t.BankAccountID.Int64 == accID {
			used = append(used, t.Name)
		}
	}
	return used
}

type nAccountMsg struct {
	Name        string `json:"name"`
	IBAN        string `json:"iban"`
	Recipient   string `json:"recipient"`
	Bank        string `json:"bank"`
	Currency    string `json:"currency"`
	Message     string `json:"message"`
	VarSymbol   string `json:"varsymbol"`
	AmountField string `json:"amountfield"`
}

// AccountSaveAPI - POST /api/accounts/{id}
func AccountSaveAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		_, acc, ok := adminAccount(w, r, db, cs, false)
		if !ok {
			return
		}
		lang := PanelLang(r)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		var m nAccountMsg
		if err := json.Unmarshal(body, &m); err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}

		if name := strings.TrimSpace(m.Name); name != "" {
			acc.Name = name
		}
		acc.IBAN = strings.TrimSpace(m.IBAN)
		acc.RecipientName = ToNS(strings.TrimSpace(m.Recipient))
		acc.BankID = ToNS(strings.TrimSpace(m.Bank))
		acc.Message = ToNS(strings.TrimSpace(m.Message))
		acc.AmountField = ToNS(strings.TrimSpace(m.AmountField))
		if c := strings.TrimSpace(m.Currency); c != "" {
			acc.Currency = c
		}
		if vs := strings.TrimSpace(m.VarSymbol); vs != "" {
			n, err := strconv.ParseInt(vs, 10, 64)
			if err != nil {
				jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "account.varsymbol_number")})
				return
			}
			acc.VarSymbol = ToNI(n)
		} else {
			acc.VarSymbol = ToNI(0)
		}

		if err := db.BankAccountMod(&acc); err != nil {
			log.Printf("AccountSaveAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(lang, "common.save_error") + ": " + err.Error()})
			return
		}

		resp := map[string]interface{}{"msg": "saved", "title": acc.Name}
		if acc.IBAN == "" {
			resp["warning"] = T(lang, "account.no_iban")
		}
		jsonMsg(w, http.StatusOK, resp)
	}
}

// AccountCreateAPI - POST /api/accounts
func AccountCreateAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		name, ok := namedBody(w, r)
		if !ok {
			return
		}

		id, err := db.BankAccountAdd(&BankAccount{
			Name:     name,
			Currency: "CZK",
			UserID:   user.ID,
		})
		if err != nil {
			log.Printf("AccountCreateAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": fmt.Sprint(err)})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "created", "id": id})
	}
}

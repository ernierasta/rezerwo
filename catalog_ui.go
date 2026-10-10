package main

// catalog_ui.go: edytory katalogów - mail i konto bankowe. Adresy z ID
// (/admin/mails/{id}, /admin/accounts/{id}), zapis jednym żądaniem JSON.

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"sort"
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
	UsedBy    []string
	// Attachments i Images - nazwy plików z media/<org>/email/ rozdzielone ";"
	Attachments string
	Images      string
	// Vars - JSON dla edytora: zmienne zamówienia i nazwy kont do kodu QR
	Vars string
	// Back - dokąd wrócić po zapisie/anulowaniu (impreza, formularz, katalogi)
	Back      string
	BackLabel string
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
			UsedBy:      mailUsedBy(db, user, mail.ID),
			Attachments: mail.AttachedFilesDelimited.String,
			Images:      mail.EmbeddedImgsDelimited.String,
			Vars:        mailEditorVars(db, user),
		}
		vars.Back, vars.BackLabel = backLink(r, lang, "catalog.mails")
		vars.UpURL, vars.UpLabel = upLink(r, db, user, lang)
		nRender(w, lang, vars, "tmpl/n_mail.html")
	}
}

func mailEditorVars(db *DB, user User) string {
	accounts := []string{}
	accs, err := db.BankAccountGetAll(user.ID)
	if err != nil {
		log.Printf("mailEditorVars: accounts: %v", err)
	}
	for _, a := range accs {
		accounts = append(accounts, a.Name)
	}
	b, _ := json.Marshal(map[string]interface{}{"vars": eventMailVars, "accounts": accounts})
	return string(b)
}

// backLink - adres powrotu z edytora maila albo konta. Imprezy i formularze
// podają ?back=/admin/events/26#mail-gosc; przyjmujemy tylko adresy panelu,
// żeby parametr nie przekierował na obcą stronę.
func backLink(r *http.Request, lang, catalogLabel string) (string, string) {
	back := r.URL.Query().Get("back")
	if strings.HasPrefix(back, "/admin/") && !strings.ContainsAny(back, "\\\r\n") && !strings.HasPrefix(back, "//") {
		return back, T(lang, "common.back")
	}
	return "/admin/catalogs", T(lang, catalogLabel)
}

// upLink - strona nadrzędna maila i konta: impreza/formularz, z którego
// przyszedł admin (?back=), a jeśli nie - lista katalogów.
func upLink(r *http.Request, db *DB, user User, lang string) (string, string) {
	back, label := backLink(r, lang, "catalog.title")
	path, _, _ := strings.Cut(strings.TrimPrefix(back, "/admin/"), "#")
	path, _, _ = strings.Cut(path, "?")
	kind, ids, _ := strings.Cut(path, "/")
	if id, err := strconv.ParseInt(ids, 10, 64); err == nil {
		switch kind {
		case "events":
			if e, err := db.EventGetByID(id); err == nil && e.UserID == user.ID {
				label = e.Name
			}
		case "forms":
			if t, err := db.FormTemplateGetByID(id, user.ID); err == nil {
				label = t.Name
			}
		}
	}
	return back, label
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
	// nazwy plików z media/<org>/email/ rozdzielone ";" - nil, gdy strona ich nie wysyła
	Attachments *string `json:"attachments"`
	Images      *string `json:"images"`
}

// cleanFileList porządkuje listę plików "a.pdf; b.jpg;" -> "a.pdf;b.jpg".
// Ścieżki odrzucamy, pliki muszą leżeć w katalogu maili organizacji.
func cleanFileList(s string) string {
	out := []string{}
	for _, f := range strings.Split(s, ";") {
		f = strings.TrimSpace(f)
		if f == "" || strings.ContainsAny(f, "/\\") || strings.HasPrefix(f, ".") {
			continue
		}
		out = append(out, f)
	}
	return strings.Join(out, ";")
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
		if err := checkMailTemplate(mail.Title.String, mail.Text); err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "mail.template_error") + ": " + err.Error()})
			return
		}
		if m.RelatedTo == "events" || m.RelatedTo == "forms" {
			mail.RelatedTo = m.RelatedTo
		}
		if m.Attachments != nil {
			mail.AttachedFilesDelimited = ToNS(cleanFileList(*m.Attachments))
		}
		if m.Images != nil {
			mail.EmbeddedImgsDelimited = ToNS(cleanFileList(*m.Images))
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

// nNewMsg - nowy element katalogu: pusty albo kopia własnego (copy_of).
type nNewMsg struct {
	Name   string `json:"name"`
	CopyOf int64  `json:"copy_of"`
}

func newBody(w http.ResponseWriter, r *http.Request) (nNewMsg, bool) {
	var m nNewMsg
	if r.Method != http.MethodPost {
		jsonMsg(w, http.StatusMethodNotAllowed, map[string]string{"msg": "POST only"})
		return m, false
	}
	body, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(body, &m); err != nil {
		jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
		return m, false
	}
	m.Name = strings.TrimSpace(m.Name)
	if m.Name == "" {
		jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "empty name"})
		return m, false
	}
	return m, true
}

// --- konto bankowe ---------------------------------------------------------

type nAccountVars struct {
	NPage
	ID        int64
	Name      string
	IBAN      string
	Recipient string
	Bank      string
	Currency  string
	VarSymbol string
	UsedBy    []string
	Back      string
	BackLabel string
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
			ID:        acc.ID,
			Name:      acc.Name,
			IBAN:      acc.IBAN,
			Recipient: acc.RecipientName.String,
			Bank:      acc.BankID.String,
			Currency:  acc.Currency,
			VarSymbol: varsym,
			UsedBy:    accountUsedBy(db, user, acc.ID),
		}
		vars.Back, vars.BackLabel = backLink(r, lang, "catalog.accounts")
		vars.UpURL, vars.UpLabel = upLink(r, db, user, lang)
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
	Name      string `json:"name"`
	IBAN      string `json:"iban"`
	Recipient string `json:"recipient"`
	Bank      string `json:"bank"`
	Currency  string `json:"currency"`
	VarSymbol string `json:"varsymbol"`
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
		} else if iban, _ := splitIBAN(acc.IBAN); !ibanValid(iban) {
			resp["warning"] = T(lang, "account.bad_iban")
		}
		jsonMsg(w, http.StatusOK, resp)
	}
}

// AccountCreateAPI - POST /api/accounts: puste konto albo kopia własnego.
func AccountCreateAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		m, ok := newBody(w, r)
		if !ok {
			return
		}

		acc := BankAccount{Currency: "CZK"}
		if m.CopyOf != 0 {
			src, err := db.BankAccountGetByID(m.CopyOf, user.ID)
			if err != nil {
				jsonMsg(w, http.StatusNotFound, map[string]string{"msg": "account not found"})
				return
			}
			acc = src
			acc.ID = 0
		}
		acc.Name = m.Name
		acc.UserID = user.ID

		id, err := db.BankAccountAdd(&acc)
		if err != nil {
			log.Printf("AccountCreateAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": fmt.Sprint(err)})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "created", "id": id})
	}
}

type nAccountSource struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	IBAN      string `json:"iban"`
	Recipient string `json:"recipient"`
	Currency  string `json:"currency"`
	VarSymbol int64  `json:"varsymbol"`
}

// AccountSourcesAPI - GET /api/accounts/sources: konta do okna "Nowe konto".
func AccountSourcesAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		mine := []nAccountSource{}
		accs, err := db.BankAccountGetAll(user.ID)
		if err != nil {
			log.Printf("AccountSourcesAPI: %v", err)
		}
		for _, a := range accs {
			mine = append(mine, nAccountSource{
				ID:        a.ID,
				Name:      a.Name,
				IBAN:      a.IBAN,
				Recipient: a.RecipientName.String,
				Currency:  a.Currency,
				VarSymbol: a.VarSymbol.Int64,
			})
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"mine": mine})
	}
}

// --- sala --------------------------------------------------------------------

// pusta sala dostaje najczęstszy rozmiar sal w bazie
const (
	newRoomWidth  = 800
	newRoomHeight = 950
)

// RoomCreateAPI - POST /api/rooms: pusta sala albo kopia własnej (z meblami).
func RoomCreateAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		m, ok := newBody(w, r)
		if !ok {
			return
		}

		var id int64
		var err error
		if m.CopyOf != 0 {
			if !roomOfUser(db, user.ID, m.CopyOf) {
				jsonMsg(w, http.StatusNotFound, map[string]string{"msg": "room not found"})
				return
			}
			id, err = db.RoomWithFurnituresCopy(m.CopyOf, m.Name)
		} else {
			id, err = db.RoomAdd(&Room{Name: m.Name, Width: newRoomWidth, Height: newRoomHeight, Sharable: ToNI(0)})
		}
		if err == nil {
			err = db.RoomAssignToUser(user.ID, id)
		}
		if err != nil {
			log.Printf("RoomCreateAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": fmt.Sprint(err)})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "created", "id": id})
	}
}

func roomOfUser(db *DB, userID, roomID int64) bool {
	rooms, err := db.RoomGetAllByUserID(userID)
	if err != nil {
		log.Printf("roomOfUser: %v", err)
		return false
	}
	for _, room := range rooms {
		if room.ID == roomID {
			return true
		}
	}
	return false
}

// nRoomShape - prostokąt (albo elipsa) w miniaturze planu sali.
type nRoomShape struct {
	Type    string  `json:"t"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	W       float64 `json:"w"`
	H       float64 `json:"h"`
	Ellipse bool    `json:"e,omitempty"`
	Color   string  `json:"c,omitempty"`
}

type nRoomSource struct {
	ID     int64        `json:"id"`
	Name   string       `json:"name"`
	Width  int64        `json:"width"`
	Height int64        `json:"height"`
	Size   string       `json:"size"`
	Seats  string       `json:"seats"`
	Shapes []nRoomShape `json:"shapes"`
}

// roomShapes - meble sali do miniatury, z tą samą geometrią co plan gościa
// (room_svg.go).
func roomShapes(db *DB, roomID int64) ([]nRoomShape, int) {
	out := []nRoomShape{}
	seats := 0
	ff, err := db.FurnitureGetAllByRoomID(roomID)
	if err != nil {
		log.Printf("roomShapes: %v", err)
		return out, 0
	}
	for _, f := range ff {
		s := nRoomShape{Type: f.Type, X: float64(f.X), Y: float64(f.Y), W: float64(f.Width.Int64), H: float64(f.Height.Int64), Color: f.Color.String}
		switch f.Type {
		case "table":
			w, h, shape := tableSize(f.Orientation.String, f.Capacity.Int64)
			if s.W <= 0 {
				s.W = w
			}
			if s.H <= 0 {
				s.H = h
			}
			s.X += svgTableMargin
			s.Y += svgTableMargin
			s.Ellipse = shape == "ellipse"
			s.Color = ""
		case "chair":
			s.X += svgChairMargin
			s.Y += svgChairMargin
			s.W, s.H = svgChairSize, svgChairSize
			s.Color = ""
			seats++
		case "label":
			if s.H <= 0 {
				s.H = svgChairSize
			}
		}
		out = append(out, s)
	}
	return out, seats
}

// RoomSourcesAPI - GET /api/rooms/sources: sale do okna "Nowa sala",
// z miniaturą planu.
func RoomSourcesAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		lang := PanelLang(r)
		mine := []nRoomSource{}
		rooms, err := db.RoomGetAllByUserID(user.ID)
		if err != nil {
			log.Printf("RoomSourcesAPI: %v", err)
		}
		for _, room := range rooms {
			shapes, seats := roomShapes(db, room.ID)
			mine = append(mine, nRoomSource{
				ID:     room.ID,
				Name:   room.Name,
				Width:  room.Width,
				Height: room.Height,
				Size:   T(lang, "catalog.size", room.Width, room.Height),
				Seats:  TN(lang, "count.seat", int64(seats)),
				Shapes: shapes,
			})
		}
		// najnowsze na górze
		sort.SliceStable(mine, func(i, j int) bool { return mine[i].ID > mine[j].ID })
		jsonMsg(w, http.StatusOK, map[string]interface{}{"mine": mine})
	}
}

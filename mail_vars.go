package main

// mail_vars.go: dane zamówienia w mailach imprez i ich podgląd w edytorze.
//
// Edytor maila (js/n_mail.js) pokazuje zmienne jako "pastylki", a w bazie
// zostają zwykłe akcje szablonu ({{.Name}}, {{.QRPayMail "Konto"}}), więc
// istniejące maile działają bez zmian. Tu jest lista tych zmiennych, wspólne
// uzupełnianie OrderFuncs dla obu ścieżek wysyłki (zamówienie gościa i wysyłka
// z panelu) oraz podgląd i mail próbny.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	texttemplate "text/template"

	"github.com/gorilla/sessions"
)

// mailVar - zmienna do wstawienia w treść maila imprezy. Code to akcja bez
// nawiasów, Label klucz tłumaczenia. InSubject: czy ma sens w temacie.
// Account: akcja bierze nazwę konta bankowego ({{.PayInfo "Konto"}}).
type mailVar struct {
	Code      string `json:"code"`
	Label     string `json:"label"`
	InSubject bool   `json:"subject"`
	Account   bool   `json:"account,omitempty"`
}

var eventMailVars = []mailVar{
	{".Name", "mail.var.name", true, false},
	{".Surname", "mail.var.surname", true, false},
	{".Email", "mail.var.email", false, false},
	{".Phone", "mail.var.phone", false, false},
	{".Notes", "mail.var.notes", false, false},
	{".Sits", "mail.var.sits", true, false},
	{".Rooms", "mail.var.rooms", true, false},
	{".TotalPrice", "mail.var.total", true, false},
	{".EventName", "mail.var.event", true, false},
	{".EventDate", "mail.var.event_date", true, false},
	// dane do przelewu dla tych, którzy nie zapłacą kodem QR (pay.go)
	{".PayInfo", "mail.var.pay_info", false, true},
	{".PayAccount", "mail.var.pay_account", false, true},
	{".PayIBAN", "mail.var.pay_iban", false, true},
	{".PayVS", "mail.var.pay_vs", false, true},
	{".PayMessage", "mail.var.pay_message", false, true},
}

// complete uzupełnia OrderFuncs tak, żeby obie ścieżki wysyłki dawały te same
// wartości. Zamówienie gościa przynosi napisy z formularza (miejsca "12,13",
// sale jako ID), wysyłka z panelu - listy. Po uzupełnieniu .Sits i .Rooms są
// czytelne ("12, 13", "Sala duża"), a listy wystarczają do biletów PDF.
func (o *OrderFuncs) complete(db *DB) {
	if len(o.SitsL) == 0 && o.Sits != "" {
		o.SitsL = splitInts(o.Sits)
	}
	if len(o.PricesL) == 0 && o.Prices != "" {
		o.PricesL = splitInts(o.Prices)
	}
	if len(o.RoomsL) == 0 && o.Rooms != "" {
		names := map[int64]string{}
		for _, part := range strings.Split(o.Rooms, ",") {
			part = strings.TrimSpace(part)
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				o.RoomsL = append(o.RoomsL, part)
				continue
			}
			if _, ok := names[id]; !ok {
				room, err := db.RoomGetByID(id)
				if err != nil {
					log.Printf("OrderFuncs.complete: room %d: %v", id, err)
					names[id] = part
				} else {
					names[id] = room.Name
				}
			}
			o.RoomsL = append(o.RoomsL, names[id])
		}
	}

	if len(o.SitsL) > 0 {
		ss := make([]string, len(o.SitsL))
		for i, s := range o.SitsL {
			ss[i] = strconv.Itoa(s)
		}
		o.Sits = strings.Join(ss, ", ")
	}
	if len(o.RoomsL) > 0 {
		o.Rooms = strings.Join(uniqueStrings(o.RoomsL), ", ")
	}

	if o.EventID != 0 {
		event, err := db.EventGetByID(o.EventID)
		if err != nil {
			log.Printf("OrderFuncs.complete: event %d: %v", o.EventID, err)
			return
		}
		if o.EventName == "" {
			o.EventName = event.Name
		}
		if o.EventDate == "" {
			o.EventDate = FormatDate(event.Language.String, event.Date)
		}
		if o.Lang == "" {
			o.Lang = event.Language.String
		}
		o.payPrefix = event.PayMessage
	}
}

func splitInts(s string) []int {
	var out []int
	for _, part := range strings.Split(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out
}

func uniqueStrings(ss []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range ss {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// newEventOrderFuncs - dane zamówienia przy wysyłce maila z panelu.
func newEventOrderFuncs(name, surname, phone, email, notes, eventName string, sits, prices []int, rooms []string, eventID, totalPrice int64, user User, db *DB) *OrderFuncs {
	of := &OrderFuncs{
		EventID:     eventID,
		EventName:   eventName,
		TotalPrice:  strconv.FormatInt(totalPrice, 10),
		SitsL:       sits,
		PricesL:     prices,
		RoomsL:      rooms,
		Email:       email,
		Name:        name,
		Surname:     surname,
		Phone:       phone,
		Notes:       notes,
		DB:          db,
		User:        user,
		NameSurname: surname + "_" + name,
	}
	of.complete(db)
	return of
}

// orderFuncsFromOrder - dane zamówienia złożonego przez gościa.
func orderFuncsFromOrder(o Order, db *DB, user User) *OrderFuncs {
	of := &OrderFuncs{
		EventID:     o.EventID,
		TotalPrice:  o.TotalPrice,
		Sits:        o.Sits,
		Prices:      o.Prices,
		Rooms:       o.Rooms,
		Email:       o.Email,
		Name:        o.Name,
		Surname:     o.Surname,
		Phone:       o.Phone,
		Notes:       o.Notes,
		DB:          db,
		User:        user,
		NameSurname: o.Surname + "_" + o.Name,
	}
	of.complete(db)
	return of
}

// OrderSubject wypełnia zmienne w temacie maila. Temat to zwykły tekst, więc
// text/template (html/template zamieniłby "&" w nazwisku na "&amp;").
func OrderSubject(subject string, of *OrderFuncs) string {
	if !strings.Contains(subject, "{{") {
		return subject
	}
	t, err := texttemplate.New("subject").Parse(subject)
	if err != nil {
		log.Printf("OrderSubject: parse %q: %v", subject, err)
		return subject
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, of); err != nil {
		log.Printf("OrderSubject: exec %q: %v", subject, err)
		return subject
	}
	return buf.String()
}

// checkMailTemplate sprawdza składnię tematu i treści przed zapisem, żeby
// błąd wyszedł w edytorze, a nie po cichu przy wysyłce.
func checkMailTemplate(subject, text string) error {
	if _, err := texttemplate.New("subject").Parse(subject); err != nil {
		return err
	}
	if _, err := template.New("text").Parse(text); err != nil {
		return err
	}
	return nil
}

// sampleOrderFuncs - przykładowe zamówienie do podglądu i maila próbnego.
// Impreza: pierwsza, która używa tego maila, inaczej najnowsza.
func sampleOrderFuncs(db *DB, user User, lang string, mailID int64, preview bool) *OrderFuncs {
	events, err := db.EventGetAllByUserID(user.ID)
	if err != nil {
		log.Printf("sampleOrderFuncs: events: %v", err)
	}
	var event Event
	for _, e := range events {
		if e.ThankYouNotificationsID.Int64 == mailID || e.AdminNotificationsID.Int64 == mailID {
			event = e
			break
		}
		if e.Date > event.Date {
			event = e
		}
	}

	room := T(lang, "mail.sample.room")
	if event.ID != 0 {
		if rooms, err := db.EventGetRooms(event.ID); err == nil && len(rooms) > 0 {
			room = rooms[0].Name
		}
	}
	price := event.DefaultPrice
	if price <= 0 {
		price = 200
	}
	name := T(lang, "mail.sample.name")
	surname := T(lang, "mail.sample.surname")

	of := &OrderFuncs{
		EventID:     event.ID,
		EventName:   event.Name,
		TotalPrice:  strconv.FormatInt(2*price, 10),
		SitsL:       []int{12, 13},
		PricesL:     []int{int(price), int(price)},
		RoomsL:      []string{room, room},
		Email:       user.Email,
		Name:        name,
		Surname:     surname,
		Phone:       "+420 123 456 789",
		Notes:       T(lang, "mail.sample.notes"),
		DB:          db,
		User:        user,
		NameSurname: "podglad",
		Preview:     preview,
		Lang:        lang,
	}
	if of.EventName == "" {
		of.EventName = T(lang, "mail.sample.event")
	}
	of.complete(db)
	return of
}

type nMailPreviewMsg struct {
	Subject   string `json:"subject"`
	Text      string `json:"text"`
	RelatedTo string `json:"related_to"`
}

// renderSampleMail wypełnia temat i treść przykładowym zamówieniem.
func renderSampleMail(of *OrderFuncs, m nMailPreviewMsg) (string, string, error) {
	if err := checkMailTemplate(m.Subject, m.Text); err != nil {
		return "", "", err
	}
	t, _ := template.New("mail").Parse(m.Text)
	var buf bytes.Buffer
	if err := t.Execute(&buf, of); err != nil {
		return "", "", err
	}
	return OrderSubject(m.Subject, of), buf.String(), nil
}

func readPreviewMsg(w http.ResponseWriter, r *http.Request) (nMailPreviewMsg, bool) {
	var m nMailPreviewMsg
	body, err := io.ReadAll(r.Body)
	if err == nil {
		err = json.Unmarshal(body, &m)
	}
	if err != nil {
		jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
		return m, false
	}
	m.Text = removeTrailingEmptyParagraphs(m.Text)
	return m, true
}

// MailPreviewAPI - POST /api/mails/{id}/preview, z niezapisaną treścią.
func MailPreviewAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, mail, ok := adminMail(w, r, db, cs, false)
		if !ok {
			return
		}
		m, ok := readPreviewMsg(w, r)
		if !ok {
			return
		}
		var files []string
		if mail.AttachedFilesDelimited.String != "" {
			files = strings.Split(mail.AttachedFilesDelimited.String, ";")
		}
		writePreview(w, db, user, PanelLang(r), mail.ID, m, files)
	}
}

// MailSourcePreviewAPI - POST /api/mails/preview: podgląd szablonu albo
// maila do skopiowania w oknie "Nowy mail". Maile formularzy pokazujemy bez
// wypełniania (ich zmienne zależą od pól formularza).
func MailSourcePreviewAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		m, ok := readPreviewMsg(w, r)
		if !ok {
			return
		}
		if m.RelatedTo == "forms" {
			jsonMsg(w, http.StatusOK, map[string]interface{}{"subject": m.Subject, "html": m.Text, "to": user.Email})
			return
		}
		m.Text = fillOnlyAccount(db, user, m.Text)
		writePreview(w, db, user, PanelLang(r), 0, m, nil)
	}
}

func writePreview(w http.ResponseWriter, db *DB, user User, lang string, mailID int64, m nMailPreviewMsg, files []string) {
	of := sampleOrderFuncs(db, user, lang, mailID, true)
	subject, text, err := renderSampleMail(of, m)
	if err != nil {
		jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "mail.template_error") + ": " + err.Error()})
		return
	}
	if of.PDFAttached {
		files = append(files, T(lang, "mail.pdf_tickets"))
	}
	jsonMsg(w, http.StatusOK, map[string]interface{}{
		"subject": subject,
		"html":    text,
		"files":   files,
		"to":      user.Email,
	})
}

// MailTestAPI - POST /api/mails/{id}/test: mail próbny na adres organizacji,
// z przykładowym zamówieniem, prawdziwym kodem QR i biletami PDF.
func MailTestAPI(db *DB, mailConf MailConfig, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, mail, ok := adminMail(w, r, db, cs, false)
		if !ok {
			return
		}
		m, ok := readPreviewMsg(w, r)
		if !ok {
			return
		}
		lang := PanelLang(r)

		of := sampleOrderFuncs(db, user, lang, mail.ID, false)
		subject, text, err := renderSampleMail(of, m)
		if err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "mail.template_error") + ": " + err.Error()})
			return
		}

		if err := mailConf.SendAsOrg(user, user.Email, fmt.Sprintf("[%s] %s", T(lang, "mail.test_tag"), subject),
			mailHTMLHead+text+"</html>", append(mail.Files(user), of.AttachedFiles...),
			append(mail.Images(user), of.EmbeddedImgs...)); err != nil {
			log.Printf("MailTestAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]string{"msg": "sent", "to": user.Email})
	}
}

type OrderFuncs struct {
	EventID             int64
	EventName           string
	EventDate           string
	TotalPrice          string
	Sits, Rooms, Prices string
	SitsL, PricesL      []int
	RoomsL              []string
	Email               string
	Password            string
	Name, Surname       string
	Phone, Notes        string

	DB            *DB
	User          User
	NameSurname   string
	EmbeddedImgs  []string // obrazki cid: (QR) do dołączenia
	AttachedFiles []string // bilety PDF

	// Preview - podgląd w edytorze maila: kod QR jako zwykły obrazek,
	// bez generowania biletów PDF (zaznaczamy tylko, że byłyby w załączniku)
	Preview     bool
	PDFAttached bool

	// Lang - język etykiet w danych do przelewu (.PayInfo), język imprezy
	Lang string

	// payPrefix - początek wiadomości dla odbiorcy z imprezy (complete)
	payPrefix string
}

// qr generuje kod QR płatności za zamówienie
func (o *OrderFuncs) qr(accountName string) (string, string) {
	am := o.TotalPrice
	if ss := strings.Split(am, " "); len(ss) == 2 { // "500 Kč"
		am = ss[0]
	}
	return qrPayImage(o.DB, o.User, accountName, "_"+o.NameSurname, am, payMessage(o.payPrefix, o.NameSurname))
}

func (o *OrderFuncs) QRPay(accountName string) template.HTML {
	_, p := o.qr(accountName)
	return qrImgHTML(p)
}

func (o *OrderFuncs) QRPayMail(accountName string) template.HTML {
	if o.Preview {
		// w podglądzie szablonu konto może być jeszcze niewybrane
		if _, err := o.DB.BankAccountGetByName(accountName, o.User.ID); err != nil {
			return template.HTML(`<span style="display:inline-block;width:150px;height:150px;border:2px dashed #b9c3cf;color:#6e7885;text-align:center;line-height:150px">QR</span>`)
		}
		return o.QRPay(accountName)
	}
	name, p := o.qr(accountName)
	return embedImg(&o.EmbeddedImgs, name, p)
}

// PDF generates and attaches generated pdf files
func (o *OrderFuncs) PDF(info string) template.HTML {
	o.PDFAttached = true
	if o.Preview {
		return template.HTML("")
	}
	p := &TicketParams{
		UserName:         o.User.Organization.String,
		UserURL:          o.User.URL,
		EventName:        o.EventName,
		EventDescription: info,
		SeatNrs:          o.SitsL,
		Rooms:            o.RoomsL,
		Lang:             KnownLang(o.Lang),
	}

	attFiles, err := GenerateTicketsPDF(p)
	if err != nil {
		log.Printf("OrderFuncs-PDF: problem generating pdf ticket, %v", err)
	}

	o.AttachedFiles = append(o.AttachedFiles, attFiles...)
	return template.HTML("")
}

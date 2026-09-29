package main

// forms_guest.go: nowa strona formularza dla gościa (rozdz. 7.3 HARMONIJKA.md).
//
// Pola renderuje serwer (html/template) z tej samej definicji JSON, którą
// zapisywał formBuilder, więc gość widzi nowy formularz niezależnie od tego,
// którym edytorem admin go zbudował. JS (js/n_formpage.js) jest tylko do
// walidacji, mnożnika, sumy i wysyłki.
//
// Adresy: /form/{userurl}/{formurl} (strona), .../send (zapis odpowiedzi),
// .../done (podziękowanie).

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// doneKey podpisuje adres strony podziękowania, żeby nie dało się podejrzeć
// cudzej odpowiedzi, zgadując jej numer. Klucz żyje do restartu serwera -
// po restarcie stary link pokaże samo podziękowanie, bez danych.
var doneKey = func() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Printf("forms_guest: random key: %v", err)
	}
	return b
}()

func doneToken(formID int64) string {
	m := hmac.New(sha256.New, doneKey)
	fmt.Fprintf(m, "form:%d", formID)
	return hex.EncodeToString(m.Sum(nil))[:24]
}

type nFormPageVars struct {
	Title    string
	Lang     string
	FormName string
	Banner   template.HTML
	HowTo    template.HTML
	Info     template.HTML
	Rows     []RenderRow
	UniqID   string
	SendURL  string

	Open      bool
	Preview   bool   // admin ogląda zamknięty formularz
	CloseNote string // "Odpowiedzi przyjmujemy do 12.1.2027"

	HasMoney bool
	Currency string
	DraftKey string
}

// guestForm szuka organizacji i formularza z adresu. Dodatki do zamówienia
// nie mają własnej strony.
func guestForm(w http.ResponseWriter, r *http.Request, db *DB) (User, FormTemplate, bool) {
	userURL := mux.Vars(r)["userurl"]
	formURL := mux.Vars(r)["formurl"]
	user, err := db.UserGetByURL(userURL)
	if err != nil {
		guestError(w, r, DEFAULTLANG, "gform.no_org", userURL)
		return User{}, FormTemplate{}, false
	}
	templ, err := db.FormTemplateGetByURL(formURL, user.ID)
	if err != nil || isAddonsTemplate(templ) {
		guestError(w, r, DEFAULTLANG, "gform.no_form", formURL)
		return User{}, FormTemplate{}, false
	}
	return user, templ, true
}

func guestError(w http.ResponseWriter, r *http.Request, lang, key, what string) {
	w.WriteHeader(http.StatusNotFound)
	guestRender(w, lang, nErrorVars{Title: T(lang, key+"_title"), Text: T(lang, key, what)}, "tmpl/n_error.html")
}

// guestRender - strony bez imprezy/formularza (logowanie, błędy): temat pastel.
func guestRender(w http.ResponseWriter, lang string, vars interface{}, files ...string) {
	guestRenderTheme(w, lang, "pastel", vars, files...)
}

// guestRenderTheme - strona gościa z tematem graficznym imprezy/formularza.
func guestRenderTheme(w http.ResponseWriter, lang, theme string, vars interface{}, files ...string) {
	all := append([]string{}, files...)
	all = append(all, "tmpl/n_guest_base.html")
	name := strings.TrimPrefix(files[0], "tmpl/")
	t, err := template.New(name).Funcs(GuestFuncs(lang, theme)).ParseFiles(all...)
	if err != nil {
		log.Printf("guestRender %v: %v", files, err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	if err := t.ExecuteTemplate(w, "base", vars); err != nil {
		log.Printf("guestRender %v: %v", files, err)
	}
}

// isFormOwnerSession mówi, czy żądanie przyszło od admina tej organizacji.
func isFormOwnerSession(r *http.Request, db *DB, cs *sessions.CookieStore, user User) bool {
	if !isAdminSession(r, cs) {
		return false
	}
	session, err := cs.Get(r, AUTHCOOKIE)
	if err != nil {
		return false
	}
	email, _ := session.Values["email"].(string)
	u, err := db.UserGetByEmail(email)
	return err == nil && u.ID == user.ID
}

// formLang to język strony formularza dla gościa.
func formLang(t FormTemplate) string {
	return KnownLang(t.Language)
}

// formCurrency bierze walutę z konta bankowego formularza (kod QR).
func formCurrency(db *DB, user User, t FormTemplate) string {
	if t.BankAccountID.Valid && t.BankAccountID.Int64 > 0 {
		if ba, err := db.BankAccountGetByID(t.BankAccountID.Int64, user.ID); err == nil && ba.Currency != "" {
			return ba.Currency
		}
	}
	return "CZK"
}

// GuestFormPage - GET /form/{userurl}/{formurl}
func GuestFormPage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := guestForm(w, r, db)
		if !ok {
			return
		}
		lang := formLang(templ)

		defs, err := ParseFormDef(templ.Content.String)
		if err != nil {
			log.Printf("GuestFormPage: form %d: %v", templ.ID, err)
		}

		v := nFormPageVars{
			Title:    templ.Name,
			Lang:     lang,
			FormName: templ.Name,
			HowTo:    template.HTML(templ.HowTo.String),
			Rows:     BuildRenderRows(defs, templ.MoneyAmountFieldName.String),
			SendURL:  fmt.Sprintf("/form/%s/%s/send", user.URL, templ.URL),
			Open:     templ.IsOpen(nowUnix()),
			Currency: formCurrency(db, user, templ),
			DraftKey: fmt.Sprintf("rezerwo.gform.%d", templ.ID),
		}
		v.Banner = ParseBanner(templ.Banner.String).ImgHTML(user.URL)
		if v.Open && templ.CloseDate.Valid && templ.CloseDate.Int64 > 0 {
			v.CloseNote = T(lang, "gform.open_until", FormatDate(lang, templ.CloseDate.Int64))
		}
		if !v.Open && isFormOwnerSession(r, db, cs, user) {
			v.Preview = true
		}
		for _, row := range v.Rows {
			for _, f := range row.Fields {
				if f.IsMoney {
					v.HasMoney = true
				}
			}
		}

		// panel informacyjny: automatyczny, z wybranych pól albo własny tekst
		v.Info = RenderInfoPanel(db, user, templ, panelMode(templ))

		// formularz bez nazwiska jest anonimowy: dostaje unikatowy numer
		if !strings.Contains(templ.Content.String, ROLESURNAME) {
			b := make([]byte, 4)
			rand.Read(b)
			v.UniqID = hex.EncodeToString(b)
		}

		guestRenderTheme(w, lang, templ.Theme, v, "tmpl/n_formpage.html", "tmpl/n_formfield.html")
	}
}

type gformSendMsg struct {
	UniqID string            `json:"uniqid"`
	Data   []FormAnswersJson `json:"data"`
}

// GuestFormSendAPI - POST /form/{userurl}/{formurl}/send
func GuestFormSendAPI(db *DB, mailConf MailConfig) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := guestFormAPI(w, r, db)
		if !ok {
			return
		}
		lang := formLang(templ)
		if !templ.IsOpen(nowUnix()) {
			jsonMsg(w, http.StatusForbidden, map[string]string{"msg": T(lang, "gform.closed")})
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		var msg gformSendMsg
		if err := json.Unmarshal(body, &msg); err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}

		if bad := checkRequired(templ, msg.Data); bad != "" {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "gform.err.missing", bad), "field": bad})
			return
		}

		formID, name, surname, err := SaveFormAnswers(db, user, templ, msg.Data, msg.UniqID)
		if err != nil {
			log.Printf("GuestFormSendAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(lang, "gform.save_error")})
			return
		}

		SendFormMails(db, mailConf, user, templ, formID, name, surname)

		jsonMsg(w, http.StatusOK, map[string]interface{}{
			"id":   formID,
			"done": fmt.Sprintf("/form/%s/%s/done?id=%d&t=%s", user.URL, templ.URL, formID, doneToken(formID)),
		})
	}
}

func guestFormAPI(w http.ResponseWriter, r *http.Request, db *DB) (User, FormTemplate, bool) {
	user, err := db.UserGetByURL(mux.Vars(r)["userurl"])
	if err != nil {
		jsonMsg(w, http.StatusNotFound, map[string]string{"msg": "organization not found"})
		return User{}, FormTemplate{}, false
	}
	templ, err := db.FormTemplateGetByURL(mux.Vars(r)["formurl"], user.ID)
	if err != nil || isAddonsTemplate(templ) {
		jsonMsg(w, http.StatusNotFound, map[string]string{"msg": "form not found"})
		return User{}, FormTemplate{}, false
	}
	return user, templ, true
}

// checkRequired sprawdza po stronie serwera pola wymagane i rolę e-maila.
// Zwraca etykietę pierwszego brakującego pola.
func checkRequired(templ FormTemplate, data []FormAnswersJson) string {
	defs, err := ParseFormDef(templ.Content.String)
	if err != nil {
		return ""
	}
	got := map[string]bool{}
	for _, d := range data {
		for _, v := range d.UserData {
			if strings.TrimSpace(v) != "" {
				got[d.Name] = true
			}
		}
	}
	for _, d := range defs {
		name := d.S("name")
		if !isInputType(d.S("type")) || name == "" {
			continue
		}
		req := d.B("required") || name == ROLENAME || name == ROLESURNAME || name == ROLEEMAIL
		if req && !got[name] {
			return plainLabel(d.S("label"))
		}
	}
	return ""
}

// SaveFormAnswers zapisuje zgłoszenie i odpowiedzi. Ta sama osoba (e-mail +
// nazwisko) wysyłająca formularz drugi raz nadpisuje swoje odpowiedzi - tak
// jak działało to w starym rendererze.
func SaveFormAnswers(db *DB, user User, templ FormTemplate, data []FormAnswersJson, uniqID string) (int64, string, string, error) {
	name, surname, email := getBasicFields(data, uniqID)

	f := &Form{
		Name:           ToNS(name),
		Surname:        ToNS(surname),
		Email:          ToNS(email),
		CreatedDate:    nowUnix(),
		UserID:         user.ID,
		FormTemplateID: templ.ID,
	}
	formID, err := db.FormAdd(f)
	if err != nil {
		// już jest - ta sama osoba wysyła drugi raz
		if err := db.FormModByEmail(f); err != nil {
			return 0, "", "", fmt.Errorf("form insert and update failed: %v", err)
		}
		formID, err = db.FormGetIDByEmail(f.Email.String, templ.ID, user.ID)
		if err != nil {
			return 0, "", "", fmt.Errorf("form id by email %q: %v", f.Email.String, err)
		}
	}

	fields, err := db.FormFieldGetAllForTmpl(templ.ID)
	if err != nil {
		log.Printf("SaveFormAnswers: fields of %d: %v", templ.ID, err)
	}
	for _, d := range data {
		n := isFieldIn(d.Name, fields)
		if n < 0 {
			continue
		}
		v := ToNS("")
		if !isEmpty(d.UserData) {
			v = ToNS(strings.Join(d.UserData, ", "))
		}
		a := &FormAnswer{Value: v, FormFieldID: fields[n].ID, FormID: formID}
		if _, err := db.FormAnswerAdd(a); err != nil {
			if err := db.FormAnswerMod(a); err != nil {
				log.Printf("SaveFormAnswers: answer %q form %d: %v", d.Name, formID, err)
			}
		}
	}
	return formID, name, surname, nil
}

// SendFormMails wysyła mail do gościa i do zarządu (jeśli są ustawione).
func SendFormMails(db *DB, mailConf MailConfig, user User, templ FormTemplate, formID int64, name, surname string) {
	form, err := db.FormGet(formID, templ.ID)
	if err != nil {
		log.Printf("SendFormMails: form %d: %v", formID, err)
		return
	}

	if templ.NotificationID.Valid && templ.NotificationID.Int64 > 0 && form.Email.String != "" {
		mail, err := db.NotificationGetByID(templ.NotificationID.Int64, user.ID)
		if err != nil {
			log.Printf("SendFormMails: guest mail %d: %v", templ.NotificationID.Int64, err)
		} else {
			conf := mailConf
			conf.EmbededHTMLImgs = getEmbeddedImgs(mail.EmbeddedImgsDelimited.String, user.URL, MEDIAEMAILSUBDIR, MEDIAROOT)
			text, imgs := parseFormMailTemplate(name, surname, mail.Text, formID, user, templ, db)
			if err := prepMailConfSendMail(form.Email.String, mail.Title.String, text, user, imgs, conf); err != nil {
				log.Printf("SendFormMails: guest mail: %v", err)
			}
		}
	}

	if templ.AdminNotificationID.Valid && templ.AdminNotificationID.Int64 > 0 {
		mail, err := db.NotificationGetByID(templ.AdminNotificationID.Int64, user.ID)
		if err != nil {
			log.Printf("SendFormMails: admin mail %d: %v", templ.AdminNotificationID.Int64, err)
			return
		}
		conf := mailConf
		conf.EmbededHTMLImgs = []EmbImg{}
		text, imgs := parseFormMailTemplate(name, surname, makeSureIsHTML(mail.Text), formID, user, templ, db)
		if err := prepMailConfSendMail(chooseEmail(user.Email, user.AltEmail.String), mail.Title.String, text, user, imgs, conf); err != nil {
			log.Printf("SendFormMails: admin mail: %v", err)
		}
	}
}

type nFormDoneVars struct {
	Title    string
	FormName string
	Text     template.HTML
	Again    string
	UniqID   string
}

// GuestFormDonePage - GET /form/{userurl}/{formurl}/done?id=..&t=..
func GuestFormDonePage(db *DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := guestForm(w, r, db)
		if !ok {
			return
		}
		lang := formLang(templ)
		v := nFormDoneVars{
			Title:    T(lang, "gform.thanks"),
			FormName: templ.Name,
			Again:    fmt.Sprintf("/form/%s/%s", user.URL, templ.URL),
		}

		formID, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		token := r.URL.Query().Get("t")
		if formID > 0 && hmac.Equal([]byte(token), []byte(doneToken(formID))) {
			if form, err := db.FormGet(formID, templ.ID); err == nil {
				if !strings.Contains(templ.Content.String, ROLESURNAME) {
					v.UniqID = form.Surname.String
				}
				nsn := form.Surname.String + "_" + form.Name.String
				funcs := &FormFuncs{DB: db, User: user, Template: templ, FormID: formID, NameSurname: nsn}
				var buf bytes.Buffer
				if t, err := template.New("thankyou").Parse(templ.ThankYou.String); err != nil {
					log.Printf("GuestFormDonePage: parse: %v", err)
				} else if err := t.Execute(&buf, funcs); err != nil {
					log.Printf("GuestFormDonePage: exec: %v", err)
				}
				v.Text = template.HTML(reallyEmpty(buf.String()))
			}
		}
		guestRenderTheme(w, lang, templ.Theme, v, "tmpl/n_form_done.html")
	}
}

// nErrorVars - strona błędu dla gościa (tmpl/n_error.html). Back pokazuje
// przycisk powrotu do poprzedniej strony.
type nErrorVars struct {
	Title string
	Text  string
	Back  bool
}

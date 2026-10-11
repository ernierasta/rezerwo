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
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// doneKey podpisuje adres strony podziękowania (żeby nie dało się podejrzeć
// cudzej odpowiedzi, zgadując jej numer), link do poprawiania i ciasteczko
// zgłoszenia (form_edit.go). main ustawia go z config.toml (SetSignKey), żeby
// podpisy przeżyły restart; losowy zostaje tylko w testach.
var doneKey = func() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Printf("forms_guest: random key: %v", err)
	}
	return b
}()

func doneToken(formID int64) string {
	return formSig("form", formID)
}

type nFormPageVars struct {
	// tryb poprawiania z linku w mailu: token, zapisane odpowiedzi (JSON)
	// i informacja nad formularzem
	EditToken string
	Prefill   string
	EditNote  string

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
	email := sessionAdmin(r, cs)
	if email == "" {
		return false
	}
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

		// link z maila: poprawianie własnego zgłoszenia (form_edit.go)
		if tok := r.URL.Query().Get("edit"); tok != "" && v.Open {
			f, err := db.FormGet(parseEditToken(tok), templ.ID)
			switch {
			case err != nil:
				v.EditNote = T(lang, "gform.edit_bad")
			case !editable(f, nowUnix()):
				v.EditNote = T(lang, "gform.err.edit_expired", EDITDAYS, chooseEmail(user.Email, user.AltEmail.String))
			default:
				v.EditToken = tok
				v.Prefill = prefillJSON(db, defs, f.ID, templ.ID)
				v.EditNote = T(lang, "gform.edit_note", FormatDate(lang, f.CreatedDate), FormatDate(lang, editUntil(f)))
			}
		}
		for _, row := range v.Rows {
			for _, f := range row.Fields {
				if f.IsMoney {
					v.HasMoney = true
				}
			}
		}

		// panel informacyjny: automatyczny, z wybranych pól albo własny tekst;
		// część pól może mieć stan pokazany przy polu zamiast w panelu
		if mode := panelMode(templ); mode == PANELAUTO || mode == PANELFIELDS {
			pv := BuildPanel(db, user, templ, mode)
			v.Info = renderPanelVars(lang, pv)
			AttachFieldStats(v.Rows, pv.Fields)
		} else {
			v.Info = RenderInfoPanel(db, user, templ, mode)
		}

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
	// Confirm - gość zobaczył zmiany względem swojego wcześniejszego
	// zgłoszenia (resubmitDiff) i chce je zapisać
	Confirm bool `json:"confirm"`
	// Edit - token z linku do poprawiania (form_edit.go)
	Edit string `json:"edit"`
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

		// ukryte pola (warunki, formconds.go) idą puste i nie są wymagane
		defs, err := ParseFormDef(templ.Content.String)
		if err != nil {
			log.Printf("GuestFormSendAPI: form %d: %v", templ.ID, err)
		}
		vis := ApplyConds(defs, msg.Data)
		if bad := checkRequired(defs, vis, msg.Data); bad != "" {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "gform.err.missing", bad), "field": bad})
			return
		}

		if bad, why := checkValues(defs, vis, msg.Data, templ.MoneyAmountFieldName.String, lang); bad != "" {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": bad + ": " + why, "field": bad})
			return
		}

		// poprawka własnego zgłoszenia tylko z dowodem (ciasteczko, link z maila);
		// sam znany adres e-mail nie wystarczy do zobaczenia odpowiedzi
		prev, refuse, err := ownSubmission(db, user, templ, msg, cookieFormID(r, templ), nowUnix())
		if err != nil {
			log.Printf("GuestFormSendAPI: earlier submissions: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(lang, "gform.save_error")})
			return
		}
		if refuse != "" {
			jsonMsg(w, http.StatusConflict, map[string]string{"msg": refuseText(lang, refuse, user), "field": ROLEEMAIL})
			return
		}
		if prev != nil && !msg.Confirm {
			changes := resubmitDiff(db, defs, *prev, msg.Data)
			if len(changes) == 0 {
				jsonMsg(w, http.StatusOK, map[string]interface{}{"same": true, "done": doneURL(user, templ, prev.ID)})
				return
			}
			jsonMsg(w, http.StatusOK, map[string]interface{}{"confirm": true, "changes": changes})
			return
		}

		formID, name, surname, err := SaveFormAnswers(db, user, templ, msg.Data, msg.UniqID, prev)
		if err != nil {
			log.Printf("GuestFormSendAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(lang, "gform.save_error")})
			return
		}

		if f, err := db.FormGet(formID, templ.ID); err == nil {
			setFormCookie(w, r, user, templ, f)
		}
		SendFormMails(db, mailConf, user, templ, formID, name, surname, requestBase(r))

		jsonMsg(w, http.StatusOK, map[string]interface{}{"id": formID, "done": doneURL(user, templ, formID)})
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

// checkRequired sprawdza po stronie serwera pola wymagane i rolę e-maila;
// pola ukryte warunkiem (vis z ApplyConds) pomija. Zwraca etykietę pierwszego
// brakującego pola.
func checkRequired(defs []FormDefField, vis []bool, data []FormAnswersJson) string {
	got := map[string]bool{}
	for _, d := range data {
		for _, v := range d.UserData {
			if strings.TrimSpace(v) != "" {
				got[d.Name] = true
			}
		}
	}
	for i, d := range defs {
		name := d.S("name")
		if !isInputType(d.S("type")) || name == "" || !vis[i] {
			continue
		}
		req := d.B("required") || name == ROLENAME || name == ROLESURNAME || name == ROLEEMAIL
		if req && !got[name] {
			return plainLabel(d.S("label"))
		}
	}
	return ""
}

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// checkValues sprawdza na serwerze to, co strona sprawdza w JS (RZ.inputError):
// e-mail, liczby z min/max/krokiem; kwota do zapłaty nie może być ujemna.
// Bez tego jedna wpisana ręcznie wartość ("abc", "-500") psuła sumy na
// stronie formularza i dawała kod QR z ujemną kwotą. Zwraca etykietę pola
// i powód albo dwa puste napisy.
func checkValues(defs []FormDefField, vis []bool, data []FormAnswersJson, moneyField, lang string) (string, string) {
	vals := map[string][]string{}
	for _, d := range data {
		vals[d.Name] = append(vals[d.Name], d.UserData...)
	}
	num := func(s string) (float64, bool) {
		f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		return f, err == nil && !math.IsInf(f, 0) && !math.IsNaN(f)
	}
	fmtNum := func(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
	for i, d := range defs {
		name := d.S("name")
		if name == "" || !vis[i] {
			continue
		}
		label := plainLabel(d.S("label"))
		for _, v := range vals[name] {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if name == ROLEEMAIL || (d.S("type") == "text" && d.S("subtype") == "email") {
				if !emailRe.MatchString(v) {
					return label, T(lang, "gform.err.email")
				}
				continue
			}
			if d.S("type") != "number" {
				continue
			}
			f, ok := num(v)
			if !ok {
				return label, T(lang, "gform.err.number")
			}
			min, hasMin := num(d.S("min"))
			if !hasMin && name == moneyField {
				min, hasMin = 0, true
			}
			if hasMin && f < min {
				return label, T(lang, "gform.err.min", fmtNum(min))
			}
			if max, ok := num(d.S("max")); ok && f > max {
				return label, T(lang, "gform.err.max", fmtNum(max))
			}
			// krok jak w przeglądarce: domyślnie 1, liczony od min albo wartości domyślnej
			if d.S("step") == "any" {
				continue
			}
			step, ok := num(d.S("step"))
			if !ok || step <= 0 {
				step = 1
			}
			base := 0.0
			if hasMin {
				base = min
			} else if dv, ok := num(d.S("value")); ok {
				base = dv
			}
			n := (f - base) / step
			if math.Abs(n-math.Round(n)) > 1e-9 {
				if step == 1 {
					return label, T(lang, "gform.err.step")
				}
				low := base + math.Floor(n)*step
				return label, T(lang, "gform.err.step_n", fmtNum(step), fmtNum(low), fmtNum(low+step))
			}
		}
	}
	return "", ""
}

func doneURL(user User, templ FormTemplate, formID int64) string {
	return fmt.Sprintf("/form/%s/%s/done?id=%d&t=%s", user.URL, templ.URL, formID, doneToken(formID))
}

// ownSubmission rozstrzyga, czy to poprawka wcześniejszego zgłoszenia:
//   - link z maila (msg.Edit) wskazuje zgłoszenie wprost;
//   - adres e-mail już użyty w tym formularzu: poprawka tylko wtedy, gdy ta
//     przeglądarka wysłała to zgłoszenie (ciasteczko), inaczej odmowa;
//   - poprawiać można przez EDITDAYS dni od pierwszego wysłania.
//
// refuse: "" (można zapisać), "edit_expired" (link po terminie), "email_used"
// (adres zajęty, można poprawić linkiem z maila), "email_used_late" (adres
// zajęty, poprawki już niemożliwe).
func ownSubmission(db *DB, user User, templ FormTemplate, msg gformSendMsg, cookieID, now int64) (prev *Form, refuse string, err error) {
	_, _, email := getBasicFields(msg.Data, msg.UniqID)
	var target *Form
	if id := parseEditToken(msg.Edit); id != 0 {
		f, err := db.FormGet(id, templ.ID)
		if err != nil {
			return nil, "edit_expired", nil
		}
		if !editable(f, now) {
			return nil, "edit_expired", nil
		}
		target = &f
	}
	if strings.TrimSpace(email) == "" {
		return target, "", nil
	}
	ff, err := db.FormGetAllByEmail(email, templ.ID, user.ID)
	if err != nil {
		return nil, "", err
	}
	for i := range ff {
		switch {
		case target != nil && ff[i].ID == target.ID:
			// ten sam adres co w poprawianym zgłoszeniu
		case target != nil:
			return nil, "email_used", nil // adres innego zgłoszenia
		case ff[i].ID == cookieID && editable(ff[i], now):
			target = &ff[i]
		}
	}
	if target == nil && len(ff) > 0 {
		for _, f := range ff {
			if editable(f, now) {
				return nil, "email_used", nil
			}
		}
		return nil, "email_used_late", nil
	}
	return target, "", nil
}

// refuseText - komunikat odmowy dla gościa (ownSubmission)
func refuseText(lang, refuse string, user User) string {
	switch refuse {
	case "email_used_late":
		return T(lang, "gform.err.email_used_late", chooseEmail(user.Email, user.AltEmail.String))
	case "edit_expired":
		return T(lang, "gform.err.edit_expired", EDITDAYS, chooseEmail(user.Email, user.AltEmail.String))
	}
	return T(lang, "gform.err.email_used", EDITDAYS)
}

func hasNameFields(data []FormAnswersJson) bool {
	for _, d := range data {
		if d.Name == ROLESURNAME {
			return true
		}
	}
	return false
}

// answerChange - jedna zmieniona odpowiedź (okno potwierdzenia w n_formpage.js)
type answerChange struct {
	Label string `json:"label"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

// resubmitDiff porównuje nowe odpowiedzi z zapisanymi w prev, w kolejności
// pól formularza; wartości opcji zamienia na ich etykiety.
func resubmitDiff(db *DB, defs []FormDefField, prev Form, data []FormAnswersJson) []answerChange {
	fields, err := db.FormFieldGetAllForTmpl(prev.FormTemplateID)
	if err != nil {
		log.Printf("resubmitDiff: fields of %d: %v", prev.FormTemplateID, err)
	}
	fieldName := map[int64]string{}
	for _, f := range fields {
		fieldName[f.ID] = f.Name
	}
	old := map[string]string{}
	answers, err := db.FormAnswerGetAll(prev.ID)
	if err != nil {
		log.Printf("resubmitDiff: answers of %d: %v", prev.ID, err)
	}
	for _, a := range answers {
		if n, ok := fieldName[a.FormFieldID]; ok {
			old[n] = a.Value.String
		}
	}
	cur := map[string]string{}
	sent := map[string]bool{}
	for _, d := range data {
		cur[d.Name] = strings.Join(d.UserData, ", ")
		sent[d.Name] = true
	}

	var out []answerChange
	for _, d := range defs {
		name := d.S("name")
		if name == "" || !isInputType(d.S("type")) || !sent[name] {
			continue
		}
		o, n := strings.TrimSpace(old[name]), strings.TrimSpace(cur[name])
		if o == n {
			continue
		}
		labels := optionLabels(d)
		out = append(out, answerChange{Label: plainLabel(d.S("label")), Old: labels(o), New: labels(n)})
	}
	return out
}

// optionLabels - zamiana zapisanych wartości opcji ("bufet, nic") na etykiety
// ("Pomoc w bufecie, Nic"); wartości spoza opcji (tekst "Inne") zostają.
func optionLabels(d FormDefField) func(string) string {
	m := map[string]string{}
	vals, _ := d["values"].([]interface{})
	for _, v := range vals {
		if vm, ok := v.(map[string]interface{}); ok {
			if l := plainLabel(str(vm["label"])); l != "" {
				m[str(vm["value"])] = l
			}
		}
	}
	return func(s string) string {
		if len(m) == 0 || s == "" {
			return s
		}
		parts := strings.Split(s, ", ")
		for i, p := range parts {
			if l, ok := m[p]; ok {
				parts[i] = l
			}
		}
		return strings.Join(parts, ", ")
	}
}

// SaveFormAnswers zapisuje zgłoszenie i odpowiedzi; prev - wcześniejsze
// zgłoszenie tej samej osoby (ownSubmission), które nadpisujemy.
func SaveFormAnswers(db *DB, user User, templ FormTemplate, data []FormAnswersJson, uniqID string, prev *Form) (int64, string, string, error) {
	name, surname, email := getBasicFields(data, uniqID)

	f := &Form{
		Name:           ToNS(name),
		Surname:        ToNS(surname),
		Email:          ToNS(email),
		CreatedDate:    nowUnix(),
		UserID:         user.ID,
		FormTemplateID: templ.ID,
	}
	var formID int64
	var err error
	if prev != nil {
		formID, f.ID = prev.ID, prev.ID
		if hasNameFields(data) {
			if err := db.FormModNames(f); err != nil {
				return 0, "", "", fmt.Errorf("form %d names: %v", formID, err)
			}
		}
	} else if formID, err = db.FormAdd(f); err != nil {
		return 0, "", "", fmt.Errorf("form insert: %v", err)
	}

	fields, err := db.FormFieldGetAllForTmpl(templ.ID)
	if err != nil {
		log.Printf("SaveFormAnswers: fields of %d: %v", templ.ID, err)
	}
	fieldIDs := map[string]int64{}
	for _, f := range fields {
		if _, dup := fieldIDs[f.Name]; !dup {
			fieldIDs[f.Name] = f.ID
		}
	}
	for _, d := range data {
		id, ok := fieldIDs[d.Name]
		if !ok {
			continue
		}
		a := &FormAnswer{Value: ToNS(strings.Join(d.UserData, ", ")), FormFieldID: id, FormID: formID}
		if _, err := db.FormAnswerAdd(a); err != nil {
			if err := db.FormAnswerMod(a); err != nil {
				log.Printf("SaveFormAnswers: answer %q form %d: %v", d.Name, formID, err)
			}
		}
	}
	return formID, name, surname, nil
}

// SendFormMails wysyła mail do gościa i do zarządu (jeśli są ustawione).
func SendFormMails(db *DB, mailConf MailConfig, user User, templ FormTemplate, formID int64, name, surname, base string) {
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
			text, imgs := parseFormMailTemplate(name, surname, mail.Text, formID, user, templ, db)
			text = addEditLink(text, formLang(templ), editURL(base, user, templ, formID), editUntil(form))
			if err := mailConf.SendAsOrg(user, form.Email.String, mail.Title.String, text, mail.Files(user), append(mail.Images(user), imgs...)); err != nil {
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
		text, imgs := parseFormMailTemplate(name, surname, makeSureIsHTML(mail.Text), formID, user, templ, db)
		if err := mailConf.SendAsOrg(user, chooseEmail(user.Email, user.AltEmail.String), mail.Title.String, text, mail.Files(user), append(mail.Images(user), imgs...)); err != nil {
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
				if v.Text == "" {
					v.Text = formDefaultPayText(db, user, templ, form, funcs, lang)
				}
			}
		}
		guestRenderTheme(w, lang, templ.Theme, v, "tmpl/n_form_done.html")
	}
}

// formDefaultPayText - formularz bez własnego podziękowania, ale z kontem
// i kwotą do zapłaty: gość musi się dowiedzieć, ile i gdzie zapłacić.
// Te same dane do przelewu co {{.PayInfo}} imprezy, plus kod QR.
func formDefaultPayText(db *DB, user User, templ FormTemplate, form Form, funcs *FormFuncs, lang string) template.HTML {
	if !templ.BankAccountID.Valid || templ.MoneyAmountFieldName.String == "" {
		return ""
	}
	ba, err := db.BankAccountGetByID(templ.BankAccountID.Int64, user.ID)
	if err != nil || ba.IBAN == "" {
		return ""
	}
	amount := funcs.MTotal(templ.MoneyAmountFieldName.String)
	if n, err := strconv.ParseInt(strings.TrimSpace(amount), 10, 64); err != nil || n <= 0 {
		return ""
	}
	of := &OrderFuncs{DB: db, User: user, Lang: lang, TotalPrice: amount,
		Name: form.Name.String, Surname: form.Surname.String, payPrefix: templ.PayMessage}
	return template.HTML("<p>"+template.HTMLEscapeString(T(lang, "gform.thanks_text"))+"</p>"+
		"<p>"+template.HTMLEscapeString(T(lang, "order.done_pay"))+"</p>") +
		of.PayInfo(ba.Name) + "<p>" + funcs.QRPay(ba.Name) + "</p>"
}

// nErrorVars - strona błędu dla gościa (tmpl/n_error.html). Back pokazuje
// przycisk powrotu do poprzedniej strony.
type nErrorVars struct {
	Title string
	Text  string
	Back  bool
}

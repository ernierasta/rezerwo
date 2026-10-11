package main

// forms_ui.go: nowy panel - strona formularza (harmonijka) i tabela odpowiedzi.
//
// Definicje pól edytuje designer pól (forms_fields.go). Tutaj są ustawienia
// formularza, teksty strony, maile, konto i odpowiedzi.

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/microcosm-cc/bluemonday"
)

// --- dostęp ----------------------------------------------------------------

func adminFormTemplate(w http.ResponseWriter, r *http.Request, db *DB, cs *sessions.CookieStore, htmlErrors bool) (User, FormTemplate, bool) {
	fail := func(status int, msg string) {
		if htmlErrors {
			http.NotFound(w, r)
			return
		}
		jsonMsg(w, status, map[string]string{"msg": msg})
	}

	user, ok := adminUser(w, r, db, cs)
	if !ok {
		return User{}, FormTemplate{}, false
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		fail(http.StatusBadRequest, "bad form id")
		return User{}, FormTemplate{}, false
	}
	templ, err := db.FormTemplateGetByID(id, user.ID)
	if err != nil {
		fail(http.StatusNotFound, "form not found")
		return User{}, FormTemplate{}, false
	}
	return user, templ, true
}

// --- strona formularza -----------------------------------------------------

type nFormVars struct {
	NPage
	ID        int64
	Name      string
	URL       string
	PublicURL string

	Banner    nBannerPick
	HowTo     template.HTML
	InfoPanel template.HTML
	ThankYou  template.HTML

	MoneyField string
	Fields     []FormField

	// nowe ustawienia (tools/migration-forms.sql) i designer pól
	Language     string
	Status       string
	CloseDateVal string
	GuestURL     string
	FieldList    []nFieldSummary
	MoreFields   int
	AnswersCount int64

	// Theme - temat strony gościa, Themes - lista do wyboru (themes.go)
	Theme  string
	Themes []GuestTheme

	// panel informacyjny (form_stats.go)
	PanelMode   string
	PanelFields template.JS // pola do menu pastylek w trybie "własny tekst"

	Notifications       []Notification
	NotificationID      int64
	AdminNotificationID int64
	BankAccounts        []BankAccount
	BankAccountID       int64
	PayMessage          string
	Events              []Event
	EventID             int64

	SumBasic, SumFields, SumPage        string
	SumMailGuest, SumMailAdmin, SumBank string
	SumEvent, SumAnswers                string
	MissMailGuest, MissMailAdmin        bool
	MissBank                            bool
}

// FormPage - /admin/forms/{id}
func FormPage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := adminFormTemplate(w, r, db, cs, true)
		if !ok {
			return
		}
		lang := PanelLang(r)
		vars := buildFormVars(db, user, templ, lang)
		nRender(w, lang, vars, "tmpl/n_form.html")
	}
}

func buildFormVars(db *DB, user User, templ FormTemplate, lang string) nFormVars {
	fields, err := db.FormFieldGetAllForTmpl(templ.ID)
	if err != nil {
		log.Printf("buildFormVars: fields: %v", err)
	}
	fields = filterFormFields(fields) // bez nagłówków, akapitów i przycisków
	for i := range fields {
		fields[i].Display = plainLabel(fields[i].Display)
	}

	notifs, err := db.NotificationGetAllForUser(user.ID)
	if err != nil {
		log.Printf("buildFormVars: notifications: %v", err)
	}
	bankAccs, err := db.BankAccountGetAll(user.ID)
	if err != nil {
		log.Printf("buildFormVars: bank accounts: %v", err)
	}
	events, err := db.EventGetAllByUserID(user.ID)
	if err != nil {
		log.Printf("buildFormVars: events: %v", err)
	}
	answers, err := db.FormGetAmmount(user.ID, templ.ID)
	if err != nil {
		log.Printf("buildFormVars: answers count: %v", err)
	}

	v := nFormVars{
		NPage: NPage{
			Title:   templ.Name,
			Crumb:   templ.Name,
			OrgName: orgName(user),
			UpURL:   "/admin",
			UpLabel: T(lang, "nav.home"),
		},
		ID:                  templ.ID,
		Name:                templ.Name,
		URL:                 templ.URL,
		PublicURL:           fmt.Sprintf("/form/%s/%s", user.URL, templ.URL),
		Banner:              bannerPick(T(lang, "field.main_banner"), "banner", templ.Banner.String, user),
		HowTo:               template.HTML(templ.HowTo.String),
		InfoPanel:           template.HTML(templ.InfoPanel.String),
		ThankYou:            template.HTML(templ.ThankYou.String),
		MoneyField:          templ.MoneyAmountFieldName.String,
		Fields:              fields,
		Notifications:       notifs,
		NotificationID:      templ.NotificationID.Int64,
		AdminNotificationID: templ.AdminNotificationID.Int64,
		BankAccounts:        bankAccs,
		BankAccountID:       templ.BankAccountID.Int64,
		PayMessage:          templ.PayMessage,
		Events:              events,
		EventID:             templ.EventID.Int64,
		Language:            formLang(templ),
		Status:              templ.Status,
		CloseDateVal:        dateInputValue(templ.CloseDate.Int64),
		GuestURL:            fmt.Sprintf("/form/%s/%s", user.URL, templ.URL),
		AnswersCount:        answers,
		Theme:               ThemeKey(templ.Theme),
		Themes:              guestThemes,
	}
	if v.Status == "" {
		v.Status = "open"
	}

	v.PanelMode = panelMode(templ)
	v.PanelFields = panelFieldsJSON(templ)

	v.FieldList = formFieldSummaries(templ, lang)
	if len(v.FieldList) > 12 {
		v.MoreFields = len(v.FieldList) - 12
		v.FieldList = v.FieldList[:12]
	}

	v.SumBasic = formStatusLabel(templ, lang) + " · " + languageName(lang, v.Language)

	v.SumFields = T(lang, "form.fields_count", len(fields))
	if templ.MoneyAmountFieldName.String != "" {
		v.SumFields += ", " + T(lang, "form.money_from", fieldDisplay(fields, templ.MoneyAmountFieldName.String))
	}

	texts := 0
	for _, s := range []string{templ.HowTo.String, templ.InfoPanel.String, templ.ThankYou.String} {
		if strings.TrimSpace(s) != "" {
			texts++
		}
	}
	v.SumPage = TN(lang, "count.text", int64(texts)) + ", " + T(lang, "panel.mode_sum."+panelMode(templ))
	if templ.Banner.String != "" {
		v.SumPage = T(lang, "field.main_banner") + ", " + v.SumPage
	}
	v.SumPage = T(lang, "theme.label") + ": " + ThemeName(lang, templ.Theme) + ", " + v.SumPage

	v.SumMailGuest, v.MissMailGuest = notificationName(db, user, templ.NotificationID.Int64, lang)
	v.SumMailAdmin, v.MissMailAdmin = notificationName(db, user, templ.AdminNotificationID.Int64, lang)

	if templ.BankAccountID.Valid && templ.BankAccountID.Int64 != 0 {
		if ba, err := db.BankAccountGetByID(templ.BankAccountID.Int64, user.ID); err == nil {
			v.SumBank = ba.Name + ", " + ba.IBAN
			if templ.PayMessage != "" {
				v.SumBank += " · " + T(lang, "pay.message") + ": " + templ.PayMessage
			}
		} else {
			v.SumBank, v.MissBank = T(lang, "common.not_set"), true
		}
	} else {
		v.SumBank, v.MissBank = T(lang, "common.not_set"), true
	}

	v.SumEvent = T(lang, "common.none")
	if templ.EventID.Valid && templ.EventID.Int64 != 0 {
		if ev, err := db.EventGetByID(templ.EventID.Int64); err == nil && ev.UserID == user.ID {
			v.SumEvent = ev.Name
		}
	}

	v.SumAnswers = TN(lang, "count.answer", answers)
	return v
}

// panelFieldsJSON - pola formularza dla menu "Wstaw zestawienie" w edytorze
// panelu (js/n_form.js): nazwa, etykieta, rodzaj i opcje.
func panelFieldsJSON(templ FormTemplate) template.JS {
	type opt struct {
		Name    string   `json:"name"`
		Label   string   `json:"label"`
		Type    string   `json:"type"`
		Options []string `json:"options,omitempty"`
	}
	out := []opt{}
	defs, _ := ParseFormDef(templ.Content.String)
	for _, d := range defs {
		name := d.S("name")
		if !isInputType(d.S("type")) || name == "" || name == ROLENAME || name == ROLESURNAME || name == ROLEEMAIL {
			continue
		}
		o := opt{Name: name, Label: plainLabel(d.S("label")), Type: d.S("type")}
		if vals, ok := d["values"].([]interface{}); ok {
			for _, v := range vals {
				if vm, ok := v.(map[string]interface{}); ok {
					o.Options = append(o.Options, plainLabel(str(vm["label"])))
				}
			}
		}
		out = append(out, o)
	}
	b, _ := json.Marshal(out)
	return template.JS(b)
}

// formStatusLabel opisuje, czy formularz przyjmuje odpowiedzi.
func formStatusLabel(t FormTemplate, lang string) string {
	if !t.IsOpen(nowUnix()) {
		return T(lang, "formpage.closed_since")
	}
	if t.CloseDate.Valid && t.CloseDate.Int64 > 0 {
		return T(lang, "formpage.open_until", FormatDate(lang, t.CloseDate.Int64))
	}
	return T(lang, "formpage.open")
}

// ADDONSKIND - rodzaj szablonu formularza, który jest definicją dodatków
// do zamówienia (przypiętą do imprezy), a nie zwykłym formularzem.
const ADDONSKIND = "order_addons"

// isAddonsTemplate mówi, czy szablon to definicja dodatków. Takie szablony nie
// pokazują się na listach formularzy - edytuje się je w sekcji imprezy.
func isAddonsTemplate(t FormTemplate) bool {
	return t.Kind.Valid && t.Kind.String == ADDONSKIND
}

// EventAddonsCreateAPI - POST /api/events/{id}/addons
// Tworzy definicję dodatków dla imprezy (szablon formularza rodzaju
// order_addons) i przypina ją do imprezy. Pola dodaje się potem tym samym
// designerem co w formularzach.
func EventAddonsCreateAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, event, ok := adminEvent(w, r, db, cs)
		if !ok {
			return
		}
		if event.AddonsFormTemplateID.Valid && event.AddonsFormTemplateID.Int64 != 0 {
			jsonMsg(w, http.StatusOK, map[string]interface{}{
				"msg": "exists", "id": event.AddonsFormTemplateID.Int64,
			})
			return
		}

		// adres nie jest używany (dodatki nie mają własnej strony), ale musi
		// być unikatowy w obrębie organizacji
		url := fmt.Sprintf("dodatki-%d", event.ID)
		id, err := db.FormTemplateAdd(&FormTemplate{
			Name:        T(PanelLang(r), "addons.template_name", event.Name),
			URL:         url,
			CreatedDate: time.Now().Unix(),
			UserID:      user.ID,
			EventID:     ToNI(event.ID),
		})
		if err != nil {
			log.Printf("EventAddonsCreateAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
			return
		}
		if err := db.FormTemplateSetKind(id, user.ID, ADDONSKIND); err != nil {
			log.Printf("EventAddonsCreateAPI: set kind: %v", err)
		}
		if err := db.EventSetAddonsTemplate(event.ID, user.ID, ToNI(id)); err != nil {
			log.Printf("EventAddonsCreateAPI: link to event: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "created", "id": id})
	}
}

// nAddonField to jedno pole dodatków w podsumowaniu sekcji.
type nAddonField struct {
	Label string
	Kind  string // opis rodzaju: wybór, ilość × cena, tekst
}

// addonFields zwraca pola definicji dodatków imprezy.
func addonFields(db *DB, event Event) []nAddonField {
	if !event.AddonsFormTemplateID.Valid || event.AddonsFormTemplateID.Int64 == 0 {
		return nil
	}
	ff, err := db.FormFieldGetAllForTmpl(event.AddonsFormTemplateID.Int64)
	if err != nil {
		log.Printf("addonFields: %v", err)
		return nil
	}
	ff = filterFormFields(ff)

	out := make([]nAddonField, 0, len(ff))
	for _, f := range ff {
		out = append(out, nAddonField{Label: plainLabel(f.Display), Kind: f.Type})
	}
	return out
}

func fieldDisplay(ff []FormField, name string) string {
	for _, f := range ff {
		if f.Name == name {
			return plainLabel(f.Display)
		}
	}
	return name
}

// FormSectionSave - POST /api/forms/{id}/section
func FormSectionSave(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := adminFormTemplate(w, r, db, cs, false)
		if !ok {
			return
		}
		lang := PanelLang(r)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		var msg nSectionMsg
		if err := json.Unmarshal(body, &msg); err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}

		val := func(key string) string {
			if s, ok := msg.Values[key].(string); ok {
				return strings.TrimSpace(s)
			}
			return ""
		}
		html := func(key string) string {
			if s, ok := msg.Values[key].(string); ok {
				return removeTrailingEmptyParagraphs(s)
			}
			return ""
		}
		id64 := func(key string) int64 {
			n, _ := strconv.ParseInt(val(key), 10, 64)
			return n
		}

		switch msg.Section {
		case "basic":
			if name := val("name"); name != "" {
				templ.Name = name
			}
			if url := val("url"); url != "" && url != templ.URL {
				if !formURLRe.MatchString(url) {
					jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "newform.bad_url")})
					return
				}
				if db.FormTemplateURLTaken(url, user.ID, templ.ID) {
					jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "newform.url_taken", url)})
					return
				}
				templ.URL = url
			}
			status := val("status")
			if status != "closed" {
				status = "open"
			}
			closeDate := ToNI(0)
			if d := val("close_date"); d != "" {
				day, err := time.ParseInLocation("2006-01-02", d, time.Local)
				if err != nil {
					jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
					return
				}
				// formularz przyjmuje odpowiedzi do końca wybranego dnia
				closeDate = ToNI(day.Add(24*time.Hour - time.Second).Unix())
			}
			if err := db.FormTemplateSetAccess(templ.ID, user.ID, KnownLang(val("language")), status, closeDate); err != nil {
				jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
				return
			}
		case "page":
			templ.Banner = ToNS(cleanBanner(val("banner")))
			templ.HowTo = ToNS(html("howto"))
			templ.InfoPanel = ToNS(html("infopanel"))
			mode := val("infopanel_mode")
			switch mode {
			case PANELNONE, PANELAUTO, PANELFIELDS, PANELTEXT:
				if err := db.FormTemplateSetPanelMode(templ.ID, user.ID, mode); err != nil {
					jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
					return
				}
			}
			templ.ThankYou = ToNS(html("thankyou"))
			if err := db.FormTemplateSetTheme(templ.ID, user.ID, ThemeKey(val("theme"))); err != nil {
				jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
				return
			}
		case "mail_guest":
			templ.NotificationID = ToNI(id64("notifications_id_fk"))
		case "mail_admin":
			templ.AdminNotificationID = ToNI(id64("admin_notifications_id_fk"))
			if err := db.FormTemplateSetAdminNotification(templ.ID, user.ID, templ.AdminNotificationID); err != nil {
				jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
				return
			}
		case "bank":
			templ.BankAccountID = ToNI(id64("bankaccounts_id_fk"))
			templ.MoneyAmountFieldName = ToNS(val("moneyfield"))
			templ.PayMessage = strings.TrimSpace(val("pay_message"))
			if err := db.FormTemplateSetPayMessage(templ.ID, user.ID, templ.PayMessage); err != nil {
				jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
				return
			}
		case "event":
			templ.EventID = ToNI(id64("events_id_fk"))
			if err := db.FormTemplateSetEvent(templ.ID, user.ID, templ.EventID); err != nil {
				jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
				return
			}
		default:
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "unknown section " + msg.Section})
			return
		}

		// pola wspólne zapisujemy jednym UPDATE (content zostaje nietknięty)
		if err := db.FormTemplateModByIDUser(&templ); err != nil {
			log.Printf("FormSectionSave: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(lang, "common.save_error") + ": " + err.Error()})
			return
		}

		templ, err = db.FormTemplateGetByID(templ.ID, user.ID)
		if err != nil {
			jsonMsg(w, http.StatusOK, nSectionResp{Msg: "saved"})
			return
		}
		v := buildFormVars(db, user, templ, lang)
		resp := nSectionResp{Msg: "saved", Title: templ.Name, Subtitle: v.PublicURL}
		switch msg.Section {
		case "basic":
			resp.Summary = v.SumBasic
		case "page":
			resp.Summary = v.SumPage
		case "mail_guest":
			resp.Summary, resp.Missing = v.SumMailGuest, v.MissMailGuest
		case "mail_admin":
			resp.Summary, resp.Missing = v.SumMailAdmin, v.MissMailAdmin
		case "bank":
			resp.Summary, resp.Missing = v.SumBank, v.MissBank
		case "event":
			resp.Summary = v.SumEvent
		}
		jsonMsg(w, http.StatusOK, resp)
	}
}

// --- odpowiedzi ------------------------------------------------------------

type nAnswerRow struct {
	ID        int64             `json:"id"`
	Name      string            `json:"name"`
	Surname   string            `json:"surname"`
	Email     string            `json:"email"`
	Status    string            `json:"status"`
	Created   int64             `json:"created"`
	LastMail  int64             `json:"last_mail"`
	MailCount int64             `json:"mail_count"`
	Values    map[string]string `json:"values"`
	// widoczność pól przy warunkach, patrz form_report.go
	NA    []string            `json:"na,omitempty"`
	Out   []string            `json:"out,omitempty"`
	Picks map[string][]string `json:"picks,omitempty"`
}

type nAnswersResp struct {
	FormID        int64            `json:"form_id"`
	FormName      string           `json:"form_name"`
	MoneyField    string           `json:"money_field"`
	Columns       []nAnswerCol     `json:"columns"`
	Sections      []nAnswerSection `json:"sections"`
	Rows          []nAnswerRow     `json:"rows"`
	Notifications []nResNotifOpt   `json:"notifications"`
}

// plainLabel czyści etykietę pola z HTML. W bazie zdarzają się etykiety
// z <b> czy <i> (formBuilder pozwala je wkleić), a w panelu chcemy czysty tekst.
// Same dane w bazie zostają nietknięte - czyszczenie treści to osobny krok
// migracji (rozdz. 7.2 HARMONIJKA.md).
func plainLabel(s string) string {
	return strings.TrimSpace(html.UnescapeString(bluemonday.StripTagsPolicy().Sanitize(s)))
}

func answerRows(db *DB, user User, templ FormTemplate, sch answerSchema) []nAnswerRow {
	forms, err := db.FormGetAll(user.ID, templ.ID)
	if err != nil {
		log.Printf("answerRows: %v", err)
	}

	rows := make([]nAnswerRow, 0, len(forms))
	for _, f := range forms {
		last, err := db.FormNotificationLogGetLast(f.ID)
		if err != nil {
			last = 0
		}
		amount, err := db.FormNotificationLogGetAmount(f.ID)
		if err != nil {
			amount = 0
		}

		row := nAnswerRow{
			ID:        f.ID,
			Name:      f.Name.String,
			Surname:   f.Surname.String,
			Email:     f.Email.String,
			Status:    f.Status.String,
			Created:   f.CreatedDate,
			LastMail:  last,
			MailCount: amount,
			Values:    map[string]string{},
		}

		ans, err := db.FormAnswerGetAll(f.ID)
		if err != nil {
			log.Printf("answerRows: answers of form %d: %v", f.ID, err)
		}
		for _, a := range ans {
			if key, ok := sch.ByID[a.FormFieldID]; ok {
				row.Values[key] = a.Value.String
			}
		}
		row.NA, row.Out, row.Picks = sch.rowVisibility(row.Values)
		rows = append(rows, row)
	}
	return rows
}

// AnswersPage - /admin/forms/{id}/answers
func AnswersPage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := adminFormTemplate(w, r, db, cs, true)
		if !ok {
			return
		}
		lang := PanelLang(r)
		vars := struct {
			NPage
			ID   int64
			Name string
		}{
			NPage: NPage{
				Title:   templ.Name + " — " + T(lang, "form.answers"),
				Crumb:   T(lang, "form.answers"),
				OrgName: orgName(user),
				UpURL:   fmt.Sprintf("/admin/forms/%d", templ.ID),
				UpLabel: templ.Name,
			},
			ID:   templ.ID,
			Name: templ.Name,
		}
		nRender(w, lang, vars, "tmpl/n_answers.html")
	}
}

// AnswersListAPI - GET /api/forms/{id}/answers
func AnswersListAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := adminFormTemplate(w, r, db, cs, false)
		if !ok {
			return
		}
		sch := loadAnswerSchema(db, templ, PanelLang(r))
		notifs, err := db.NotificationGetAllForUser(user.ID)
		if err != nil {
			log.Printf("AnswersListAPI: notifications: %v", err)
		}

		resp := nAnswersResp{
			FormID:     templ.ID,
			FormName:   templ.Name,
			MoneyField: templ.MoneyAmountFieldName.String,
			Columns:    sch.Cols,
			Sections:   sch.Sections,
			Rows:       answerRows(db, user, templ, sch),
		}
		for _, n := range notifs {
			resp.Notifications = append(resp.Notifications, nResNotifOpt{ID: n.ID, Name: n.Name})
		}
		jsonMsg(w, http.StatusOK, resp)
	}
}

func answersRefreshed(db *DB, user User, templ FormTemplate, lang string) []nAnswerRow {
	return answerRows(db, user, templ, loadAnswerSchema(db, templ, lang))
}

// AnswersStatusAPI - POST /api/forms/{id}/answers/status
func AnswersStatusAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := adminFormTemplate(w, r, db, cs, false)
		if !ok {
			return
		}
		m, ok := decodeIDs(w, r)
		if !ok {
			return
		}
		switch m.Status {
		case "payed", "ordered", "":
		default:
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "unknown status " + m.Status})
			return
		}

		var failed []int64
		for _, id := range m.IDs {
			if err := db.FormSetStatus(id, templ.ID, m.Status); err != nil {
				log.Printf("AnswersStatusAPI: %d: %v", id, err)
				failed = append(failed, id)
			}
		}
		if len(failed) > 0 {
			jsonMsg(w, http.StatusTeapot, map[string]interface{}{"msg": "some answers not updated", "failed": failed})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "ok", "rows": answersRefreshed(db, user, templ, PanelLang(r))})
	}
}

// AnswersDeleteAPI - DELETE /api/forms/{id}/answers
func AnswersDeleteAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := adminFormTemplate(w, r, db, cs, false)
		if !ok {
			return
		}
		m, ok := decodeIDs(w, r)
		if !ok {
			return
		}

		var failed []int64
		for _, id := range m.IDs {
			if err := db.FormAnswerDel(id, templ.ID); err != nil {
				log.Printf("AnswersDeleteAPI: answers of %d: %v", id, err)
			}
			if err := db.FormDel(id, templ.ID); err != nil {
				log.Printf("AnswersDeleteAPI: form %d: %v", id, err)
				failed = append(failed, id)
			}
		}
		if len(failed) > 0 {
			jsonMsg(w, http.StatusTeapot, map[string]interface{}{"msg": "some answers not deleted", "failed": failed})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "ok", "rows": answersRefreshed(db, user, templ, PanelLang(r))})
	}
}

// AnswersMailAPI - POST /api/forms/{id}/answers/mail
func AnswersMailAPI(db *DB, mailConf MailConfig, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := adminFormTemplate(w, r, db, cs, false)
		if !ok {
			return
		}
		m, ok := decodeIDs(w, r)
		if !ok {
			return
		}
		if m.NotificationID == 0 {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "no mail template selected"})
			return
		}
		mail, err := db.NotificationGetByID(m.NotificationID, user.ID)
		if err != nil {
			jsonMsg(w, http.StatusNotFound, map[string]string{"msg": "mail template not found"})
			return
		}

		sent := 0
		var failed []int64
		for _, id := range m.IDs {
			form, err := db.FormGet(id, templ.ID)
			if err != nil {
				log.Printf("AnswersMailAPI: form %d: %v", id, err)
				failed = append(failed, id)
				continue
			}
			if err := sendFormAnswerMail(db, mailConf, user, templ, form, mail); err != nil {
				log.Printf("AnswersMailAPI: sending to %s: %v", form.Email.String, err)
				failed = append(failed, id)
				continue
			}
			sent++
		}

		status := http.StatusOK
		if len(failed) > 0 {
			status = http.StatusTeapot
		}
		jsonMsg(w, status, map[string]interface{}{
			"msg":    fmt.Sprintf("sent %d of %d", sent, len(m.IDs)),
			"sent":   sent,
			"failed": failed,
			"rows":   answersRefreshed(db, user, templ, PanelLang(r)),
		})
	}
}

// sendFormAnswerMail wysyła jeden mail do autora odpowiedzi i zapisuje wysyłkę
// w logu (jak /api/formanssendmail, tylko z obsługą błędów).
func sendFormAnswerMail(db *DB, mailConf MailConfig, user User, templ FormTemplate, form Form, mail Notification) error {
	if form.Email.String == "" {
		return fmt.Errorf("answer %d has no e-mail", form.ID)
	}

	parsedText, embImgs := parseFormMailTemplate(form.Name.String, form.Surname.String,
		makeSureIsHTML(mail.Text), form.ID, user, templ, db)
	if err := mailConf.SendAsOrg(user, form.Email.String, mail.Title.String, parsedText, mail.Files(user), append(mail.Images(user), embImgs...)); err != nil {
		return err
	}

	if _, err := db.FormNotificationLogAdd(&FormNotificationLog{
		Date:           time.Now().Unix(),
		NotificationID: mail.ID,
		FormID:         form.ID,
	}); err != nil {
		log.Printf("sendFormAnswerMail: log: %v", err)
	}
	return nil
}

// AnswersCSVAPI - GET /api/forms/{id}/answers.csv
// Parametry (eksport widoku ze strony odpowiedzi):
//
//	ids=1,2,3  tylko te zgłoszenia
//	cols=a,b   tylko te pola (kolejność z formularza)
//	na=1       pola, których gość nie widział, jako "n/d" zamiast pustych
//
// Pole z opcjami zależnymi stoi zaraz za swoim rodzicem, żeby było wiadomo,
// do czego odnosi się wybór.
func AnswersCSVAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := adminFormTemplate(w, r, db, cs, false)
		if !ok {
			return
		}
		lang := PanelLang(r)
		q := r.URL.Query()
		sch := loadAnswerSchema(db, templ, lang)
		rows := answerRows(db, user, templ, sch)

		if ids := splitParam(q.Get("ids")); ids != nil {
			keep := rows[:0]
			for _, row := range rows {
				if ids[strconv.FormatInt(row.ID, 10)] {
					keep = append(keep, row)
				}
			}
			rows = keep
		}
		cols := csvColumns(sch.Cols, splitParam(q.Get("cols")))
		na := ""
		if q.Get("na") == "1" {
			na = T(lang, "answers.na")
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q",
			strings.ReplaceAll(templ.Name, "\"", "")+".csv"))

		cw := csv.NewWriter(w)
		defer cw.Flush()

		head := []string{"id", T(lang, "field.name"), "e-mail", T(lang, "res.status"), T(lang, "form.created")}
		for _, c := range cols {
			head = append(head, c.Label)
		}
		cw.Write(head)

		for _, row := range rows {
			rec := []string{
				strconv.FormatInt(row.ID, 10),
				strings.TrimSpace(row.Name + " " + row.Surname),
				row.Email,
				row.Status,
				FormatDate(lang, row.Created),
			}
			hidden := map[string]bool{}
			for _, k := range row.NA {
				hidden[k] = true
			}
			for _, c := range cols {
				v := row.Values[c.Key]
				if hidden[c.Key] && strings.TrimSpace(v) == "" {
					v = na
				}
				rec = append(rec, v)
			}
			cw.Write(rec)
		}
	}
}

// splitParam zamienia "a,b,c" na zbiór (nil, gdy parametru nie ma).
func splitParam(s string) map[string]bool {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	out := map[string]bool{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out[p] = true
		}
	}
	return out
}

// csvColumns wybiera kolumny (only == nil: wszystkie) i przestawia pola
// zależne zaraz za ich rodzica (a kilka zależnych w kolejności formularza).
func csvColumns(all []nAnswerCol, only map[string]bool) []nAnswerCol {
	var cols []nAnswerCol
	for _, c := range all {
		if only == nil || only[c.Key] {
			cols = append(cols, c)
		}
	}
	present := map[string]bool{}
	for _, c := range cols {
		present[c.Key] = true
	}
	children := map[string][]nAnswerCol{}
	for _, c := range cols {
		if c.Parent != "" && present[c.Parent] {
			children[c.Parent] = append(children[c.Parent], c)
		}
	}
	out := make([]nAnswerCol, 0, len(cols))
	var add func(c nAnswerCol)
	add = func(c nAnswerCol) {
		out = append(out, c)
		for _, ch := range children[c.Key] {
			add(ch)
		}
	}
	for _, c := range cols {
		if c.Parent != "" && present[c.Parent] {
			continue
		}
		add(c)
	}
	return out
}

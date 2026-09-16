package main

// forms_ui.go: nowy panel - strona formularza (harmonijka) i tabela odpowiedzi.
//
// Definicje pól dalej edytuje formBuilder w starym edytorze (/admin/formeditor),
// zgodnie z etapem A z rozdz. 7.2 HARMONIJKA.md - format JSON się nie zmienia.
// Tutaj są tylko ustawienia formularza, teksty strony, maile, konto i odpowiedzi.

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

	Banner    string
	HowTo     template.HTML
	InfoPanel template.HTML
	ThankYou  template.HTML

	MoneyField string
	Fields     []FormField

	Notifications       []Notification
	NotificationID      int64
	AdminNotificationID int64
	BankAccounts        []BankAccount
	BankAccountID       int64
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
		},
		ID:                  templ.ID,
		Name:                templ.Name,
		URL:                 templ.URL,
		PublicURL:           fmt.Sprintf("/form/%s/%s", user.URL, templ.URL),
		Banner:              templ.Banner.String,
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
		Events:              events,
		EventID:             templ.EventID.Int64,
	}

	v.SumBasic = templ.Name + ", " + templ.URL

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
	v.SumPage = TN(lang, "count.text", int64(texts))
	if templ.Banner.String != "" {
		v.SumPage = T(lang, "field.main_banner") + ", " + v.SumPage
	}

	v.SumMailGuest, v.MissMailGuest = notificationName(db, user, templ.NotificationID.Int64, lang)
	v.SumMailAdmin, v.MissMailAdmin = notificationName(db, user, templ.AdminNotificationID.Int64, lang)

	if templ.BankAccountID.Valid && templ.BankAccountID.Int64 != 0 {
		if ba, err := db.BankAccountGetByID(templ.BankAccountID.Int64, user.ID); err == nil {
			v.SumBank = ba.Name + ", " + ba.IBAN
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
			if url := val("url"); url != "" {
				templ.URL = url
			}
		case "page":
			templ.Banner = ToNS(val("banner"))
			templ.HowTo = ToNS(html("howto"))
			templ.InfoPanel = ToNS(html("infopanel"))
			templ.ThankYou = ToNS(html("thankyou"))
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

type nAnswerCol struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

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
}

type nAnswersResp struct {
	FormID        int64          `json:"form_id"`
	FormName      string         `json:"form_name"`
	MoneyField    string         `json:"money_field"`
	Columns       []nAnswerCol   `json:"columns"`
	Rows          []nAnswerRow   `json:"rows"`
	Notifications []nResNotifOpt `json:"notifications"`
}

func answerColumns(db *DB, templ FormTemplate) ([]nAnswerCol, map[int64]string) {
	ff, err := db.FormFieldGetAllForTmpl(templ.ID)
	if err != nil {
		log.Printf("answerColumns: %v", err)
	}
	ff = filterFormFields(ff)

	cols := make([]nAnswerCol, 0, len(ff))
	byID := map[int64]string{}
	for _, f := range ff {
		cols = append(cols, nAnswerCol{Key: f.Name, Label: plainLabel(f.Display)})
		byID[f.ID] = f.Name
	}
	return cols, byID
}

// plainLabel czyści etykietę pola z HTML. W bazie zdarzają się etykiety
// z <b> czy <i> (formBuilder pozwala je wkleić), a w panelu chcemy czysty tekst.
// Same dane w bazie zostają nietknięte - czyszczenie treści to osobny krok
// migracji (rozdz. 7.2 HARMONIJKA.md).
func plainLabel(s string) string {
	return strings.TrimSpace(html.UnescapeString(bluemonday.StripTagsPolicy().Sanitize(s)))
}

func answerRows(db *DB, user User, templ FormTemplate, byID map[int64]string) []nAnswerRow {
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
			if key, ok := byID[a.FormFieldID]; ok {
				row.Values[key] = a.Value.String
			}
		}
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
				Crumb:   templ.Name,
				OrgName: orgName(user),
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
		cols, byID := answerColumns(db, templ)
		notifs, err := db.NotificationGetAllForUser(user.ID)
		if err != nil {
			log.Printf("AnswersListAPI: notifications: %v", err)
		}

		resp := nAnswersResp{
			FormID:     templ.ID,
			FormName:   templ.Name,
			MoneyField: templ.MoneyAmountFieldName.String,
			Columns:    cols,
			Rows:       answerRows(db, user, templ, byID),
		}
		for _, n := range notifs {
			resp.Notifications = append(resp.Notifications, nResNotifOpt{ID: n.ID, Name: n.Name})
		}
		jsonMsg(w, http.StatusOK, resp)
	}
}

func answersRefreshed(db *DB, user User, templ FormTemplate) []nAnswerRow {
	_, byID := answerColumns(db, templ)
	return answerRows(db, user, templ, byID)
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
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "ok", "rows": answersRefreshed(db, user, templ)})
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
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "ok", "rows": answersRefreshed(db, user, templ)})
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
			"rows":   answersRefreshed(db, user, templ),
		})
	}
}

// sendFormAnswerMail wysyła jeden mail do autora odpowiedzi i zapisuje wysyłkę
// w logu (jak /api/formanssendmail, tylko z obsługą błędów).
func sendFormAnswerMail(db *DB, mailConf MailConfig, user User, templ FormTemplate, form Form, mail Notification) error {
	if form.Email.String == "" {
		return fmt.Errorf("answer %d has no e-mail", form.ID)
	}

	mailConf.EmbededHTMLImgs = getEmbeddedImgs(mail.EmbeddedImgsDelimited.String, user.URL, MEDIAEMAILSUBDIR, MEDIAROOT)
	parsedText, embImgs := parseFormMailTemplate(form.Name.String, form.Surname.String,
		makeSureIsHTML(mail.Text), form.ID, user, templ, db)

	if err := prepMailConfSendMail(form.Email.String, mail.Title.String, parsedText, user, embImgs, mailConf); err != nil {
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
func AnswersCSVAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := adminFormTemplate(w, r, db, cs, false)
		if !ok {
			return
		}
		lang := PanelLang(r)
		cols, byID := answerColumns(db, templ)
		rows := answerRows(db, user, templ, byID)

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
			for _, c := range cols {
				rec = append(rec, row.Values[c.Key])
			}
			cw.Write(rec)
		}
	}
}

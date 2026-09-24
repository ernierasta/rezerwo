package main

// forms_manage.go: zakładanie, kopiowanie i kasowanie formularzy w nowym
// panelu. Formularze powtarzają się co roku (Bal 2025 -> Bal 2026), więc
// kopia z poprzedniego formularza jest pierwszą opcją, a nie dodatkiem.

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/gorilla/sessions"
)

// slugify robi z nazwy adres: "Bal 2026 – deklaracja" -> "bal-2026-deklaracja".
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer("ł", "l", "đ", "d", "ß", "ss").Replace(s)
	var b strings.Builder
	dash := false
	for _, r := range norm.NFD.String(s) {
		switch {
		case unicode.Is(unicode.Mn, r):
			// znak diakrytyczny po rozłożeniu - pomijamy
		case r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	return out
}

var formURLRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{2,60}$`)

// uniqueFormURL dopisuje -2, -3, ... gdy adres jest zajęty.
func uniqueFormURL(db *DB, userID int64, base string) string {
	if base == "" {
		base = "formularz"
	}
	url := base
	for i := 2; db.FormTemplateURLTaken(url, userID, 0); i++ {
		url = fmt.Sprintf("%s-%d", base, i)
	}
	return url
}

// personFields to startowe pola "dane osoby": imię, nazwisko, e-mail.
func personFields(lang string) []FormDefField {
	f := func(label, name string, width int, subtype string) FormDefField {
		return FormDefField{
			"type": "text", "subtype": subtype, "required": true,
			"label": label, "name": name,
			"className": fmt.Sprintf("form-control row-1 col-md-%d", width),
		}
	}
	return []FormDefField{
		{"type": "header", "subtype": "h2", "label": T(lang, "fields.preset.person_header")},
		f(T(lang, "fields.role.name"), ROLENAME, 4, "text"),
		f(T(lang, "fields.role.surname"), ROLESURNAME, 4, "text"),
		f(T(lang, "fields.role.email"), ROLEEMAIL, 4, "email"),
	}
}

type nFormCreateMsg struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Language string `json:"language"`
	FromID   int64  `json:"from_id"`
	Preset   string `json:"preset"` // empty | person
}

// FormCreateAPI - POST /api/forms
func FormCreateAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		lang := PanelLang(r)

		var msg nFormCreateMsg
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		msg.Name = strings.TrimSpace(msg.Name)
		if msg.Name == "" {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "newform.need_name")})
			return
		}
		url := strings.TrimSpace(msg.URL)
		if url == "" {
			url = uniqueFormURL(db, user.ID, slugify(msg.Name))
		}
		if !formURLRe.MatchString(url) {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "newform.bad_url")})
			return
		}
		if db.FormTemplateURLTaken(url, user.ID, 0) {
			jsonMsg(w, http.StatusConflict, map[string]string{"msg": T(lang, "newform.url_taken", url)})
			return
		}

		t := FormTemplate{
			Name:        msg.Name,
			URL:         url,
			CreatedDate: nowUnix(),
			UserID:      user.ID,
			Content:     ToNS("[]"),
		}
		language := KnownLang(msg.Language)

		if msg.FromID > 0 {
			src, err := db.FormTemplateGetByID(msg.FromID, user.ID)
			if err != nil || isAddonsTemplate(src) {
				jsonMsg(w, http.StatusNotFound, map[string]string{"msg": "form not found"})
				return
			}
			t.HowTo, t.Banner, t.InfoPanel, t.ThankYou = src.HowTo, src.Banner, src.InfoPanel, src.ThankYou
			t.Content = src.Content
			t.MoneyAmountFieldName = src.MoneyAmountFieldName
			t.BankAccountID = src.BankAccountID
			t.NotificationID = src.NotificationID
			if msg.Language == "" {
				language = formLang(src)
			}
		} else if msg.Preset == "person" {
			b, _ := json.Marshal(personFields(language))
			t.Content = ToNS(string(b))
		}

		id, err := db.FormTemplateAdd(&t)
		if err != nil {
			log.Printf("FormCreateAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(lang, "common.save_error") + ": " + err.Error()})
			return
		}
		if err := db.FormTemplateSetAccess(id, user.ID, language, "open", ToNI(0)); err != nil {
			log.Printf("FormCreateAPI: access: %v", err)
		}
		if msg.FromID > 0 {
			src, _ := db.FormTemplateGetByID(msg.FromID, user.ID)
			if src.AdminNotificationID.Valid {
				db.FormTemplateSetAdminNotification(id, user.ID, src.AdminNotificationID)
			}
		}

		var ffd []FormFieldDef
		if err := json.Unmarshal([]byte(t.Content.String), &ffd); err == nil {
			updateFormFieldsInDB(id, ffd, db)
		}

		next := fmt.Sprintf("/admin/forms/%d", id)
		if msg.FromID == 0 {
			next = fmt.Sprintf("/admin/forms/%d/fields", id)
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "created", "id": id, "next": next})
	}
}

// FormDeleteAPI - DELETE /api/forms/{id}
func FormDeleteAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := adminFormTemplate(w, r, db, cs, false)
		if !ok {
			return
		}
		if isAddonsTemplate(templ) {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "addons template belongs to event"})
			return
		}
		if err := db.FormTemplateDelete(templ.ID, user.ID); err != nil {
			log.Printf("FormDeleteAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]string{"msg": "deleted", "next": "/admin/start"})
	}
}

// FormURLSuggestAPI - GET /api/forms/url?name=...  (podpowiedź adresu w oknie "Nowy formularz")
func FormURLSuggestAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		jsonMsg(w, http.StatusOK, map[string]string{
			"url":  uniqueFormURL(db, user.ID, slugify(r.URL.Query().Get("name"))),
			"base": "/form/" + user.URL + "/",
		})
	}
}

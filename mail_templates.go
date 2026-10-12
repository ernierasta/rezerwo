package main

// mail_templates.go: gotowe szablony maili i okno "Nowy mail" w katalogach.
//
// Zamiast współdzielenia maili między organizacjami (flaga sharable, przez
// którą każdy widział cudze maile) nowy mail powstaje jako kopia: pusty,
// z gotowego szablonu albo z własnego maila (np. "Bal 2025" -> "Bal 2026").
//
// Szablony to pliki mailtemplates/<język>/*.html z nagłówkiem w komentarzu:
//
//	<!--
//	name: Podziękowanie za rezerwację z kodem QR
//	description: Wysyłany gościowi zaraz po zamówieniu.
//	related_to: events
//	subject: Rezerwacja: {{.EventName}}
//	-->
//	<p>Dzień dobry {{.Name}} ...</p>
//
// Pliki czytamy przy każdym żądaniu, więc poprawki nie wymagają restartu.
// {{.QRPayMail ""}} oznacza "kod QR, konto do wybrania" - przy tworzeniu
// wpisujemy konto organizacji, jeśli ma tylko jedno.

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/sessions"
)

const MAILTEMPLATESDIR = "mailtemplates"

type mailTemplate struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	RelatedTo   string `json:"related_to"`
	Subject     string `json:"subject"`
	Text        string `json:"text"`
}

// loadMailTemplates zwraca szablony w języku panelu, a gdy ich brak - polskie.
func loadMailTemplates(lang string) []mailTemplate {
	dir := filepath.Join(MAILTEMPLATESDIR, KnownLang(lang))
	files, _ := filepath.Glob(filepath.Join(dir, "*.html"))
	if len(files) == 0 {
		files, _ = filepath.Glob(filepath.Join(MAILTEMPLATESDIR, DEFAULTLANG, "*.html"))
	}
	sort.Strings(files)

	var out []mailTemplate
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			log.Printf("loadMailTemplates: %v", err)
			continue
		}
		t, err := parseMailTemplate(string(b))
		if err != nil {
			log.Printf("loadMailTemplates: %s: %v", f, err)
			continue
		}
		t.Key = filepath.Base(filepath.Dir(f)) + "/" + strings.TrimSuffix(filepath.Base(f), ".html")
		out = append(out, t)
	}
	return out
}

func parseMailTemplate(s string) (mailTemplate, error) {
	var t mailTemplate
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "<!--") {
		return t, fmt.Errorf("no header comment")
	}
	end := strings.Index(s, "-->")
	if end < 0 {
		return t, fmt.Errorf("unterminated header comment")
	}
	for _, line := range strings.Split(s[4:end], "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "name":
			t.Name = v
		case "description":
			t.Description = v
		case "related_to":
			t.RelatedTo = v
		case "subject":
			t.Subject = v
		}
	}
	// treść w pliku może być ładnie złamana, w edytorze wiersze to <p>
	var lines []string
	for _, l := range strings.Split(s[end+3:], "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	t.Text = strings.Join(lines, "")
	if t.Name == "" || (t.RelatedTo != "events" && t.RelatedTo != "forms") {
		return t, fmt.Errorf("missing name or related_to")
	}
	if err := checkMailTemplate(t.Subject, t.Text); err != nil {
		return t, err
	}
	return t, nil
}

func findMailTemplate(key string) (mailTemplate, bool) {
	lang, _, _ := strings.Cut(key, "/")
	for _, t := range loadMailTemplates(lang) {
		if t.Key == key {
			return t, true
		}
	}
	return mailTemplate{}, false
}

// fillOnlyAccount wpisuje jedyne konto organizacji do kodów QR bez konta.
func fillOnlyAccount(db *DB, user User, text string) string {
	accs, err := db.BankAccountGetAll(user.ID)
	if err != nil || len(accs) != 1 {
		return text
	}
	return strings.ReplaceAll(text, `{{.QRPayMail ""}}`, fmt.Sprintf(`{{.QRPayMail %q}}`, accs[0].Name))
}

type nMailSource struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	RelatedTo string `json:"related_to"`
	Subject   string `json:"subject"`
	Text      string `json:"text"`
	FormID    int64  `json:"form_id,omitempty"`
}

// MailSourcesAPI - GET /api/mails/sources: szablony i własne maile do okna
// "Nowy mail".
func MailSourcesAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		lang := PanelLang(r)

		mine := []nMailSource{}
		mails, err := db.NotificationGetAllForUser(user.ID)
		if err != nil {
			log.Printf("MailSourcesAPI: %v", err)
		}
		for _, m := range mails {
			mine = append(mine, nMailSource{
				ID:        m.ID,
				Name:      m.Name,
				RelatedTo: m.RelatedTo,
				Subject:   m.Title.String,
				Text:      m.Text,
				FormID:    m.FormTemplateID.Int64,
			})
		}
		// najnowsze na górze - zwykle kopiuje się zeszłoroczny mail
		sort.SliceStable(mine, func(i, j int) bool { return mine[i].ID > mine[j].ID })

		templates := loadMailTemplates(lang)
		if templates == nil {
			templates = []mailTemplate{}
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"templates": templates, "mine": mine, "forms": mailForms(db, user, false)})
	}
}

type nMailCreateMsg struct {
	Name     string `json:"name"`
	Template string `json:"template"`
	CopyOf   int64  `json:"copy_of"`
	// RelatedTo - dla pustego maila (szablon i kopia mają własne)
	RelatedTo string `json:"related_to"`
	// FormID - formularz, do którego należy mail formularza (wymagany);
	// Template "form:guest" / "form:admin" - mail z wszystkimi jego polami
	FormID int64 `json:"form_id"`
}

// MailCreateAPI - POST /api/mails: pusty mail, z szablonu albo kopia własnego.
func MailCreateAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		var m nMailCreateMsg
		body, err := io.ReadAll(r.Body)
		if err == nil {
			err = json.Unmarshal(body, &m)
		}
		if err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		m.Name = strings.TrimSpace(m.Name)
		if m.Name == "" {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "empty name"})
			return
		}

		mail := Notification{
			Name:        m.Name,
			Type:        "email",
			RelatedTo:   "events",
			Title:       ToNS(m.Name),
			CreatedDate: time.Now().Unix(),
			UserID:      user.ID,
		}
		if m.RelatedTo == "forms" {
			mail.RelatedTo = "forms"
		}

		switch {
		case m.CopyOf != 0:
			src, err := db.NotificationGetByID(m.CopyOf, user.ID)
			if err != nil {
				jsonMsg(w, http.StatusNotFound, map[string]string{"msg": "mail not found"})
				return
			}
			mail.Type = src.Type
			mail.RelatedTo = src.RelatedTo
			mail.Title = src.Title
			mail.Text = src.Text
			// pliki leżą w katalogu organizacji, więc kopia może ich używać
			mail.EmbeddedImgsDelimited = src.EmbeddedImgsDelimited
			mail.AttachedFilesDelimited = src.AttachedFilesDelimited
		case m.Template == "form:guest" || m.Template == "form:admin":
			templ, err := ownedForm(db, user, m.FormID)
			if err != nil {
				jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(PanelLang(r), "mail.form_needed")})
				return
			}
			admin := m.Template == "form:admin"
			subject, text := formMailDraft(db, user, templ, admin, draftLang(r, templ, admin))
			mail.RelatedTo = "forms"
			mail.Title = ToNS(subject)
			mail.Text = text
		case m.Template != "":
			t, ok := findMailTemplate(m.Template)
			if !ok {
				jsonMsg(w, http.StatusNotFound, map[string]string{"msg": "template not found"})
				return
			}
			mail.RelatedTo = t.RelatedTo
			mail.Title = ToNS(t.Subject)
			mail.Text = fillOnlyAccount(db, user, t.Text)
		}

		if mail.RelatedTo == "forms" {
			if _, err := ownedForm(db, user, m.FormID); err != nil {
				jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(PanelLang(r), "mail.form_needed")})
				return
			}
			mail.FormTemplateID = ToNI(m.FormID)
		}

		id, err := db.NotificationAdd(&mail)
		if err != nil {
			log.Printf("MailCreateAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "created", "id": id})
	}
}

package main

// forms_fields.go: własny designer pól (rozdz. 7.2 wariant B HARMONIJKA.md).
//
// Strona /admin/forms/{id}/fields zastępuje formBuildera. Zapisuje ten sam
// format JSON (formtemplates.content), więc raporty, formfields i FormFuncs
// w szablonach maili działają bez zmian. Z tego samego designera korzystają
// dodatki do zamówienia (szablon rodzaju order_addons).

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/sessions"
)

type nFieldsVars struct {
	NPage
	ID        int64
	Name      string
	IsAddons  bool
	BackURL   string
	PublicURL string
	FormLang  string
	Currency  string

	Content    template.JS // definicja pól (JSON)
	MoneyField string
	Counts     template.JS // {"nazwa pola": liczba odpowiedzi}
	PanelMode  string      // tryb panelu informacyjnego (form_stats.go)
}

// FormFieldsPage - /admin/forms/{id}/fields
func FormFieldsPage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := adminFormTemplate(w, r, db, cs, true)
		if !ok {
			return
		}
		lang := PanelLang(r)

		defs, err := ParseFormDef(templ.Content.String)
		if err != nil {
			log.Printf("FormFieldsPage: form %d: %v", templ.ID, err)
			defs = []FormDefField{}
		}
		content, _ := json.Marshal(defs)

		counts, err := db.FormFieldAnswerCounts(templ.ID)
		if err != nil {
			log.Printf("FormFieldsPage: counts: %v", err)
		}
		countsJSON, _ := json.Marshal(counts)

		v := nFieldsVars{
			NPage: NPage{
				Title:   T(lang, "fields.title") + " – " + templ.Name,
				Crumb:   templ.Name,
				OrgName: orgName(user),
			},
			ID:         templ.ID,
			Name:       templ.Name,
			IsAddons:   isAddonsTemplate(templ),
			PublicURL:  fmt.Sprintf("/form/%s/%s", user.URL, templ.URL),
			FormLang:   formLang(templ),
			Currency:   formCurrency(db, user, templ),
			Content:    template.JS(content),
			MoneyField: templ.MoneyAmountFieldName.String,
			Counts:     template.JS(countsJSON),
			BackURL:    fmt.Sprintf("/admin/forms/%d#pola", templ.ID),
			PanelMode:  panelMode(templ),
		}
		if v.IsAddons && templ.EventID.Valid {
			v.BackURL = fmt.Sprintf("/admin/events/%d#dodatki", templ.EventID.Int64)
			if ev, err := db.EventGetByID(templ.EventID.Int64); err == nil && ev.UserID == user.ID {
				v.Crumb = ev.Name
				v.Currency = ev.DefaultCurrency
			}
		}
		if back := r.URL.Query().Get("back"); strings.HasPrefix(back, "/admin/") {
			v.BackURL = back
		}
		nRender(w, lang, v, "tmpl/n_fields.html")
	}
}

type nFieldsSaveMsg struct {
	Content    []FormDefField    `json:"content"`
	MoneyField string            `json:"moneyfield"`
	Renames    map[string]string `json:"renames"`
}

var fieldNameRe = regexp.MustCompile(`^[\p{L}0-9_.-]{1,80}$`)

// FormFieldsSaveAPI - POST /api/forms/{id}/fields
func FormFieldsSaveAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := adminFormTemplate(w, r, db, cs, false)
		if !ok {
			return
		}
		lang := PanelLang(r)

		body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		var msg nFieldsSaveMsg
		if err := json.Unmarshal(body, &msg); err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		if msg.Content == nil {
			msg.Content = []FormDefField{}
		}

		defs, err := CleanFormDef(msg.Content)
		if err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "fields.err.invalid", err.Error())})
			return
		}
		names := map[string]bool{}
		for _, d := range defs {
			if n := d.S("name"); n != "" {
				if !fieldNameRe.MatchString(n) {
					jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "fields.err.bad_key", n)})
					return
				}
				names[n] = true
			}
		}
		money := strings.TrimSpace(msg.MoneyField)
		if money != "" && !names[money] {
			money = ""
		}

		// zmiany nazw pól: najpierw na nazwy tymczasowe, potem na docelowe,
		// żeby zamiana dwóch nazw miejscami nie zderzyła się z UNIQUE
		type rename struct{ from, tmp, to string }
		var rr []rename
		i := 0
		for from, to := range msg.Renames {
			if from == "" || to == "" || from == to || !names[to] {
				continue
			}
			rr = append(rr, rename{from, fmt.Sprintf("__rename_%d_%d", templ.ID, i), to})
			i++
		}
		for _, x := range rr {
			if err := db.FormFieldRename(templ.ID, x.from, x.tmp); err != nil {
				log.Printf("FormFieldsSaveAPI: rename %q: %v", x.from, err)
			}
		}
		for _, x := range rr {
			if err := db.FormFieldRename(templ.ID, x.tmp, x.to); err != nil {
				log.Printf("FormFieldsSaveAPI: rename %q -> %q: %v", x.from, x.to, err)
			}
		}

		content, err := json.Marshal(defs)
		if err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		if err := db.FormTemplateSetContent(templ.ID, user.ID, string(content), ToNS(money)); err != nil {
			log.Printf("FormFieldsSaveAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(lang, "common.save_error") + ": " + err.Error()})
			return
		}

		// formfields: lista pól, z której korzystają raporty i FormFuncs
		var ffd []FormFieldDef
		if err := json.Unmarshal(content, &ffd); err == nil {
			updateFormFieldsInDB(templ.ID, ffd, db)
		}

		counts, _ := db.FormFieldAnswerCounts(templ.ID)
		jsonMsg(w, http.StatusOK, map[string]interface{}{
			"msg":        "saved",
			"content":    defs,
			"moneyfield": money,
			"counts":     counts,
		})
	}
}

// fieldSummaries opisuje pola na stronie formularza (sekcja "Pola").
type nFieldSummary struct {
	Label string
	Kind  string
	Role  string
	Req   bool
}

func formFieldSummaries(templ FormTemplate, lang string) []nFieldSummary {
	defs, err := ParseFormDef(templ.Content.String)
	if err != nil {
		return nil
	}
	var out []nFieldSummary
	for _, d := range defs {
		t := d.S("type")
		if !isInputType(t) {
			continue
		}
		s := nFieldSummary{
			Label: plainLabel(d.S("label")),
			Kind:  T(lang, "fields.type."+t),
			Req:   d.B("required"),
		}
		switch d.S("name") {
		case ROLENAME:
			s.Role, s.Req = T(lang, "fields.role.name"), true
		case ROLESURNAME:
			s.Role, s.Req = T(lang, "fields.role.surname"), true
		case ROLEEMAIL:
			s.Role, s.Req = T(lang, "fields.role.email"), true
		}
		if n := d.S("name"); n != "" && n == templ.MoneyAmountFieldName.String {
			if s.Role != "" {
				s.Role += ", "
			}
			s.Role += T(lang, "fields.role.money")
		}
		out = append(out, s)
	}
	return out
}

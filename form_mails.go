package main

// form_mails.go: maile formularzy. Każdy należy do jednego formularza
// (notifications.formtemplates_id_fk, tools/migration-mailform.sql):
// edytor maila podpowiada pola tego formularza (formMailVars), okno
// "Nowy mail" proponuje mail z wszystkimi polami (formMailDraft), a podgląd
// pokazuje zmienne jako etykiety pól (formMailPreview) - odpowiedzi gościa
// w nim nie ma.

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gorilla/sessions"
)

// formMailFields - pola formularza do maila: pola z odpowiedzią i nagłówki
// sekcji, bez imienia, nazwiska i e-maila (te mają własne zmienne).
func formMailFields(templ FormTemplate) []FormDefField {
	var out []FormDefField
	defs, _ := ParseFormDef(templ.Content.String)
	for _, d := range defs {
		name := d.S("name")
		if d.S("type") == "header" {
			if fieldLabel(d) != "" {
				out = append(out, d)
			}
			continue
		}
		if !isInputType(d.S("type")) || name == "" || name == ROLENAME || name == ROLESURNAME || name == ROLEEMAIL {
			continue
		}
		out = append(out, d)
	}
	return out
}

func fieldLabel(d FormDefField) string {
	if l := plainLabel(d.S("label")); l != "" {
		return l
	}
	return d.S("name")
}

// userForms - zwykłe formularze organizacji (bez definicji dodatków),
// najnowsze pierwsze.
func userForms(db *DB, user User) []FormTemplate {
	all, err := db.FormTemplateGetAll(user.ID)
	if err != nil {
		log.Printf("userForms: %v", err)
	}
	var out []FormTemplate
	for _, t := range all {
		if t.Kind.String != ADDONSKIND {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

type mailForm struct {
	ID   int64     `json:"id"`
	Name string    `json:"name"`
	Vars []mailVar `json:"vars,omitempty"`
}

// mailForms - formularze do wyboru w edytorze maila i oknie "Nowy mail";
// withVars - z ich zmiennymi (edytor przełącza je bez przeładowania).
func mailForms(db *DB, user User, withVars bool) []mailForm {
	out := []mailForm{}
	for _, t := range userForms(db, user) {
		f := mailForm{ID: t.ID, Name: t.Name}
		if withVars {
			f.Vars = formMailVars(t)
		}
		out = append(out, f)
	}
	return out
}

// ownedForm - formularz organizacji (nie dodatki) albo błąd.
func ownedForm(db *DB, user User, id int64) (FormTemplate, error) {
	t, err := db.FormTemplateGetByID(id, user.ID)
	if err == nil && t.Kind.String == ADDONSKIND {
		err = fmt.Errorf("form %d is order addons", id)
	}
	return t, err
}

func fieldAction(name string) string { return `{{.Field ` + strconv.Quote(name) + `}}` }

// formMailDraft - propozycja maila z wszystkimi polami formularza: dla
// gościa (potwierdzenie z odpowiedziami i danymi do przelewu) albo dla
// administratora (nowe zgłoszenie). lang - język maila.
func formMailDraft(db *DB, user User, templ FormTemplate, admin bool, lang string) (string, string) {
	esc := template.HTMLEscapeString
	var b strings.Builder
	p := func(s string) { b.WriteString("<p>" + s + "</p>") }
	gap := func() { p("<br>") }
	line := func(label, value string) { p(esc(label) + ": <strong>" + value + "</strong>") }

	var subject string
	if admin {
		subject = T(lang, "mailgen.admin_subject", templ.Name) + " – {{.Name}} {{.Surname}}"
		p(esc(T(lang, "mailgen.admin_intro", templ.Name)))
	} else {
		subject = T(lang, "mailgen.guest_subject", templ.Name)
		p(esc(T(lang, "mailgen.hello")) + ",")
		gap()
		p(esc(T(lang, "mailgen.thanks", templ.Name)))
	}
	gap()
	line(T(lang, "mailgen.name"), "{{.Name}} {{.Surname}}")
	line(T(lang, "mail.var.email"), "{{.Email}}")
	for _, d := range formMailFields(templ) {
		if d.S("type") == "header" {
			gap()
			p("<strong>" + esc(fieldLabel(d)) + "</strong>")
			continue
		}
		line(fieldLabel(d), fieldAction(d.S("name")))
	}

	if templ.MoneyAmountFieldName.String != "" {
		gap()
		line(T(lang, "mail.var.total"), "{{.TotalPrice}}")
		if !admin && templ.BankAccountID.Valid {
			if ba, err := db.BankAccountGetByID(templ.BankAccountID.Int64, user.ID); err == nil {
				acc := strconv.Quote(ba.Name)
				gap()
				p(esc(T(lang, "mailgen.pay")))
				p("{{.PayInfo " + acc + "}}")
				p("{{.QRPayMail " + acc + "}}")
			}
		}
	}

	if !admin {
		gap()
		p(esc(T(lang, "mailgen.bye")))
		if org := user.Organization.String; org != "" {
			p(esc(org))
		}
	}
	return subject, b.String()
}

// mailAction - akcja szablonu z argumentami w cudzysłowach, jak w n_vars.js
var mailAction = regexp.MustCompile(`\{\{\s*(\.[A-Za-z]+)((?:\s+"(?:[^"\\]|\\.)*")*)\s*\}\}`)
var mailActionArg = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)

// varLabelFor - etykieta znanej zmiennej (jak pastylka w edytorze) albo "".
func varLabelFor(vars []mailVar, lang, code string, args []string) string {
	if (code == ".QRPayMail" || code == ".QRPay") && len(args) == 1 {
		return T(lang, "mail.qr") + ": " + args[0]
	}
	for _, v := range vars {
		switch {
		case v.Code != code:
		case v.Account && len(args) == 1:
			return T(lang, v.Label) + ": " + args[0]
		case v.Arg != "" && len(args) == 1 && args[0] == v.Arg:
			if v.Text != "" {
				return v.Text
			}
			return T(lang, v.Label)
		case !v.Account && v.Arg == "" && len(args) == 0:
			return T(lang, v.Label)
		}
	}
	return ""
}

// formMailPreview - treść i temat maila formularza z etykietami zamiast
// odpowiedzi. Szablonu nie wykonujemy, więc inne akcje ({{.Sum ..}},
// {{if}}) zostają widoczne jako kod.
func formMailPreview(templ FormTemplate, lang, subject, text string) (string, string) {
	vars := formMailVars(templ)
	parse := func(m []string) (string, []string) {
		var args []string
		for _, a := range mailActionArg.FindAllString(m[2], -1) {
			if u, err := strconv.Unquote(a); err == nil {
				args = append(args, u)
			} else {
				args = append(args, strings.Trim(a, `"`))
			}
		}
		return m[1], args
	}
	subject = mailAction.ReplaceAllStringFunc(subject, func(s string) string {
		code, args := parse(mailAction.FindStringSubmatch(s))
		if l := varLabelFor(vars, lang, code, args); l != "" {
			return "[" + l + "]"
		}
		return s
	})
	text = mailAction.ReplaceAllStringFunc(text, func(s string) string {
		code, args := parse(mailAction.FindStringSubmatch(s))
		if l := varLabelFor(vars, lang, code, args); l != "" {
			return `<span style="background:#e8eef6;border-radius:4px;padding:0 5px;color:#2b4a6f">` + template.HTMLEscapeString(l) + `</span>`
		}
		return `<code>` + template.HTMLEscapeString(s) + `</code>`
	})
	// akcje, których wyrażenie nie objęło ({{if ...}}, {{end}}), też jako tekst
	text = strings.NewReplacer("{{", "&#123;&#123;", "}}", "&#125;&#125;").Replace(text)
	return subject, text
}

// FormMailDraftAPI - GET /api/forms/{id}/maildraft?for=guest|admin:
// propozycja maila z polami formularza do okna "Nowy mail", z podglądem.
func FormMailDraftAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := adminFormTemplate(w, r, db, cs, false)
		if !ok {
			return
		}
		admin := r.URL.Query().Get("for") == "admin"
		lang := draftLang(r, templ, admin)
		subject, text := formMailDraft(db, user, templ, admin, lang)
		ps, ph := formMailPreview(templ, PanelLang(r), subject, text)
		jsonMsg(w, http.StatusOK, map[string]interface{}{
			"subject": subject, "text": text,
			"preview": map[string]interface{}{"subject": ps, "html": ph, "to": user.Email},
		})
	}
}

// draftLang - mail dla gościa w języku formularza, dla administratora w języku panelu.
func draftLang(r *http.Request, templ FormTemplate, admin bool) string {
	if admin {
		return PanelLang(r)
	}
	return formLang(templ)
}

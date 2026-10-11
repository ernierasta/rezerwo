package main

// form_edit.go: poprawianie wysłanego formularza przez gościa.
//
// Wcześniejsze odpowiedzi pokazujemy tylko temu, kto udowodni, że to jego
// zgłoszenie: ciasteczkiem z przeglądarki, z której je wysłał, albo linkiem
// z maila z potwierdzeniem (dowód, że ma dostęp do skrzynki). Sam adres
// e-mail, imię i nazwisko nie wystarczą - zna je wiele osób. Poprawiać można
// przez EDITDAYS dni od pierwszego wysłania (i dopóki formularz jest otwarty).

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// EDITDAYS - przez tyle dni od wysłania gość może poprawić odpowiedzi
const EDITDAYS = 5

// SetSignKey ustawia klucz podpisów (linki podziękowania i poprawiania,
// ciasteczko zgłoszenia) wyprowadzony z AuthenticationKey z config.toml, żeby
// podpisy przeżyły restart serwera. Bez wywołania (testy) klucz jest losowy.
func SetSignKey(authKey []byte) {
	m := hmac.New(sha256.New, authKey)
	m.Write([]byte("rezerwo form links"))
	doneKey = m.Sum(nil)
}

func formSig(kind string, formID int64) string {
	m := hmac.New(sha256.New, doneKey)
	fmt.Fprintf(m, "%s:%d", kind, formID)
	return hex.EncodeToString(m.Sum(nil))[:24]
}

// editToken - "<id>.<podpis>": w linku z maila i w ciasteczku
func editToken(formID int64) string {
	return strconv.FormatInt(formID, 10) + "." + formSig("edit", formID)
}

// parseEditToken zwraca numer zgłoszenia z poprawnie podpisanego tokenu, inaczej 0
func parseEditToken(tok string) int64 {
	i := strings.IndexByte(tok, '.')
	if i <= 0 {
		return 0
	}
	id, err := strconv.ParseInt(tok[:i], 10, 64)
	if err != nil || id <= 0 || !hmac.Equal([]byte(tok[i+1:]), []byte(formSig("edit", id))) {
		return 0
	}
	return id
}

// editUntil - do kiedy zgłoszenie można poprawiać
func editUntil(f Form) int64 {
	return f.CreatedDate + EDITDAYS*24*3600
}

func editable(f Form, now int64) bool {
	return now <= editUntil(f)
}

func editURL(base string, user User, templ FormTemplate, formID int64) string {
	return fmt.Sprintf("%s/form/%s/%s?edit=%s", base, user.URL, templ.URL, editToken(formID))
}

// --- ciasteczko: "to zgłoszenie wysłano z tej przeglądarki" ----------------

func formCookieName(templ FormTemplate) string {
	return fmt.Sprintf("rzform%d", templ.ID)
}

func setFormCookie(w http.ResponseWriter, r *http.Request, user User, templ FormTemplate, f Form) {
	http.SetCookie(w, &http.Cookie{
		Name:     formCookieName(templ),
		Value:    editToken(f.ID),
		Path:     fmt.Sprintf("/form/%s/%s", user.URL, templ.URL),
		Expires:  time.Unix(editUntil(f), 0),
		HttpOnly: true,
		Secure:   requestScheme(r) == "https",
		SameSite: http.SameSiteLaxMode,
	})
}

// cookieFormID - zgłoszenie wysłane z tej przeglądarki (0 = brak)
func cookieFormID(r *http.Request, templ FormTemplate) int64 {
	c, err := r.Cookie(formCookieName(templ))
	if err != nil {
		return 0
	}
	return parseEditToken(c.Value)
}

// requestScheme / requestBase - adres serwera do linków w mailach; za
// serwerem pośredniczącym liczy się X-Forwarded-Proto.
func requestScheme(r *http.Request) string {
	if p := r.Header.Get("X-Forwarded-Proto"); p == "https" || p == "http" {
		return p
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func requestBase(r *http.Request) string {
	return requestScheme(r) + "://" + r.Host
}

// --- wpisanie zapisanych odpowiedzi w formularz (tryb poprawiania) ----------

// savedField - zapisana odpowiedź w formacie szkicu z n_formpage.js:
// v - wartości, o - tekst opcji "Inne"
type savedField struct {
	V []string `json:"v"`
	O string   `json:"o,omitempty"`
}

// prefillJSON zamienia zapisane odpowiedzi na dane do wpisania w pola.
func prefillJSON(db *DB, defs []FormDefField, formID, templID int64) string {
	fields, _ := db.FormFieldGetAllForTmpl(templID)
	names := map[int64]string{}
	for _, f := range fields {
		names[f.ID] = f.Name
	}
	stored := map[string]string{}
	answers, _ := db.FormAnswerGetAll(formID)
	for _, a := range answers {
		if n, ok := names[a.FormFieldID]; ok && a.RepeatIndex.Int64 == 0 {
			stored[n] = a.Value.String
		}
	}
	out := map[string]savedField{}
	for _, d := range defs {
		name := d.S("name")
		val, ok := stored[name]
		if name == "" || !ok {
			continue
		}
		typ := d.S("type")
		multi := typ == "checkbox-group" || (typ == "select" && d.B("multiple"))
		if !multi && typ != "radio-group" {
			out[name] = savedField{V: []string{val}}
			continue
		}
		var vals []string
		if val != "" {
			vals = strings.Split(val, ", ")
			if !multi {
				vals = []string{val}
			}
		}
		sf := savedField{V: vals}
		// wartość spoza opcji to tekst "Inne"
		known := map[string]bool{}
		eachOption(d, func(v, _ string) { known[v] = true })
		for _, v := range vals {
			if !known[v] {
				sf.O = v
			}
		}
		out[name] = sf
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// addEditLink dopisuje do maila gościa link do poprawienia odpowiedzi (szablony
// maili organizacji go nie mają, a bez niego nie da się poprawić zgłoszenia
// z innego urządzenia).
func addEditLink(text, lang, link string, until int64) string {
	p := fmt.Sprintf(`<p style="margin-top:18px;color:#555;font-size:13px">%s <a href="%s">%s</a></p>`,
		template.HTMLEscapeString(T(lang, "gform.mail_edit", FormatDate(lang, until))),
		template.HTMLEscapeString(link), template.HTMLEscapeString(T(lang, "gform.mail_edit_link")))
	if i := strings.LastIndex(strings.ToLower(text), "</body>"); i >= 0 {
		return text[:i] + p + text[i:]
	}
	if i := strings.LastIndex(strings.ToLower(text), "</html>"); i >= 0 {
		return text[:i] + p + text[i:]
	}
	return text + p
}

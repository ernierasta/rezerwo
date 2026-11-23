package main

// form_conds_mail.go: pola warunkowe w mailach formularzy.
//
// Gość widzi tylko część pól (show_if, depends_on - formconds.go), więc mail
// pokazuje tylko je:
//   - linijka treści (akapit, punkt listy, nagłówek, wiersz tabeli) z pastylką
//     pola, którego gość nie widział, znika w całości (hideHiddenLines); lista
//     tuż pod taką linijką (Quill: wcięte szczegóły do pola) znika razem z nią;
//   - {{.Answers}} wstawia odpowiedzi tylko z pól, które gość widział.
// Admin nic nie ustawia: przed wykonaniem szablonu linijki z polami dostają
// {{if .Shown "pole"}}...{{end}}. Szablon, który po tej zmianie się nie
// parsuje, idzie po staremu (parseFormMailTemplate).

import (
	"html/template"
	"log"
	"regexp"
	"strconv"
	"strings"
)

// mailBlockTags - elementy, które są "linijką" maila
var mailBlockTags = []string{"p", "li", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "pre", "tr"}

var (
	mailBlockOpen = regexp.MustCompile(`(?i)<(p|li|h[1-6]|blockquote|pre|tr)\b[^>]*>`)
	mailBlockEnd  = map[string]*regexp.Regexp{}
	mailBlockSame = map[string]*regexp.Regexp{}
	// akcje z nazwą pola jako pierwszym argumentem
	mailFieldRef  = regexp.MustCompile(`\{\{-?\s*\.(?:Field|FieldOr|MAmmount|MMultipl|MTotal)\s+("(?:[^"\\]|\\.)*"|` + "`[^`]*`" + `)`)
	mailCtlOpen   = regexp.MustCompile(`\{\{-?\s*(?:if|range|with|block|define)\b`)
	mailCtlEnd    = regexp.MustCompile(`\{\{-?\s*end\b`)
	mailListOpen  = regexp.MustCompile(`(?i)^\s*<(?:ul|ol)\b[^>]*>`)
	mailListInner = regexp.MustCompile(`(?i)<(?:ul|ol)\b`)
	mailListEnd   = regexp.MustCompile(`(?i)</(?:ul|ol)\s*>`)
)

// listAfter - długość listy na początku s (0, gdy jej nie ma albo ma
// zagnieżdżone listy)
func listAfter(s string) int {
	open := mailListOpen.FindStringIndex(s)
	if open == nil {
		return 0
	}
	end := mailListEnd.FindStringIndex(s[open[1]:])
	if end == nil || mailListInner.MatchString(s[open[1]:open[1]+end[0]]) {
		return 0
	}
	return open[1] + end[1]
}

func init() {
	for _, t := range mailBlockTags {
		mailBlockEnd[t] = regexp.MustCompile(`(?i)</` + t + `\s*>`)
		mailBlockSame[t] = regexp.MustCompile(`(?i)<` + t + `\b`)
	}
}

// fieldRefs - nazwy pól użyte w kawałku szablonu
func fieldRefs(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range mailFieldRef.FindAllStringSubmatch(s, -1) {
		n, err := strconv.Unquote(m[1])
		if err != nil || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

// ctlBalanced - kawałek zamyka wszystkie swoje {{if}}/{{range}}/{{with}},
// więc można go objąć własnym {{if}}
func ctlBalanced(s string) bool {
	return len(mailCtlOpen.FindAllStringIndex(s, -1)) == len(mailCtlEnd.FindAllStringIndex(s, -1))
}

// hideHiddenLines obejmuje każdą linijkę z pastylką pola warunkiem
// {{if .Shown "pole" ...}}: linijka zostaje, gdy gość widział choć jedno z jej
// pól.
func hideHiddenLines(s string) string {
	var b strings.Builder
	for {
		loc := mailBlockOpen.FindStringSubmatchIndex(s)
		if loc == nil {
			b.WriteString(s)
			return b.String()
		}
		tag := strings.ToLower(s[loc[2]:loc[3]])
		rest := s[loc[1]:]
		end := mailBlockEnd[tag].FindStringIndex(rest)
		if end == nil || mailBlockSame[tag].MatchString(rest[:end[0]]) {
			// bez zamknięcia albo zagnieżdżony taki sam element - zostawiamy
			b.WriteString(s[:loc[1]])
			s = rest
			continue
		}
		b.WriteString(s[:loc[0]])
		block := s[loc[0] : loc[1]+end[1]]
		s = rest[end[1]:]
		names := fieldRefs(block)
		if len(names) == 0 || !ctlBalanced(block) {
			b.WriteString(block)
			continue
		}
		tail := ""
		if tag != "li" && tag != "tr" {
			if n := listAfter(s); n > 0 && ctlBalanced(s[:n]) {
				tail, s = s[:n], s[n:]
			}
		}
		b.WriteString("{{if .Shown")
		for _, n := range names {
			b.WriteString(" " + strconv.Quote(n))
		}
		b.WriteString("}}" + block + hideHiddenLines(tail) + "{{end}}")
	}
}

// parseFormMail - szablon maila formularza z ukrywaniem linijek pól, których
// gość nie widział; gdy się nie da, szablon jak jest.
func parseFormMail(name, text string) (*template.Template, error) {
	t, err := template.New(name).Parse(hideHiddenLines(text))
	if err == nil {
		return t, nil
	}
	log.Printf("parseFormMail: bez ukrywania pól: %v", err)
	return template.New(name).Parse(text)
}

// formHasConds - formularz ma pola warunkowe albo zależne opcje
func formHasConds(templ FormTemplate) bool {
	defs, _ := ParseFormDef(templ.Content.String)
	for k, c := range FieldConds(defs) {
		if c != nil || dependsOn(defs[k]) != "" {
			return true
		}
	}
	return false
}

// condView - pola formularza, ich widoczność dla gościa i zapisane odpowiedzi
type condView struct {
	defs []FormDefField
	vis  []bool
	vals map[string]string
}

func (i *FormFuncs) condView() *condView {
	if i.cv != nil {
		return i.cv
	}
	cv := &condView{vals: map[string]string{}}
	i.cv = cv
	defs, err := ParseFormDef(i.Template.Content.String)
	if err != nil {
		log.Printf("FormFuncs.condView: form %d: %v", i.Template.ID, err)
	}
	cv.defs = defs
	if i.FormID > 0 {
		ff, err := i.DB.FormFieldGetAllForTmpl(i.Template.ID)
		if err != nil {
			log.Printf("FormFuncs.condView: fields %d: %v", i.Template.ID, err)
		}
		names := map[int64]string{}
		for _, f := range ff {
			names[f.ID] = f.Name
		}
		fa, err := i.DB.FormAnswerGetAll(i.FormID)
		if err != nil {
			log.Printf("FormFuncs.condView: answers %d: %v", i.FormID, err)
		}
		for _, a := range fa {
			if n, ok := names[a.FormFieldID]; ok && a.RepeatIndex.Int64 == 0 {
				cv.vals[n] = a.Value.String
			}
		}
	}
	var data []FormAnswersJson
	for _, d := range defs {
		name := d.S("name")
		v := strings.TrimSpace(cv.vals[name])
		if name == "" || v == "" || !isInputType(d.S("type")) {
			continue
		}
		ud := []string{v}
		if isChoiceType(d.S("type")) {
			ud = allPicks(d, v)
		}
		data = append(data, FormAnswersJson{Name: name, UserData: ud})
	}
	cv.vis = ApplyConds(defs, data)
	return cv
}

// Shown - czy gość widział choć jedno z pól (nazwa albo etykieta pola, jak
// w .Field). Pole, którego formularz nie zna, jest widoczne; bez zgłoszenia
// (podgląd) widać wszystko.
func (i *FormFuncs) Shown(names ...string) bool {
	if i.FormID <= 0 || len(names) == 0 {
		return true
	}
	cv := i.condView()
	for _, n := range names {
		found := false
		for k, d := range cv.defs {
			if d.S("name") == n || (d.S("name") != "" && fieldLabel(d) == plainLabel(n)) {
				found = true
				if cv.vis[k] {
					return true
				}
				break
			}
		}
		if !found {
			return true
		}
	}
	return false
}

// Answers - odpowiedzi gościa z pól, które widział i wypełnił, z nagłówkami
// ich sekcji. Linijki rozdziela <br>, bo pastylka stoi w akapicie.
func (i *FormFuncs) Answers() template.HTML {
	cv := i.condView()
	esc := template.HTMLEscapeString
	var b strings.Builder
	header := ""
	for k, d := range cv.defs {
		t, name := d.S("type"), d.S("name")
		if t == "header" {
			header = ""
			if cv.vis[k] {
				header = fieldLabel(d)
			}
			continue
		}
		if !cv.vis[k] || name == "" || !isInputType(t) || isRoleName(name) {
			continue
		}
		v := strings.TrimSpace(cv.vals[name])
		if v == "" {
			continue
		}
		if header != "" {
			if b.Len() > 0 {
				b.WriteString("<br>")
			}
			b.WriteString("<strong>" + esc(header) + "</strong><br>")
			header = ""
		}
		v = strings.ReplaceAll(esc(v), "\n", "<br>")
		b.WriteString(esc(fieldLabel(d)) + ": <strong>" + v + "</strong><br>")
	}
	return template.HTML(b.String())
}

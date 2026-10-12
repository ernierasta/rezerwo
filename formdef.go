package main

// formdef.go: definicja pól formularza (JSON w formacie formBuildera w
// formtemplates.content) - czytanie, czyszczenie przy zapisie i przygotowanie
// do wyświetlenia gościowi.
//
// Format zostaje ten sam co w formBuilderze (rozdz. 7.2 HARMONIJKA.md), bo
// korzystają z niego formfields, raporty i FormFuncs w szablonach maili.
// Czytamy go tolerancyjnie: atrybuty, których nie znamy, przechodzą przez
// zapis bez zmian.

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"log"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
)

// Role pól zapisujemy dalej jako nazwy (zgodność z getBasicFields).
const (
	ROLENAME    = "name-REQUIRED"
	ROLESURNAME = "surname-REQUIRED"
	ROLEEMAIL   = "email-REQUIRED"
)

// FormDefField to jedno pole z definicji. Raw trzyma wszystkie atrybuty.
type FormDefField map[string]interface{}

func (f FormDefField) S(key string) string {
	switch v := f[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	}
	return ""
}

func (f FormDefField) B(key string) bool { return boolOf(f[key]) }

// ParseFormDef czyta definicję pól. Pusta treść to pusta lista.
func ParseFormDef(content string) ([]FormDefField, error) {
	content = strings.TrimSpace(content)
	if content == "" || content == "null" {
		return []FormDefField{}, nil
	}
	var out []FormDefField
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// --- czyszczenie HTML w etykietach -----------------------------------------

var labelPolicy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("b", "strong", "i", "em", "u", "br")
	p.AllowAttrs("href").OnElements("a")
	p.AllowStandardURLs()
	p.AllowURLSchemes("mailto", "http", "https", "tel")
	p.RequireNoFollowOnLinks(false)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return p
}()

var trailingJunk = regexp.MustCompile(`(?i)(\s|<br\s*/?>|&nbsp;)+$`)

// CleanLabel zostawia w etykiecie tylko pogrubienie, kursywę, podkreślenie,
// łamanie wiersza i linki. Wklejone style, spany i fonty znikają.
func CleanLabel(s string) string {
	// formBuilder zapisywał czasem HTML akapitów jako encje (&lt;b&gt;),
	// a formRender pokazywał je jako formatowanie - robimy to samo
	if !strings.Contains(s, "<") && strings.Contains(s, "&lt;") {
		s = html.UnescapeString(s)
	}
	s = labelPolicy.Sanitize(s)
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = trailingJunk.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// SafeLabel to etykieta gotowa do wstawienia w stronę gościa. Stare definicje
// mogą mieć dowolny HTML (formBuilder zapisywał contenteditable), więc czyścimy
// je także przy wyświetlaniu.
func SafeLabel(s string) template.HTML {
	return template.HTML(CleanLabel(s))
}

// isInputType mówi, czy pole zbiera odpowiedź (ma nazwę i trafia do formfields).
func isInputType(t string) bool {
	return t != "" && !isTypeIgnored(t)
}

// CleanFormDef przygotowuje definicję z designera do zapisu: czyści etykiety,
// sprawdza nazwy. Zwraca błąd, gdy definicja się nie nadaje.
func CleanFormDef(ff []FormDefField) ([]FormDefField, error) {
	seen := map[string]bool{}
	for i, f := range ff {
		t := f.S("type")
		if t == "" {
			return nil, fmt.Errorf("field %d: no type", i+1)
		}
		if l, ok := f["label"].(string); ok {
			f["label"] = CleanLabel(l)
		}
		if d, ok := f["description"].(string); ok {
			f["description"] = strings.TrimSpace(bluemonday.StripTagsPolicy().Sanitize(d))
		}
		if vals, ok := f["values"].([]interface{}); ok {
			for _, v := range vals {
				if vm, ok := v.(map[string]interface{}); ok {
					if l, ok := vm["label"].(string); ok {
						vm["label"] = plainLabel(l)
					}
					if s, ok := vm["value"].(string); ok {
						vm["value"] = strings.TrimSpace(s)
					}
				}
			}
		}
		if !isInputType(t) {
			delete(f, "name")
			continue
		}
		name := strings.TrimSpace(f.S("name"))
		if name == "" {
			return nil, fmt.Errorf("field %q: no name", plainLabel(f.S("label")))
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate field name %q", name)
		}
		seen[name] = true
		f["name"] = name
	}
	if err := checkConds(ff); err != nil {
		return nil, err
	}
	return ff, nil
}

// --- przygotowanie do wyświetlenia -----------------------------------------

// RenderOption to jedna pozycja listy, radia albo checkboxów.
type RenderOption struct {
	Label    string
	Value    string
	Selected bool
	ID       string
	Stat     *OptionStat // stan opcji przy polu (panel_place), patrz form_stats.go
	When     string      // opcja pola zależnego: wartość rodzica (formconds.go)
}

// RenderField to pole gotowe dla szablonu tmpl/n_formfield.html.
type RenderField struct {
	Type    string // text, number, textarea, select, radio-group, checkbox-group, date, header, paragraph, multiplierField
	Subtype string // text/email/tel, h1-h4, p
	Name    string
	ID      string
	Label   template.HTML
	Desc    string

	Required bool
	Inline   bool
	Other    bool
	Multiple bool
	Toggle   bool

	Placeholder string
	Value       string
	Min, Max    string
	Step        string
	MaxLength   string
	Rows        string

	Options []RenderOption

	// Wielokrotność: liczba × cena, z tekstem przed i po wyniku
	Price       string
	Before      string
	After       string
	IsMoney     bool // to pole jest kwotą do zapłaty (formtemplates.moneyfield)
	Width       int  // 1-12 (col-md-N), 12 = cały wiersz
	HeaderLevel string
	Stat        *PanelItem // stan pola przy polu (panel_place), patrz form_stats.go
	ShowIf      string     // warunek pokazania (własny albo sekcji) jako JSON, formconds.go
	DependsOn   string     // opcje zależne od pola o tej nazwie, formconds.go
}

// IsInput mówi, czy pole zbiera odpowiedź.
func (f RenderField) IsInput() bool { return isInputType(f.Type) }

// RenderRow to wiersz pól. Pola bez klasy row-N zajmują własny wiersz.
type RenderRow struct {
	Fields []RenderField
}

var (
	rowClass = regexp.MustCompile(`\brow-([^\s]+)`)
	colClass = regexp.MustCompile(`\bcol-(?:md-|sm-|lg-)?(\d{1,2})\b`)
)

// FieldLayout czyta z className formBuildera numer wiersza i szerokość.
func FieldLayout(className string) (row string, width int) {
	width = 12
	if m := rowClass.FindStringSubmatch(className); m != nil {
		row = m[1]
	}
	if m := colClass.FindStringSubmatch(className); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n >= 1 && n <= 12 {
			width = n
		}
	}
	return row, width
}

// BuildRenderRows zamienia definicję na wiersze pól. Tak jak formRender,
// wszystkie pola z tym samym row-N trafiają do jednego wiersza, w miejscu
// pierwszego z nich.
func BuildRenderRows(ff []FormDefField, moneyField string) []RenderRow {
	var rows []RenderRow
	rowIdx := map[string]int{}
	conds := FieldConds(ff)

	for i, d := range ff {
		f := renderField(d, i)
		if f.Type == "" || f.Type == "button" || f.Type == "hidden" {
			continue
		}
		if conds[i] != nil {
			f.ShowIf = conds[i].JSON()
		}
		f.DependsOn = dependsOn(d)
		if f.IsInput() && f.Name == "" {
			continue
		}
		f.IsMoney = moneyField != "" && f.Name == moneyField
		if f.IsMoney && f.Type == "number" && f.Min == "" {
			f.Min = "0" // kwota do zapłaty nie może być ujemna (checkValues)
		}

		row, width := FieldLayout(d.S("className"))
		f.Width = width
		if !f.IsInput() {
			f.Width = 12
			row = ""
		}
		if row != "" {
			if n, ok := rowIdx[row]; ok {
				rows[n].Fields = append(rows[n].Fields, f)
				continue
			}
			rowIdx[row] = len(rows)
		}
		rows = append(rows, RenderRow{Fields: []RenderField{f}})
	}
	return rows
}

// guestSubtypes to typy pola tekstowego, które przeglądarka pokazuje bez
// własnych tekstów (podpowiedzi, kalendarzy).
var guestSubtypes = map[string]bool{"text": true, "email": true, "tel": true}

// displayDate zmienia zapis rrrr-mm-dd na dd.mm.rrrr, tak jak gość wpisuje
// datę w polu daty; inne wartości zostają bez zmian.
func displayDate(s string) string {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return s
	}
	return t.Format("02.01.2006")
}

func renderField(d FormDefField, idx int) RenderField {
	f := RenderField{
		Type:        d.S("type"),
		Subtype:     d.S("subtype"),
		Name:        d.S("name"),
		Label:       SafeLabel(d.S("label")),
		Desc:        plainLabel(d.S("description")),
		Required:    d.B("required"),
		Inline:      d.B("inline"),
		Other:       d.B("other"),
		Multiple:    d.B("multiple"),
		Toggle:      d.B("toggle"),
		Placeholder: d.S("placeholder"),
		Value:       d.S("value"),
		Min:         d.S("min"),
		Max:         d.S("max"),
		Step:        d.S("step"),
		MaxLength:   d.S("maxlength"),
		Rows:        d.S("rows"),
		Price:       d.S("multipl"),
		Before:      d.S("pre"),
		After:       d.S("post"),
	}
	f.ID = fmt.Sprintf("f%d", idx)
	if f.Type == "header" {
		switch f.Subtype {
		case "h1", "h2":
			f.HeaderLevel = "h2"
		case "h3":
			f.HeaderLevel = "h3"
		default:
			f.HeaderLevel = "h4"
		}
	}
	if f.Type == "text" && !guestSubtypes[f.Subtype] {
		// inne typy z dawnego formBuildera (color, date, password...) rysuje
		// przeglądarka w swoim języku, a strona gościa ma być w języku formularza
		f.Subtype = "text"
	}
	if f.Type == "date" {
		f.Value = displayDate(f.Value)
	}
	if f.Name == ROLEEMAIL {
		f.Subtype = "email"
		f.Required = true
	}
	if f.Name == ROLENAME || f.Name == ROLESURNAME {
		f.Required = true
	}

	if vals, ok := d["values"].([]interface{}); ok {
		for j, v := range vals {
			vm, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			o := RenderOption{
				Label:    plainLabel(str(vm["label"])),
				Value:    str(vm["value"]),
				Selected: boolOf(vm["selected"]),
				ID:       fmt.Sprintf("f%d-%d", idx, j),
				When:     strings.TrimSpace(str(vm["when"])),
			}
			if o.Value == "" {
				o.Value = o.Label
			}
			f.Options = append(f.Options, o)
		}
	}
	return f
}

type FormFieldDef struct {
	Type    string `json:"type"`
	Display string `json:"label"`
	Name    string `json:"name"`
}

func (f FormFieldDef) GetType() string {
	return f.Type
}

func (f FormFieldDef) GetName() string {
	return f.Name
}

// updateFormFieldsInDB synchronizuje tabelę formfields z definicją formularza:
// nowe pola dodaje, istniejące (po nazwie) zmienia, usunięte kasuje
func updateFormFieldsInDB(templateID int64, current []FormFieldDef, db *DB) {
	previous, err := db.FormFieldGetAllForTmpl(templateID)
	if err != nil {
		log.Printf("updateFormFieldsInDB: error getting FormFields for template %d, %v", templateID, err)
	}
	old := map[string]bool{}
	for _, f := range filterFormFields(previous) {
		old[f.Name] = true
	}

	for _, c := range filterFormFields(current) {
		f := &FormField{Name: c.Name, Display: c.Display, Type: c.Type, FormTemplateID: templateID}
		if old[c.Name] {
			delete(old, c.Name)
			if err := db.FormFieldModByName(f); err != nil {
				log.Printf("updateFormFieldsInDB: error updating field %q, %v", f.Name, err)
			}
		} else if _, err := db.FormFieldAdd(f); err != nil {
			log.Printf("updateFormFieldsInDB: error adding field %q, %v", f.Name, err)
		}
	}

	// pola, których nie ma już w definicji
	for name := range old {
		if err := db.FormFieldDelByName(name, templateID); err != nil {
			log.Printf("updateFormFieldsInDB: error deleting old formfield %q, templateID %d, %v", name, templateID, err)
		}
	}
}

type FormFieldType interface {
	FormField | FormFieldDef
	GetName() string
	GetType() string
}

// filterFormFields is generic function which works
// for both formfield types
func filterFormFields[L FormFieldType](l []L) []L {
	var ret []L
	for i := range l {
		if l[i].GetName() == "" {
			continue
		}
		if !isTypeIgnored(l[i].GetType()) {
			ret = append(ret, l[i])
		}
	}
	return ret
}

func isTypeIgnored(t string) bool {
	return slices.Contains([]string{"header", "paragraph", "button", "hidden"}, t)
}

type FormAnswersJson struct {
	Name     string   `json:"name"`
	UserData []string `json:"userData"`
}

func makeSureIsHTML(s string) string {
	if strings.Contains(s, "<html") {
		return s
	}
	return mailHTMLHead + s + "</html>"
}

// getBasicFields - imię, nazwisko i e-mail z odpowiedzi (pola o stałych
// nazwach); formularz anonimowy dostaje "- Anon -" i numer zgłoszenia
func getBasicFields(j []FormAnswersJson, uniqID string) (string, string, string) {
	v := map[string]string{}
	for _, a := range j {
		if len(a.UserData) > 0 {
			v[a.Name] = a.UserData[0]
		}
	}
	name, surname, email := v["name-REQUIRED"], v["surname-REQUIRED"], v["email-REQUIRED"]
	if surname == "" {
		name, surname = "- Anon -", uniqID
	}
	return name, surname, email
}

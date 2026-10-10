package main

// form_stats.go: zestawienia odpowiedzi formularza i panel informacyjny.
//
// Stare funkcje szablonów (.Sum, .SumAlfa, .SumStrings, .SumOnly w main.go)
// zgadują rodzaj danych po samych odpowiedziach: gdy wszystkie są liczbami,
// sumują, w przeciwnym razie liczą wystąpienia. Stąd kłopoty: lista klas
// "1", "2", "3" dawała sumę numerów, jedna pusta odpowiedź w polu liczbowym
// przełączała je na liczenie napisów, a opcja z przecinkiem w nazwie liczyła
// się jako dwie. Tutaj rodzaj zestawienia wynika z rodzaju pola w definicji.
//
// Panel informacyjny ma cztery tryby (formtemplates.infopanel_mode):
//   none   - bez panelu
//   auto   - buduje się sam ze wszystkich pól z liczbami i opcjami
//   fields - tylko pola z włączonym "Pokaż w panelu" (atrybut panel w JSON)
//   text   - własny tekst admina (szablon Go, jak dotąd; w edytorze pastylki)
//
// Ustawienia pola w definicji (dopisuje je designer pól):
//   panel       true/false - pokaż/ukryj w panelu (brak = domyślnie dla trybu)
//   panel_goal  cel, np. 10 osób do szatni - rysujemy pasek postępu
//   panel_only  licz tylko tę opcję (dawne .SumOnly "pole" "Tak")
//   panel_place "field" - pokaż stan przy samym polu zamiast w panelu
//               (brak = w panelu)

import (
	"bytes"
	"html/template"
	"log"
	"sort"
	"strconv"
	"strings"
)

const (
	PANELNONE   = "none"
	PANELAUTO   = "auto"
	PANELFIELDS = "fields"
	PANELTEXT   = "text"

	// PLACEFIELD - panel_place pola: stan pokazany przy polu, nie w panelu
	PLACEFIELD = "field"
)

// panelMode zwraca tryb panelu (stare formularze: tekst).
func panelMode(t FormTemplate) string {
	switch t.InfoPanelMode {
	case PANELNONE, PANELAUTO, PANELFIELDS:
		return t.InfoPanelMode
	}
	return PANELTEXT
}

// OptionStat - ile razy wybrano opcję.
type OptionStat struct {
	Label string
	Count int64
	Other bool   // odpowiedź spoza listy opcji ("Inne: ...")
	When  string // pole zależne: grupa opcji (wartość rodzica)
	Goal  int64
	Pct   int
}

// Done mówi, czy opcja osiągnęła cel.
func (o OptionStat) Done() bool { return o.Goal > 0 && o.Count >= o.Goal }

// FieldStat - zestawienie odpowiedzi jednego pola.
type FieldStat struct {
	Name  string
	Label string
	Type  string
	// Kind: "sum" (liczby, ilość × cena), "options" (lista, radio, checkboxy),
	// "filled" (tekst - ile osób wypełniło)
	Kind    string
	Sum     float64
	Money   float64 // ilość × cena: łączna kwota
	Filled  int64
	Options []OptionStat
}

// formAnswers to wszystkie niepuste odpowiedzi formularza wg nazwy pola.
type formAnswers map[string][]string

func loadFormAnswers(db *DB, tmplID int64) formAnswers {
	rows, err := db.FormAnswerValuesForTemplate(tmplID)
	if err != nil {
		log.Printf("loadFormAnswers %d: %v", tmplID, err)
	}
	out := formAnswers{}
	for _, r := range rows {
		v := strings.TrimSpace(r.Value)
		if v == "" {
			continue
		}
		out[r.Name] = append(out[r.Name], v)
	}
	return out
}

// isSummable mówi, czy pole ma sens w automatycznym panelu.
func isSummable(t string) bool {
	switch t {
	case "number", "multiplierField", "select", "radio-group", "checkbox-group":
		return true
	}
	return false
}

func parseNumber(s string) (float64, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), " ", "")
	s = strings.ReplaceAll(s, ",", ".")
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

// fieldStat liczy zestawienie pola na podstawie jego definicji.
func fieldStat(d FormDefField, answers []string) FieldStat {
	return fieldStatDep(d, answers, nil)
}

// fieldStatDep jak fieldStat; parents (równoległe do answers) to wybory
// rodzica pola zależnego (formconds.go) - opcje liczą się wtedy osobno
// w każdej grupie "when".
func fieldStatDep(d FormDefField, answers []string, parents [][]string) FieldStat {
	st := FieldStat{
		Name:  d.S("name"),
		Label: plainLabel(d.S("label")),
		Type:  d.S("type"),
	}
	switch st.Type {
	case "number":
		st.Kind = "sum"
		for _, a := range answers {
			if f, ok := parseNumber(a); ok {
				st.Sum += f
				if f != 0 {
					st.Filled++
				}
			}
		}
	case "multiplierField":
		st.Kind = "sum"
		for _, a := range answers {
			am, _, tot := ConvertMultiplicationToAmmountMultiplTotal(a)
			if n, ok := parseNumber(am); ok {
				st.Sum += n
				if n != 0 {
					st.Filled++
				}
			}
			if m, ok := parseNumber(tot); ok {
				st.Money += m
			}
		}
	case "select", "radio-group", "checkbox-group":
		st.Kind = "options"
		st.Options = countOptions(d, answers, parents)
	default:
		st.Kind = "filled"
		st.Filled = int64(len(answers))
	}
	return st
}

// choiceOpt - opcja pola wyboru z definicji.
type choiceOpt struct{ label, value, when string }

// choiceOptions zwraca opcje pola w kolejności z definicji. Pole zależne może
// mieć tę samą wartość w kilku grupach "when" - wtedy jest kilka wpisów.
func choiceOptions(d FormDefField) []choiceOpt {
	var opts []choiceOpt
	vals, _ := d["values"].([]interface{})
	for _, v := range vals {
		vm, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		o := choiceOpt{label: plainLabel(str(vm["label"])), value: strings.TrimSpace(str(vm["value"])), when: strings.TrimSpace(str(vm["when"]))}
		if o.value == "" {
			o.value = o.label
		}
		opts = append(opts, o)
	}
	return opts
}

// isMultiChoice mówi, czy odpowiedź pola może mieć kilka wartości ("A, B").
func isMultiChoice(d FormDefField) bool {
	return d.S("type") == "checkbox-group" || (d.S("type") == "select" && d.B("multiple"))
}

// answerPicks dzieli zapisaną odpowiedź pola wyboru na wybrane wartości.
// Checkboxy są zapisane jako "A, B", więc najpierw szukamy całych znanych
// opcji (także tych z przecinkiem w nazwie, dłuższe najpierw, żeby "Tak,
// chętnie" nie zostało policzone jako "Tak"); reszta to wpisy "Inne".
// Zwraca znane wartości (known) i resztę (other).
func answerPicks(d FormDefField, a string) (known, other []string) {
	a = strings.TrimSpace(a)
	if a == "" {
		return nil, nil
	}
	var values []string
	seen := map[string]bool{}
	for _, o := range choiceOptions(d) {
		if o.value != "" && !seen[o.value] {
			seen[o.value] = true
			values = append(values, o.value)
		}
	}
	if !isMultiChoice(d) {
		if seen[a] {
			return []string{a}, nil
		}
		return nil, []string{a}
	}
	sort.SliceStable(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	rest := ", " + a + ", "
	for _, v := range values {
		if strings.Contains(rest, ", "+v+", ") {
			known = append(known, v)
			rest = strings.Replace(rest, ", "+v+", ", ", ", 1)
		}
	}
	for _, p := range strings.Split(rest, ", ") {
		if p = strings.TrimSpace(p); p != "" {
			other = append(other, p)
		}
	}
	return known, other
}

// allPicks to answerPicks jako jedna lista (znane i "Inne").
func allPicks(d FormDefField, a string) []string {
	k, o := answerPicks(d, a)
	return append(k, o...)
}

// countOptions liczy wybory opcji w kolejności z definicji, a odpowiedzi
// spoza listy jako "Inne". Z parents (pole zależne) każda grupa "when" ma
// własne liczniki: opcja liczy się w grupie, gdy rodzic ma wartość "when".
func countOptions(d FormDefField, answers []string, parents [][]string) []OptionStat {
	opts := choiceOptions(d)
	grouped := parents != nil
	if !grouped {
		// bez rodzica ta sama wartość w kilku grupach to jedna opcja
		seen := map[string]bool{}
		var uniq []choiceOpt
		for _, o := range opts {
			if !seen[o.value] {
				seen[o.value] = true
				o.when = ""
				uniq = append(uniq, o)
			}
		}
		opts = uniq
	}

	counts := make([]int64, len(opts))
	other := map[string]int64{}
	var otherOrder []string
	for i, a := range answers {
		known, rest := answerPicks(d, a)
		for _, v := range known {
			for j, o := range opts {
				if o.value != v {
					continue
				}
				if grouped && o.when != "" && (i >= len(parents) || !anyIn([]string{o.when}, parents[i])) {
					continue
				}
				counts[j]++
			}
		}
		for _, p := range rest {
			if _, ok := other[p]; !ok {
				otherOrder = append(otherOrder, p)
			}
			other[p]++
		}
	}

	out := make([]OptionStat, 0, len(opts)+len(other))
	for i, o := range opts {
		out = append(out, OptionStat{Label: o.label, Count: counts[i], When: o.when})
	}
	for _, p := range otherOrder {
		out = append(out, OptionStat{Label: p, Count: other[p], Other: true})
	}
	return out
}

// OptionCount zwraca liczbę wyborów jednej opcji (po etykiecie albo wartości).
// Pole zależne ma opcję w każdej grupie osobno - liczy się suma.
func (st FieldStat) OptionCount(option string) int64 {
	var n int64
	for _, o := range st.Options {
		if o.Label == option {
			n += o.Count
		}
	}
	return n
}

// Value to główna liczba zestawienia jako tekst.
func (st FieldStat) Value() string {
	switch st.Kind {
	case "sum":
		return formatNumber(st.Sum)
	case "filled":
		return strconv.FormatInt(st.Filled, 10)
	}
	var n int64
	for _, o := range st.Options {
		n += o.Count
	}
	return strconv.FormatInt(n, 10)
}

func formatNumber(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', 2, 64)
}

// --- panel informacyjny ----------------------------------------------------

// PanelItem to jeden wiersz panelu: nagłówek, liczba albo lista opcji.
type PanelItem struct {
	Header  template.HTML // nagłówek sekcji formularza
	Label   string
	Value   string
	Unit    string // np. "Kč" przy kwocie
	Money   string // ilość × cena: łączna kwota
	Goal    int64
	Pct     int
	Done    bool // cel osiągnięty
	Options []OptionStat
	// Groups - pole zależne: opcje w grupach wg wyboru rodzica
	Groups []OptionGroup
	// Compact - krótkie opcje (klasy "1A", "2B") jako kafelki, a nie lista
	Compact bool
	// Only - "licz tylko opcję": stan tej jednej opcji (przy polu)
	Only *OptionStat
}

// OptionGroup - opcje pola zależnego dostępne przy jednym wyborze rodzica
// (Label pusty: opcje wspólne i "Inne").
type OptionGroup struct {
	Label   string
	Options []OptionStat
}

// groupOptions układa opcje pola zależnego w grupy "when" w kolejności
// pierwszego wystąpienia; etykiety grup to etykiety opcji rodzica.
func groupOptions(opts []OptionStat, parent FormDefField) []OptionGroup {
	labels := map[string]string{}
	for _, o := range choiceOptions(parent) {
		if _, ok := labels[o.value]; !ok {
			labels[o.value] = o.label
		}
	}
	var out []OptionGroup
	idx := map[string]int{}
	var common []OptionStat
	for _, o := range opts {
		if o.When == "" {
			common = append(common, o)
			continue
		}
		i, ok := idx[o.When]
		if !ok {
			l := labels[o.When]
			if l == "" {
				l = o.When
			}
			i = len(out)
			idx[o.When] = i
			out = append(out, OptionGroup{Label: l})
		}
		out[i].Options = append(out[i].Options, o)
	}
	if len(common) > 0 {
		out = append(out, OptionGroup{Options: common})
	}
	return out
}

// depAnswers zbiera odpowiedzi pola zależnego i wybory rodzica z całych
// zgłoszeń (równoległe listy dla fieldStatDep).
func depAnswers(recs map[int64]map[string]string, name string, parent FormDefField) ([]string, [][]string) {
	ids := make([]int64, 0, len(recs))
	for id := range recs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
	var answers []string
	var parents [][]string
	for _, id := range ids {
		v := strings.TrimSpace(recs[id][name])
		if v == "" {
			continue
		}
		answers = append(answers, v)
		parents = append(parents, allPicks(parent, recs[id][parent.S("name")]))
	}
	return answers, parents
}

// loadFormRecords zwraca odpowiedzi wg zgłoszenia: id -> pole -> wartość.
func loadFormRecords(db *DB, tmplID int64) map[int64]map[string]string {
	rows, err := db.FormAnswerRecordsForTemplate(tmplID)
	if err != nil {
		log.Printf("loadFormRecords %d: %v", tmplID, err)
	}
	out := map[int64]map[string]string{}
	for _, r := range rows {
		if out[r.FormID] == nil {
			out[r.FormID] = map[string]string{}
		}
		out[r.FormID][r.Name] = r.Value
	}
	return out
}

type PanelVars struct {
	Submissions int64
	Items       []PanelItem
	// Fields - pola ze stanem pokazanym przy polu (panel_place), wg nazwy
	Fields map[string]PanelItem
	Empty  bool // tryb "wybrane pola", ale nic nie wybrano
}

// fieldInPanel mówi, czy pole trafia do panelu w danym trybie.
func fieldInPanel(d FormDefField, mode string) bool {
	t := d.S("type")
	name := d.S("name")
	if !isInputType(t) || name == ROLENAME || name == ROLESURNAME || name == ROLEEMAIL {
		return false
	}
	flag, set := d["panel"]
	switch mode {
	case PANELAUTO:
		if set {
			return boolOf(flag)
		}
		return isSummable(t)
	case PANELFIELDS:
		return set && boolOf(flag)
	}
	return false
}

func pct(n, goal int64) int {
	if goal <= 0 {
		return 0
	}
	p := int(n * 100 / goal)
	if p > 100 {
		p = 100
	}
	return p
}

// BuildPanel składa panel z definicji pól i odpowiedzi.
func BuildPanel(db *DB, user User, templ FormTemplate, mode string) PanelVars {
	defs, err := ParseFormDef(templ.Content.String)
	if err != nil {
		log.Printf("BuildPanel %d: %v", templ.ID, err)
	}
	answers := loadFormAnswers(db, templ.ID)
	n, err := db.FormGetAmmount(user.ID, templ.ID)
	if err != nil {
		log.Printf("BuildPanel %d: count: %v", templ.ID, err)
	}
	v := PanelVars{Submissions: n, Fields: map[string]PanelItem{}}
	currency := currencySign(formCurrency(db, user, templ))

	byName := map[string]FormDefField{}
	for _, d := range defs {
		if n := d.S("name"); n != "" {
			byName[n] = d
		}
	}
	var recs map[int64]map[string]string // całe zgłoszenia, tylko dla pól zależnych

	var pendingHeader template.HTML
	for _, d := range defs {
		if d.S("type") == "header" {
			pendingHeader = SafeLabel(d.S("label"))
			continue
		}
		if !fieldInPanel(d, mode) {
			continue
		}
		atField := d.S("panel_place") == PLACEFIELD
		if pendingHeader != "" && !atField {
			v.Items = append(v.Items, PanelItem{Header: pendingHeader})
			pendingHeader = ""
		}
		st := fieldStat(d, answers[d.S("name")])
		parent, dep := byName[dependsOn(d)]
		if dep {
			if recs == nil {
				recs = loadFormRecords(db, templ.ID)
			}
			ans, par := depAnswers(recs, d.S("name"), parent)
			st = fieldStatDep(d, ans, par)
		}
		goal := intOf(d["panel_goal"])
		it := PanelItem{Label: st.Label, Value: st.Value(), Goal: goal}

		if only := d.S("panel_only"); only != "" && st.Kind == "options" {
			c := st.OptionCount(only)
			it.Label = st.Label + ": " + only
			it.Value = strconv.FormatInt(c, 10)
			it.Pct, it.Done = pct(c, goal), goal > 0 && c >= goal
			it.Only = &OptionStat{Label: only, Count: c, Goal: goal, Pct: it.Pct}
		} else if st.Kind == "options" {
			short := true
			for _, o := range st.Options {
				if o.Count == 0 && o.Other {
					continue
				}
				o.Goal = goal
				o.Pct = pct(o.Count, goal)
				it.Options = append(it.Options, o)
				if len([]rune(o.Label)) > 5 {
					short = false
				}
			}
			it.Compact = short && goal == 0 && len(it.Options) >= 5
			if dep {
				it.Groups = groupOptions(it.Options, parent)
			}
		} else if st.Kind == "filled" {
			it.Unit = T(formLang(templ), "panel.filled")
			it.Pct, it.Done = pct(st.Filled, goal), goal > 0 && st.Filled >= goal
		} else {
			it.Pct, it.Done = pct(int64(st.Sum), goal), goal > 0 && int64(st.Sum) >= goal
			if st.Type == "multiplierField" && st.Money > 0 {
				it.Money = formatNumber(st.Money) + " " + currency
			}
		}
		if atField {
			v.Fields[st.Name] = it
			continue
		}
		v.Items = append(v.Items, it)
	}
	v.Empty = mode == PANELFIELDS && len(v.Items) == 0 && len(v.Fields) == 0
	return v
}

// AttachFieldStats dopisuje do pól formularza gościa stan pokazywany przy
// polu: przy polach z opcjami liczby przy opcjach, przy pozostałych wiersz
// pod polem.
func AttachFieldStats(rows []RenderRow, items map[string]PanelItem) {
	for r := range rows {
		for i := range rows[r].Fields {
			f := &rows[r].Fields[i]
			it, ok := items[f.Name]
			if !ok {
				continue
			}
			if it.Only == nil && len(f.Options) == 0 {
				f.Stat = &it
				continue
			}
			for j := range f.Options {
				o := &f.Options[j]
				if it.Only != nil {
					if o.Label == it.Only.Label {
						o.Stat = it.Only
					}
					continue
				}
				for k := range it.Options {
					w := it.Options[k].When
					if it.Options[k].Label == o.Label && !it.Options[k].Other && (w == "" || w == o.When) {
						o.Stat = &it.Options[k]
						break
					}
				}
			}
		}
	}
}

func currencySign(c string) string {
	switch c {
	case "CZK":
		return "Kč"
	case "EUR":
		return "€"
	case "PLN":
		return "zł"
	}
	return c
}

// RenderInfoPanel zwraca gotowy HTML panelu informacyjnego formularza (pusty,
// gdy panelu nie ma). Tryb "text" wykonuje tekst admina jako szablon z
// funkcjami formularza, jak dotąd.
func RenderInfoPanel(db *DB, user User, templ FormTemplate, mode string) template.HTML {
	lang := formLang(templ)
	switch mode {
	case PANELNONE:
		return ""
	case PANELAUTO, PANELFIELDS:
		return renderPanelVars(lang, BuildPanel(db, user, templ, mode))
	}

	if strings.TrimSpace(reallyEmpty(templ.InfoPanel.String)) == "" {
		return ""
	}
	var buf bytes.Buffer
	funcs := &FormFuncs{DB: db, User: user, Template: templ}
	if t, err := template.New("infopanel").Parse(templ.InfoPanel.String); err != nil {
		log.Printf("RenderInfoPanel: parse: %v", err)
	} else if err := t.Execute(&buf, funcs); err != nil {
		log.Printf("RenderInfoPanel: exec: %v", err)
	}
	return template.HTML(reallyEmpty(buf.String()))
}

// renderPanelVars rysuje panel zbudowany przez BuildPanel.
func renderPanelVars(lang string, pv PanelVars) template.HTML {
	t, err := template.New("n_infopanel.html").Funcs(TmplFuncs(lang)).ParseFiles("tmpl/n_infopanel.html")
	if err != nil {
		log.Printf("RenderInfoPanel: %v", err)
		return ""
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "infopanel", pv); err != nil {
		log.Printf("RenderInfoPanel: %v", err)
	}
	return template.HTML(buf.String())
}

// --- nowe funkcje szablonów (tryb "własny tekst") --------------------------
//
// W edytorze to pastylki; w bazie akcje szablonu. Stare .Sum* działają dalej.

// fieldStatByName liczy zestawienie pola o podanej nazwie (albo etykiecie).
func (i *FormFuncs) fieldStatByName(name string) (FieldStat, bool) {
	defs, err := ParseFormDef(i.Template.Content.String)
	if err != nil {
		return FieldStat{}, false
	}
	for _, d := range defs {
		if d.S("name") == name || (d.S("name") != "" && plainLabel(d.S("label")) == name) {
			if i.answers == nil {
				i.answers = loadFormAnswers(i.DB, i.Template.ID)
			}
			return fieldStat(d, i.answers[d.S("name")]), true
		}
	}
	return FieldStat{}, false
}

// Submissions - ile osób wysłało formularz (dawne {{.Sum "forms"}}).
func (i *FormFuncs) Submissions() int64 {
	n, err := i.DB.FormGetAmmount(i.User.ID, i.Template.ID)
	if err != nil {
		log.Printf("Submissions: %v", err)
	}
	return n
}

// Summary - zestawienie pola wg jego rodzaju: suma liczb, ilość sztuk,
// lista opcji z liczbą wyborów albo liczba wypełnień pola tekstowego.
func (i *FormFuncs) Summary(name string) template.HTML {
	st, ok := i.fieldStatByName(name)
	if !ok {
		return ""
	}
	if st.Kind != "options" {
		return template.HTML(template.HTMLEscapeString(st.Value()))
	}
	var b strings.Builder
	b.WriteString(`<ul class="sum-list">`)
	for _, o := range st.Options {
		if o.Other && o.Count == 0 {
			continue
		}
		b.WriteString(`<li class="sum-list-el">`)
		b.WriteString(template.HTMLEscapeString(o.Label))
		b.WriteString(": <b>")
		b.WriteString(strconv.FormatInt(o.Count, 10))
		b.WriteString("</b></li>")
	}
	b.WriteString("</ul>")
	return template.HTML(b.String())
}

// OptionCount - ile razy wybrano jedną opcję pola (dawne .SumOnly).
func (i *FormFuncs) OptionCount(name, option string) int64 {
	st, ok := i.fieldStatByName(name)
	if !ok {
		return 0
	}
	return st.OptionCount(option)
}

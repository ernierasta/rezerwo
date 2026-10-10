package main

// form_report.go: odpowiedzi formularza z sekcjami i warunkami (formconds.go).
//
// Ukryte pola są przy wysyłce czyszczone, więc w danych puste pole wygląda tak
// samo, gdy gość je pominął i gdy go w ogóle nie widział. Tabela odpowiedzi
// liczy więc widoczność każdego zgłoszenia jeszcze raz (ApplyConds) i wysyła:
//   na  - pola, których gość nie widział ("nie dotyczy")
//   out - pola ukryte dzisiejszymi warunkami, ale z odpowiedzią (zgłoszenia
//         sprzed dodania warunku) - pokazujemy je z oznaczeniem, nie gubimy
//   picks - wybrane opcje pól wyboru (checkboxy rozdzielone), do filtra gałęzi
//         i zestawienia na stronie odpowiedzi (js/n_answers.js)

import (
	"log"
	"strings"
)

type nAnswerOpt struct {
	Value string `json:"value"`
	Label string `json:"label"`
	When  string `json:"when,omitempty"`
}

type nAnswerCol struct {
	Key     string       `json:"key"`
	Label   string       `json:"label"`
	Type    string       `json:"type"`
	Section int          `json:"section"`          // indeks w sections, -1 = przed pierwszym nagłówkiem
	Parent  string       `json:"parent,omitempty"` // depends_on
	Cond    *Cond        `json:"cond,omitempty"`   // własny warunek pola albo jego sekcji
	Options []nAnswerOpt `json:"options,omitempty"`
	Role    bool         `json:"role,omitempty"` // imię, nazwisko, e-mail
}

type nAnswerSection struct {
	Label string `json:"label"`
	Cond  *Cond  `json:"cond,omitempty"`
}

// answerSchema - kolumny tabeli odpowiedzi wg definicji formularza.
type answerSchema struct {
	Cols     []nAnswerCol
	Sections []nAnswerSection
	ByID     map[int64]string // id pola (formfields) -> nazwa
	Defs     []FormDefField
	defOf    map[string]FormDefField
}

// loadAnswerSchema składa kolumny w kolejności formularza, pogrupowane
// w sekcje (nagłówki). Pola, które są w bazie, a zniknęły z definicji (stare
// odpowiedzi), trafiają na koniec do osobnej sekcji.
func loadAnswerSchema(db *DB, templ FormTemplate, lang string) answerSchema {
	s := answerSchema{ByID: map[int64]string{}, defOf: map[string]FormDefField{}}
	defs, err := ParseFormDef(templ.Content.String)
	if err != nil {
		log.Printf("loadAnswerSchema %d: %v", templ.ID, err)
	}
	s.Defs = defs
	ff, err := db.FormFieldGetAllForTmpl(templ.ID)
	if err != nil {
		log.Printf("loadAnswerSchema %d: fields: %v", templ.ID, err)
	}
	ff = filterFormFields(ff)
	stored := map[string]bool{}
	for _, f := range ff {
		s.ByID[f.ID] = f.Name
		stored[f.Name] = true
	}

	conds := FieldConds(defs)
	section := -1
	inDef := map[string]bool{}
	for i, d := range defs {
		t := d.S("type")
		if t == "header" {
			section = len(s.Sections)
			s.Sections = append(s.Sections, nAnswerSection{Label: plainLabel(d.S("label")), Cond: condOf(d)})
			continue
		}
		name := d.S("name")
		if name == "" || !isInputType(t) || !stored[name] {
			continue
		}
		inDef[name] = true
		s.defOf[name] = d
		c := nAnswerCol{Key: name, Label: plainLabel(d.S("label")), Type: t, Section: section,
			Parent: dependsOn(d), Cond: conds[i], Role: isRoleName(name)}
		if isChoiceType(t) {
			for _, o := range choiceOptions(d) {
				c.Options = append(c.Options, nAnswerOpt{Value: o.value, Label: o.label, When: o.when})
			}
		}
		s.Cols = append(s.Cols, c)
	}

	old := -1
	for _, f := range ff {
		if inDef[f.Name] {
			continue
		}
		if old < 0 {
			old = len(s.Sections)
			s.Sections = append(s.Sections, nAnswerSection{Label: T(lang, "answers.old_fields")})
		}
		s.Cols = append(s.Cols, nAnswerCol{Key: f.Name, Label: plainLabel(f.Display), Type: f.Type, Section: old})
	}
	return s
}

// rowVisibility liczy dla jednego zgłoszenia pola "nie dotyczy", pola "poza
// warunkiem" i wybory pól wyboru.
func (s answerSchema) rowVisibility(values map[string]string) (na, out []string, picks map[string][]string) {
	data := make([]FormAnswersJson, 0, len(s.defOf))
	for name, d := range s.defOf {
		v := strings.TrimSpace(values[name])
		if v == "" {
			continue
		}
		var ud []string
		if isChoiceType(d.S("type")) {
			ud = allPicks(d, v)
			if picks == nil {
				picks = map[string][]string{}
			}
			picks[name] = ud
		} else {
			ud = []string{v}
		}
		data = append(data, FormAnswersJson{Name: name, UserData: append([]string(nil), ud...)})
	}
	after := map[string]int{}
	for i, d := range data {
		after[d.Name] = i
	}

	vis := ApplyConds(s.Defs, data)
	for i, d := range s.Defs {
		name := d.S("name")
		if _, ok := s.defOf[name]; !ok || !isInputType(d.S("type")) {
			continue
		}
		had := strings.TrimSpace(values[name]) != ""
		switch {
		case !vis[i] && had:
			out = append(out, name)
		case !vis[i]:
			na = append(na, name)
		case had && len(nonEmpty(data[after[name]].UserData)) == 0:
			// pole zależne z opcją, która nie pasuje do wyboru rodzica
			out = append(out, name)
		}
	}
	return na, out, picks
}

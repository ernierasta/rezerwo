package main

// formconds.go: pola warunkowe i zależne opcje w formularzach.
//
// Atrybuty w definicji pola (formtemplates.content):
//   show_if     {"field": "klasa", "values": ["1A", "1B"]} - pole widać, gdy
//               pole "klasa" ma jedną z tych wartości. Warunek na nagłówku
//               obejmuje wszystkie pola do następnego nagłówka (sekcja),
//               własny warunek pola zastępuje warunek sekcji.
//   depends_on  "kategoria" - opcje pola zależą od wyboru w polu "kategoria":
//               opcja z "when": "zwierzak" jest tylko przy tym wyborze, opcja
//               bez "when" przy każdym. Pole jest ukryte, dopóki rodzic jest
//               pusty.
// Warunki odwołują się tylko do pól wyboru wyżej w formularzu (pilnuje tego
// CleanFormDef), więc widoczność liczy się jednym przejściem od góry. To samo
// liczy strona gościa w js/n_formconds.js. Imię, nazwisko i e-mail są zawsze
// widoczne.

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Cond - warunek pokazania pola: pole Field ma jedną z wartości Values.
type Cond struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

func (c *Cond) JSON() string {
	b, _ := json.Marshal(c)
	return string(b)
}

func condOf(d FormDefField) *Cond {
	m, ok := d["show_if"].(map[string]interface{})
	if !ok {
		return nil
	}
	c := &Cond{Field: strings.TrimSpace(str(m["field"])), Values: []string{}}
	if vals, ok := m["values"].([]interface{}); ok {
		for _, v := range vals {
			if s := str(v); s != "" {
				c.Values = append(c.Values, s)
			}
		}
	}
	if c.Field == "" {
		return nil
	}
	return c
}

func isRoleName(name string) bool {
	return name == ROLENAME || name == ROLESURNAME || name == ROLEEMAIL
}

func isChoiceType(t string) bool {
	return t == "select" || t == "radio-group" || t == "checkbox-group"
}

func dependsOn(d FormDefField) string {
	if !isChoiceType(d.S("type")) {
		return ""
	}
	return strings.TrimSpace(d.S("depends_on"))
}

// FieldConds zwraca warunek każdego pola: własny albo jego sekcji.
func FieldConds(defs []FormDefField) []*Cond {
	out := make([]*Cond, len(defs))
	var section *Cond
	for i, d := range defs {
		c := condOf(d)
		if d.S("type") == "header" {
			section = c
		}
		if c == nil {
			c = section
		}
		if isRoleName(d.S("name")) {
			c = nil
		}
		out[i] = c
	}
	return out
}

func anyIn(vals, set []string) bool {
	for _, v := range vals {
		for _, s := range set {
			if v == s {
				return true
			}
		}
	}
	return false
}

// eachOption woła fn dla każdej opcji pola: wartość i "when".
func eachOption(d FormDefField, fn func(value, when string)) {
	vals, _ := d["values"].([]interface{})
	for _, v := range vals {
		vm, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		val := str(vm["value"])
		if val == "" {
			val = plainLabel(str(vm["label"]))
		}
		fn(val, strings.TrimSpace(str(vm["when"])))
	}
}

// optionAllowed mówi, czy odpowiedź pola zależnego pasuje do wyboru rodzica.
// Wartości spoza opcji (tekst "Inne") przechodzą.
func optionAllowed(d FormDefField, value string, parent []string) bool {
	known, ok := false, false
	eachOption(d, func(v, when string) {
		if v != value {
			return
		}
		known = true
		if when == "" || anyIn([]string{when}, parent) {
			ok = true
		}
	})
	return ok || !known
}

// hasOptionsFor mówi, czy pole zależne ma co pokazać przy wyborze rodzica.
func hasOptionsFor(d FormDefField, parent []string) bool {
	if d.B("other") {
		return true
	}
	ok := false
	eachOption(d, func(_, when string) {
		if when == "" || anyIn([]string{when}, parent) {
			ok = true
		}
	})
	return ok
}

func nonEmpty(vals []string) []string {
	var out []string
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

// ApplyConds liczy, które pola gość widzi przy danych odpowiedziach, i czyści
// odpowiedzi: ukryte pola są puste, w polach zależnych zostają tylko opcje
// pasujące do wyboru rodzica. Zwraca widoczność pól (indeksy jak w defs).
func ApplyConds(defs []FormDefField, data []FormAnswersJson) []bool {
	byName := map[string]int{}
	for i, d := range data {
		byName[d.Name] = i
	}
	conds := FieldConds(defs)
	vis := make([]bool, len(defs))
	got := map[string][]string{} // odpowiedzi widocznych pól
	for i, d := range defs {
		name := d.S("name")
		var ans []string
		di, has := byName[name]
		if has && name != "" {
			ans = data[di].UserData
		}
		v := conds[i] == nil || anyIn(got[conds[i].Field], conds[i].Values)
		if p := dependsOn(d); v && p != "" {
			pv := got[p]
			if len(pv) == 0 || !hasOptionsFor(d, pv) {
				v = false
			} else {
				var keep []string
				for _, a := range ans {
					if optionAllowed(d, a, pv) {
						keep = append(keep, a)
					}
				}
				ans = keep
			}
		}
		vis[i] = v
		if name == "" || !isInputType(d.S("type")) {
			continue
		}
		if !v {
			ans = nil
		}
		if has {
			data[di].UserData = ans
		}
		if v {
			got[name] = nonEmpty(ans)
		}
	}
	return vis
}

// checkConds sprawdza warunki przy zapisie definicji: odnoszą się do pola
// wyboru wyżej w formularzu i mają co najmniej jedną wartość. Warunki pól
// z rolą (imię, nazwisko, e-mail) znikają.
func checkConds(ff []FormDefField) error {
	choice := map[string]bool{}
	for _, f := range ff {
		label := plainLabel(f.S("label"))
		if _, ok := f["show_if"]; ok {
			c := condOf(f)
			switch {
			case c == nil || isRoleName(f.S("name")):
				delete(f, "show_if")
			case !choice[c.Field]:
				return fmt.Errorf("field %q: condition refers to %q, which is not a choice field above it", label, c.Field)
			case len(c.Values) == 0:
				return fmt.Errorf("field %q: condition has no values", label)
			default:
				f["show_if"] = map[string]interface{}{"field": c.Field, "values": c.Values}
			}
		}
		if dep := dependsOn(f); dep != "" {
			if !choice[dep] {
				return fmt.Errorf("field %q: options depend on %q, which is not a choice field above it", label, dep)
			}
			f["depends_on"] = dep
		} else {
			delete(f, "depends_on")
		}
		if isChoiceType(f.S("type")) && f.S("name") != "" {
			choice[f.S("name")] = true
		}
	}
	return nil
}

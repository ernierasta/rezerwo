package main

// people_list.go: lista zapisanych przy polu wyboru na stronie gościa.
//
// Pole checkbox z ustawieniem "Pokaż listę zapisanych" (people_list) ma pod
// każdą opcją imiona i nazwiska osób, które ją zaznaczyły - zgłoszenia gości
// i osoby dopisane przez menedżerów (form_fill.go), bez e-maili. Lista
// odświeża się z resztą stanu formularza (live.go), a gość, który zaznacza
// opcję, od razu widzi przy niej siebie (js/n_formpage.js). Zgłoszenie bez
// imienia i nazwiska (formularz anonimowy) się nie pokazuje. Pusta lista
// się nie pokazuje; z ustawieniem people_slots i celem pola (panel_goal)
// brakujące osoby to puste miejsca (0/3 = trzy puste), kolejne osoby ponad
// cel po prostu dochodzą.

import (
	"log"
	"strings"
)

// isPeopleList - pole z listą zapisanych na stronie gościa.
func isPeopleList(d FormDefField) bool {
	return d.S("type") == "checkbox-group" && d.B("people_list") && d.S("name") != ""
}

// personLabel - "Imię Nazwisko" zgłoszenia.
func personLabel(name, surname string) string {
	return strings.Join(strings.Fields(name+" "+surname), " ")
}

// peopleLists - osoby przy opcjach pól z listą zapisanych:
// pole -> wartość opcji -> imiona i nazwiska (po nazwisku). Nil, gdy
// formularz nie ma takich pól.
func peopleLists(db *DB, templ FormTemplate, defs []FormDefField) map[string]map[string][]string {
	out := map[string]map[string][]string{}
	for _, d := range defs {
		if !isPeopleList(d) {
			continue
		}
		m := map[string][]string{}
		eachOption(d, func(value, _ string) { m[value] = []string{} })
		out[d.S("name")] = m
	}
	if len(out) == 0 {
		return nil
	}
	aa, err := db.PersonAnswersForTemplate(templ.ID)
	if err != nil {
		log.Printf("peopleLists: form %d: %v", templ.ID, err)
		return out
	}
	for _, a := range aa {
		m, ok := out[a.Field]
		if !ok {
			continue
		}
		who := personLabel(a.Name.String, a.Surname.String)
		if who == "" {
			continue
		}
		seen := map[string]bool{}
		for _, v := range strings.Split(a.Value, ", ") {
			v = strings.TrimSpace(v)
			if l, ok := m[v]; ok && !seen[v] {
				seen[v] = true
				m[v] = append(l, who)
			}
		}
	}
	return out
}

// peopleSlots - ile miejsc na osoby pokazać przy opcji pola (0 = bez
// pustych miejsc).
func peopleSlots(d FormDefField) int64 {
	if !d.B("people_slots") {
		return 0
	}
	g := intOf(d["panel_goal"])
	if g < 0 || g > 200 {
		return 0
	}
	return g
}

// AttachPeopleLists dokleja listy zapisanych do pól strony gościa.
func AttachPeopleLists(rows []RenderRow, defs []FormDefField, lists map[string]map[string][]string) {
	slots := map[string]int64{}
	for _, d := range defs {
		if isPeopleList(d) {
			slots[d.S("name")] = peopleSlots(d)
		}
	}
	for i := range rows {
		for j := range rows[i].Fields {
			f := &rows[i].Fields[j]
			m, ok := lists[f.Name]
			if !ok || f.Type != "checkbox-group" {
				continue
			}
			f.PeopleList = true
			f.Slots = slots[f.Name]
			for k := range f.Options {
				o := &f.Options[k]
				o.People = m[o.Value]
				if n := f.Slots - int64(len(o.People)); n > 0 {
					o.Blanks = make([]struct{}, n)
				}
			}
		}
	}
}

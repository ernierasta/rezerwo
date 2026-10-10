package main

import (
	"reflect"
	"sort"
	"testing"
)

// reportSchema składa schemat jak loadAnswerSchema, ale bez bazy.
func reportSchema(t *testing.T, def string) answerSchema {
	defs, err := ParseFormDef(def)
	if err != nil {
		t.Fatal(err)
	}
	s := answerSchema{Defs: defs, defOf: map[string]FormDefField{}}
	for _, d := range defs {
		if n := d.S("name"); n != "" && isInputType(d.S("type")) {
			s.defOf[n] = d
		}
	}
	return s
}

func sorted(s []string) []string {
	sort.Strings(s)
	return s
}

func TestRowVisibility(t *testing.T) {
	s := reportSchema(t, condForm)

	// 1A: sekcja ciasta widoczna, bilety (tylko 2B) nie dotyczą, "co" ukryte
	// do wyboru kategorii
	na, out, picks := s.rowVisibility(map[string]string{"klasa": "1A", "ciasto": "sernik"})
	// (kategoria jest zawsze widoczna - pusta odpowiedź to nie "n/d")
	if want := []string{"bilety", "co"}; !reflect.DeepEqual(sorted(na), want) {
		t.Errorf("na = %v", na)
	}
	if len(out) != 0 {
		t.Errorf("out = %v", out)
	}
	if !reflect.DeepEqual(picks["klasa"], []string{"1A"}) {
		t.Errorf("picks = %v", picks)
	}

	// zgłoszenie sprzed warunku: 2B z ciastem - ciasto "poza warunkiem"
	na, out, _ = s.rowVisibility(map[string]string{"klasa": "2B", "ciasto": "sernik", "bilety": "2"})
	if !reflect.DeepEqual(out, []string{"ciasto"}) {
		t.Errorf("out = %v", out)
	}
	if !reflect.DeepEqual(sorted(na), []string{"co"}) {
		t.Errorf("na = %v", na)
	}

	// opcja zależna niepasująca do rodzica
	_, out, _ = s.rowVisibility(map[string]string{"klasa": "2B", "kategoria": "mebel", "co": "kot"})
	if !reflect.DeepEqual(out, []string{"co"}) {
		t.Errorf("out dep = %v", out)
	}
}

func TestCountOptionsGrouped(t *testing.T) {
	defs, _ := ParseFormDef(`[
		{"type":"select","name":"szkola","label":"Szkoła","values":[{"label":"Karwina","value":"k"},{"label":"Trzyniec","value":"t"}]},
		{"type":"checkbox-group","name":"klasa","label":"Klasa","depends_on":"szkola","values":[
			{"label":"1","when":"k"},{"label":"2","when":"k"},{"label":"1","when":"t"},{"label":"wspólna"}]}]`)
	st := fieldStatDep(defs[1], []string{"1", "1, 2", "1", "wspólna, xyz"}, [][]string{{"k"}, {"k"}, {"t"}, {"t"}})
	got := map[string]int64{}
	for _, o := range st.Options {
		got[o.When+"/"+o.Label] = o.Count
	}
	want := map[string]int64{"k/1": 2, "k/2": 1, "t/1": 1, "/wspólna": 1, "/xyz": 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if st.OptionCount("1") != 3 {
		t.Errorf("OptionCount(1) = %d", st.OptionCount("1"))
	}

	groups := groupOptions(st.Options, defs[0])
	if len(groups) != 3 || groups[0].Label != "Karwina" || groups[1].Label != "Trzyniec" || groups[2].Label != "" {
		t.Errorf("groups = %+v", groups)
	}

	// bez rodzica ta sama wartość to jedna opcja
	st = fieldStat(defs[1], []string{"1", "1, 2"})
	if st.OptionCount("1") != 2 || len(st.Options) != 3 {
		t.Errorf("flat = %+v", st.Options)
	}
}

func TestCSVColumns(t *testing.T) {
	cols := []nAnswerCol{{Key: "a"}, {Key: "p"}, {Key: "b"}, {Key: "c", Parent: "p"}, {Key: "d", Parent: "c"}}
	keys := func(cc []nAnswerCol) []string {
		var out []string
		for _, c := range cc {
			out = append(out, c.Key)
		}
		return out
	}
	if got := keys(csvColumns(cols, nil)); !reflect.DeepEqual(got, []string{"a", "p", "c", "d", "b"}) {
		t.Errorf("all = %v", got)
	}
	// bez rodzica w eksporcie pole zostaje na swoim miejscu
	if got := keys(csvColumns(cols, map[string]bool{"a": true, "b": true, "c": true})); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("only = %v", got)
	}
}

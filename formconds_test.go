package main

import (
	"reflect"
	"testing"
)

const condForm = `[
	{"type":"text","name":"surname-REQUIRED","label":"Nazwisko"},
	{"type":"radio-group","name":"klasa","label":"Klasa","values":[{"label":"1A","value":"1A"},{"label":"2B","value":"2B"}]},
	{"type":"header","label":"Obowiązki 1A","show_if":{"field":"klasa","values":["1A"]}},
	{"type":"text","name":"ciasto","label":"Ciasto","required":true},
	{"type":"text","name":"email-REQUIRED","label":"E-mail"},
	{"type":"number","name":"bilety","label":"Bilety","show_if":{"field":"klasa","values":["2B"]}},
	{"type":"header","label":"Reszta"},
	{"type":"select","name":"kategoria","label":"Kategoria","values":[{"label":"zwierzak"},{"label":"mebel"}]},
	{"type":"select","name":"co","label":"Co","required":true,"depends_on":"kategoria","values":[
		{"label":"kot","when":"zwierzak"},{"label":"pies","when":"zwierzak"},
		{"label":"stół","when":"mebel"},{"label":"inne"}]}
]`

func condData(kv ...string) []FormAnswersJson {
	var out []FormAnswersJson
	for i := 0; i < len(kv); i += 2 {
		out = append(out, FormAnswersJson{Name: kv[i], UserData: []string{kv[i+1]}})
	}
	return out
}

func TestApplyConds(t *testing.T) {
	defs, err := ParseFormDef(condForm)
	if err != nil {
		t.Fatal(err)
	}

	// klasa 2B: sekcja 1A ukryta (bez e-maila - rola zawsze widoczna), bilety widoczne
	data := condData("surname-REQUIRED", "Nowak", "klasa", "2B", "ciasto", "sernik",
		"email-REQUIRED", "a@b.cz", "bilety", "3", "kategoria", "zwierzak", "co", "stół")
	vis := ApplyConds(defs, data)
	want := []bool{true, true, false, false, true, true, true, true, true}
	if !reflect.DeepEqual(vis, want) {
		t.Errorf("vis = %v, want %v", vis, want)
	}
	if data[2].UserData != nil {
		t.Errorf("hidden ciasto kept %v", data[2].UserData)
	}
	if data[6].UserData != nil {
		t.Errorf("stół for zwierzak kept %v", data[6].UserData)
	}
	if bad := checkRequired(defs, vis, data); bad != "Co" {
		t.Errorf("checkRequired = %q, want Co", bad)
	}

	// klasa 1A: wymagane ciasto; zależne pole ukryte bez kategorii
	data = condData("surname-REQUIRED", "Nowak", "klasa", "1A", "ciasto", "", "email-REQUIRED", "a@b.cz",
		"kategoria", "", "co", "kot")
	vis = ApplyConds(defs, data)
	if vis[5] || vis[8] || !vis[3] {
		t.Errorf("vis = %v", vis)
	}
	if bad := checkRequired(defs, vis, data); bad != "Ciasto" {
		t.Errorf("checkRequired = %q, want Ciasto", bad)
	}

	// opcja bez "when" pasuje do każdego wyboru
	data = condData("kategoria", "mebel", "co", "inne")
	ApplyConds(defs, data)
	if !reflect.DeepEqual(data[1].UserData, []string{"inne"}) {
		t.Errorf("co = %v", data[1].UserData)
	}
}

func TestCheckConds(t *testing.T) {
	defs, _ := ParseFormDef(condForm)
	if _, err := CleanFormDef(defs); err != nil {
		t.Fatalf("valid form: %v", err)
	}
	bad := []string{
		// rodzic niżej
		`[{"type":"text","name":"a","show_if":{"field":"b","values":["x"]}},{"type":"select","name":"b","values":[{"label":"x"}]}]`,
		// rodzic nie jest polem wyboru
		`[{"type":"text","name":"b"},{"type":"text","name":"a","show_if":{"field":"b","values":["x"]}}]`,
		// brak wartości
		`[{"type":"select","name":"b","values":[{"label":"x"}]},{"type":"text","name":"a","show_if":{"field":"b","values":[]}}]`,
		// zależne od samego siebie
		`[{"type":"select","name":"b","depends_on":"b","values":[{"label":"x"}]}]`,
	}
	for _, s := range bad {
		defs, _ := ParseFormDef(s)
		if _, err := CleanFormDef(defs); err == nil {
			t.Errorf("accepted %s", s)
		}
	}
	defs, _ = ParseFormDef(`[{"type":"select","name":"b","values":[{"label":"x"}]},
		{"type":"text","name":"email-REQUIRED","show_if":{"field":"b","values":["x"]}}]`)
	out, err := CleanFormDef(defs)
	if err != nil || out[1]["show_if"] != nil {
		t.Errorf("role condition kept: %v %v", out[1], err)
	}
}

func TestBuildRenderRowsConds(t *testing.T) {
	defs, _ := ParseFormDef(condForm)
	rows := BuildRenderRows(defs, "")
	find := func(name string) RenderField {
		for _, r := range rows {
			for _, f := range r.Fields {
				if f.Name == name {
					return f
				}
			}
		}
		t.Fatalf("no field %q", name)
		return RenderField{}
	}
	if s := find("ciasto").ShowIf; s != `{"field":"klasa","values":["1A"]}` {
		t.Errorf("ciasto ShowIf = %s", s)
	}
	if s := find("email-REQUIRED").ShowIf; s != "" {
		t.Errorf("email ShowIf = %s", s)
	}
	if s := find("kategoria").ShowIf; s != "" {
		t.Errorf("section without condition: ShowIf = %s", s)
	}
	co := find("co")
	if co.DependsOn != "kategoria" || co.Options[0].When != "zwierzak" || co.Options[3].When != "" {
		t.Errorf("co = %+v", co)
	}
}

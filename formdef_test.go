package main

import (
	"html/template"
	"strings"
	"testing"
)

func TestCleanLabel(t *testing.T) {
	cases := map[string]string{
		`Klasa (<b>zaznacz</b>) `:                                   `Klasa (<b>zaznacz</b>)`,
		`<span style="font-weight: bolder;">godz. 17:00</span><br>`: `godz. 17:00`,
		`&lt;b&gt;Większość&lt;/b&gt;`:                              `<b>Większość</b>`,
		`Dar<br>(Kč)&nbsp;`:                                         `Dar<br>(Kč)`,
		`<script>alert(1)</script>Tekst`:                            `Tekst`,
	}
	for in, want := range cases {
		if got := CleanLabel(in); got != want {
			t.Errorf("CleanLabel(%q) = %q, want %q", in, got, want)
		}
	}
	if got := CleanLabel(`<a href="javascript:x()">a</a>`); strings.Contains(got, "javascript") {
		t.Errorf("javascript link kept: %q", got)
	}
}

func TestBuildRenderRows(t *testing.T) {
	// jak formRender: pola z tym samym row-N w jednym wierszu, w miejscu pierwszego
	defs, err := ParseFormDef(`[
		{"type":"header","label":"Dane"},
		{"type":"text","name":"a","label":"A","className":"form-control row-1 col-md-4"},
		{"type":"text","name":"b","label":"B","className":"form-control row-1 col-md-8"},
		{"type":"number","name":"c","label":"C"},
		{"type":"text","name":"d","label":"D","className":"form-control row-1 col-md-3"},
		{"type":"button","name":"x","label":"X"}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	rows := BuildRenderRows(defs, "c")
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if n := len(rows[1].Fields); n != 3 || rows[1].Fields[2].Name != "d" || rows[1].Fields[0].Width != 4 {
		t.Errorf("row 1 = %+v", rows[1].Fields)
	}
	if !rows[2].Fields[0].IsMoney || rows[2].Fields[0].Width != 12 {
		t.Errorf("money field = %+v", rows[2].Fields[0])
	}
}

func TestCleanFormDefDuplicate(t *testing.T) {
	defs, _ := ParseFormDef(`[{"type":"text","name":"a"},{"type":"text","name":"a"}]`)
	if _, err := CleanFormDef(defs); err == nil {
		t.Error("duplicate names accepted")
	}
	defs, _ = ParseFormDef(`[{"type":"paragraph","name":"p","label":"x"}]`)
	out, err := CleanFormDef(defs)
	if err != nil || out[0]["name"] != nil {
		t.Errorf("paragraph: %v %v", out, err)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Bal 2026 – deklaracja": "bal-2026-deklaracja",
		"Zjazd Gwiaździsty":     "zjazd-gwiazdzisty",
		"Příjmení žáka":         "prijmeni-zaka",
		"Łódź":                  "lodz",
		"  ":                    "",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

// Odpowiedź pola "Wielokrotność" ma postać "2 * 85 = 170".
func TestMultiplierValue(t *testing.T) {
	a, m, tot := ConvertMultiplicationToAmmountMultiplTotal("2 * 85 = 170")
	if a != "2" || m != "85" || tot != "170" {
		t.Errorf("round trip: %q %q %q", a, m, tot)
	}
}

func TestLineLook(t *testing.T) {
	defs, err := ParseFormDef(`[
		{"type":"divider"},
		{"type":"divider","line":"double","line_color":"#b23b3b"},
		{"type":"divider","line":"none","line_color":"red;}x{"},
		{"type":"header","label":"A"},
		{"type":"header","label":"B","line":"none","line_color":"#2463a6"},
		{"type":"header","label":"C","line":"dashed","line_color":"#2463A6"}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ line, color string }{
		{"solid", ""}, {"double", "#b23b3b"}, {"solid", ""}, {"", ""}, {"none", ""}, {"dashed", "#2463A6"},
	}
	var got []RenderField
	for _, r := range BuildRenderRows(defs, "") {
		got = append(got, r.Fields...)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d fields, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Line != w.line || string(got[i].LineColor) != w.color {
			t.Errorf("field %d: line %q color %q, want %q %q", i, got[i].Line, got[i].LineColor, w.line, w.color)
		}
	}

	// zapis: złe wartości znikają, oddzielacz nie dostaje nazwy ani pola w formfields
	defs, _ = ParseFormDef(`[{"type":"divider","name":"x","line":"none","line_color":"url(x)"},{"type":"header","label":"H","line":"wavy"}]`)
	out, err := CleanFormDef(defs)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"name", "line", "line_color"} {
		if _, ok := out[0][k]; ok {
			t.Errorf("divider kept %q", k)
		}
	}
	if _, ok := out[1]["line"]; ok {
		t.Errorf("header kept bad line")
	}
	if isInputType("divider") {
		t.Errorf("divider is an input")
	}

	// szablon gościa: kolor w atrybucie style przechodzi przez html/template
	tm, err := template.New("").Funcs(template.FuncMap{"T": func(...interface{}) string { return "" }}).ParseFiles("tmpl/n_formfield.html")
	if err != nil {
		t.Fatal(err)
	}
	for i, w := range []string{
		`<div class="ff ff-divider w12 ln-double" style="--lc: #b23b3b"><hr></div>`,
		`<div class="ff ff-header w12 ln-dashed" style="--lc: #2463A6">`,
	} {
		var b strings.Builder
		if err := tm.ExecuteTemplate(&b, "ffield", got[[]int{1, 5}[i]]); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(b.String(), w) {
			t.Errorf("template: %s, want %s", b.String(), w)
		}
	}
}

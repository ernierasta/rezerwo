package main

import (
	"strings"
	"testing"
)

func TestCleanLabel(t *testing.T) {
	cases := map[string]string{
		`Klasa (<b>zaznacz</b>) `:                                  `Klasa (<b>zaznacz</b>)`,
		`<span style="font-weight: bolder;">godz. 17:00</span><br>`: `godz. 17:00`,
		`&lt;b&gt;Większość&lt;/b&gt;`:                             `<b>Większość</b>`,
		`Dar<br>(Kč)&nbsp;`:                                        `Dar<br>(Kč)`,
		`<script>alert(1)</script>Tekst`:                           `Tekst`,
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

func TestMultiplierValue(t *testing.T) {
	if got := MultiplierValue(3, 590); got != "3 * 590 = 1770" {
		t.Errorf("got %q", got)
	}
	a, m, tot := ConvertMultiplicationToAmmountMultiplTotal(MultiplierValue(2, 85))
	if a != "2" || m != "85" || tot != "170" {
		t.Errorf("round trip: %q %q %q", a, m, tot)
	}
}

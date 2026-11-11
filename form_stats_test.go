package main

import (
	"strings"
	"testing"
)

func TestCountOptions(t *testing.T) {
	defs, _ := ParseFormDef(`[{"type":"checkbox-group","name":"k","label":"Klasa","values":[
		{"label":"1","value":"1"},{"label":"Tak, chętnie","value":"Tak, chętnie"},{"label":"Tak","value":"Tak"},{"label":"9","value":"9"}]}]`)
	st := fieldStat(defs[0], []string{"1, 9", "Tak, chętnie", "Tak", "1, własne"})
	if st.Kind != "options" {
		t.Fatalf("kind %q", st.Kind)
	}
	want := map[string]int64{"1": 2, "Tak, chętnie": 1, "Tak": 1, "9": 1, "własne": 1}
	for _, o := range st.Options {
		if want[o.Label] != o.Count {
			t.Errorf("%q = %d, want %d", o.Label, o.Count, want[o.Label])
		}
	}
	if last := st.Options[len(st.Options)-1]; !last.Other || last.Label != "własne" {
		t.Errorf("other = %+v", last)
	}
	// numery klas to opcje, nie liczby do zsumowania (dawny kłopot .Sum)
	if st.Value() != "6" {
		t.Errorf("value %q", st.Value())
	}
}

func TestFieldStatSums(t *testing.T) {
	defs, _ := ParseFormDef(`[{"type":"number","name":"n"},{"type":"multiplierField","name":"m"},{"type":"text","name":"t"}]`)
	if st := fieldStat(defs[0], []string{"2", "3,5", "abc"}); st.Value() != "5.50" {
		t.Errorf("number: %q", st.Value())
	}
	st := fieldStat(defs[1], []string{"3 * 590 = 1770", "1 * 590 = 590"})
	if st.Value() != "4" || st.Money != 2360 {
		t.Errorf("multiplier: %q %v", st.Value(), st.Money)
	}
	if st := fieldStat(defs[2], []string{"a", "b"}); st.Value() != "2" {
		t.Errorf("text: %q", st.Value())
	}
}

func TestFieldInPanel(t *testing.T) {
	defs, _ := ParseFormDef(`[
		{"type":"number","name":"a"},
		{"type":"number","name":"b","panel":false},
		{"type":"text","name":"c"},
		{"type":"text","name":"d","panel":true},
		{"type":"text","name":"email-REQUIRED","panel":true}]`)
	auto := []bool{true, false, false, true, false}
	fields := []bool{false, false, false, true, false}
	for i, d := range defs {
		if got := fieldInPanel(d, PANELAUTO); got != auto[i] {
			t.Errorf("auto %d: %v", i, got)
		}
		if got := fieldInPanel(d, PANELFIELDS); got != fields[i] {
			t.Errorf("fields %d: %v", i, got)
		}
	}
}

func TestOptionsLook(t *testing.T) {
	src := `{"type":"checkbox-group","name":"k","label":"Klasa","values":[
		{"label":"1A","value":"1a"},{"label":"1B","value":"1b"},{"label":"2A","value":"2a"},{"label":"2B","value":"2b"},{"label":"3A","value":"3a"}]`
	answers := []string{"1b", "1b, 2a", "jeśli trzeba dzwońcie", "2a, coś"}
	html := func(extra string) string {
		defs, err := ParseFormDef("[" + src + extra + "}]")
		if err != nil {
			t.Fatal(err)
		}
		return string(optionsHTML(defs[0], fieldStat(defs[0], answers), "pl"))
	}

	// domyślnie: kafelki (dużo krótkich opcji), etykiety, zera, "Inne" razem
	got := html("")
	want := `<ul class="sum-list compact"><li class="sum-list-el"><span>1A</span> <b>0</b></li>` +
		`<li class="sum-list-el"><span>1B</span> <b>2</b></li><li class="sum-list-el"><span>2A</span> <b>2</b></li>` +
		`<li class="sum-list-el"><span>2B</span> <b>0</b></li><li class="sum-list-el"><span>3A</span> <b>0</b></li>` +
		`<li class="sum-list-el other"><span>Inne</span> <b>2</b></li></ul>`
	if got != want {
		t.Errorf("default:\n%s\nwant\n%s", got, want)
	}

	got = html(`,"panel_layout":"list","panel_zero":"hide","panel_other":"each","panel_sort":"count"`)
	want = `<ul class="sum-list"><li class="sum-list-el"><span>1B:</span> <b>2</b></li><li class="sum-list-el"><span>2A:</span> <b>2</b></li>` +
		`<li class="sum-list-el other"><span>jeśli trzeba dzwońcie:</span> <b>1</b></li><li class="sum-list-el other"><span>coś:</span> <b>1</b></li></ul>`
	if got != want {
		t.Errorf("list:\n%s\nwant\n%s", got, want)
	}

	if got = html(`,"panel_other":"hide","panel_zero":"hide"`); strings.Contains(got, "Inne") || strings.Contains(got, "<b>0</b>") {
		t.Errorf("hide: %s", got)
	}
	if got = html(`,"panel_layout":"total"`); got != "6" {
		t.Errorf("total: %q", got)
	}
}

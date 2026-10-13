package main

import (
	"strings"
	"testing"
)

func TestPageHeadParseClean(t *testing.T) {
	if got := ParsePageHead(""); got != (PageHead{}) {
		t.Errorf("empty: %+v", got)
	}
	if got := ParsePageHead("{zepsuty"); got != (PageHead{}) {
		t.Errorf("broken JSON: %+v", got)
	}
	if s := (PageHead{}).String(); s != "" {
		t.Errorf("default head stored as %q, want empty", s)
	}

	h := ParsePageHead(`{"name":"  Zjazd \n 2026 ","align":"center","over":"Deklaracja","over_color":"red","over_size":"xl","over_bold":true,"over_upper":true,"over_align":"right"}`)
	want := PageHead{Name: "Zjazd 2026", Align: "center", Over: "Deklaracja", OverColor: "red", OverSize: "xl", OverBold: true, OverUpper: true, OverAlign: "right"}
	if h != want {
		t.Errorf("parse = %+v, want %+v", h, want)
	}
	if back := ParsePageHead(h.String()); back != h {
		t.Errorf("round trip = %+v, want %+v", back, h)
	}

	// nieznane wartości (np. wstrzyknięte klasy) wracają do domyślnych
	bad := ParsePageHead(`{"align":"justify","over_color":"x\" onclick=\"y","over_size":"huge","over_align":"left"}`)
	if bad != (PageHead{}) {
		t.Errorf("unknown values kept: %+v", bad)
	}

	long := strings.Repeat("ą", PAGEHEADMAX+50)
	if n := len([]rune(ParsePageHead(`{"over":"` + long + `"}`).Over)); n != PAGEHEADMAX {
		t.Errorf("over length %d, want %d", n, PAGEHEADMAX)
	}
}

func TestPageHeadFromPanel(t *testing.T) {
	vals := map[string]string{"head_name": "Bal 2026", "head_align": "center", "head_over": "Zaproszenie", "head_over_color": "accent"}
	flags := map[string]bool{"head_over_italic": true, "head_hide_name": true}
	val := func(k string) string { return vals[k] }
	flag := func(k string) bool { return flags[k] }

	// nazwa taka jak nazwa imprezy = pusta, czyli idzie za nazwą
	h := pageHeadFromPanel(val, flag, "Bal 2026")
	if h.Name != "" || h.Align != "center" || h.Over != "Zaproszenie" || h.OverColor != "accent" || !h.OverItalic || !h.HideName {
		t.Errorf("same name: %+v", h)
	}
	if h := pageHeadFromPanel(val, flag, "Bal karnawałowy"); h.Name != "Bal 2026" {
		t.Errorf("own name lost: %+v", h)
	}
	if p := headPick(PageHead{}, "Bal karnawałowy", false); p.Name != "Bal karnawałowy" {
		t.Errorf("panel field without own name = %q", p.Name)
	}
}

func TestGuestHead(t *testing.T) {
	g := guestHead(PageHead{}, "Bal", "banner.png", "org", "1.1.2027")
	if g.Name != "Bal" || g.NameClass != "" || g.OverClass != "" || g.BannerFull || g.Banner == "" || g.When != "1.1.2027" {
		t.Errorf("default: %+v", g)
	}
	h := PageHead{Name: "Wielki Bal", Align: "right", Over: "Macierz", OverColor: "red", OverSize: "l", OverBold: true, OverItalic: true, OverUpper: true, OverAlign: "center"}
	g = guestHead(h, "Bal", "banner.png;full", "org", "")
	if g.Name != "Wielki Bal" || g.NameClass != "ta-right" || !g.BannerFull {
		t.Errorf("custom: %+v", g)
	}
	if g.OverClass != "c-red sz-l gh-b gh-i gh-up ta-center" {
		t.Errorf("over class = %q", g.OverClass)
	}
	if g := guestHead(ParsePageHead(`{"width":"page"}`), "Bal", "", "org", ""); !g.Wide {
		t.Errorf("wide: %+v", g)
	}
	if g := guestHead(ParsePageHead(`{"width":"huge"}`), "Bal", "", "org", ""); g.Wide {
		t.Errorf("unknown width: %+v", g)
	}
	if g := guestHead(ParsePageHead(`{"hide_name":true}`), "Bal", "", "org", ""); !g.HideName || g.Name != "Bal" {
		t.Errorf("hidden name: %+v", g)
	}
	if g := guestHead(PageHead{}, "Bal", ";full", "org", ""); g.BannerFull || g.Banner != "" {
		t.Errorf("full without file: %+v", g)
	}
}

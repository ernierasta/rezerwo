package main

// pagehead.go: nagłówek strony gościa imprezy i formularza - wyświetlana
// nazwa (zamiast nazwy imprezy/formularza) z wyrównaniem i linijka "nad
// nazwą" z kolorem, wielkością i formatowaniem.
//
// W bazie (events.pagehead, formtemplates.pagehead,
// tools/migration-pagehead.sql) jest JSON PageHead, pusty = wszystko
// domyślne. Pusta nazwa oznacza "jak nazwa imprezy/formularza" - zmiana nazwy
// zmienia wtedy też nagłówek; wpisana inna jest od niej niezależna.

import (
	"encoding/json"
	"html/template"
	"strings"
	"unicode/utf8"
)

// pageHeadAligns - wyrównania nazwy i linijki nad nią ("" = do lewej).
var pageHeadAligns = []string{"", "center", "right"}

// pageHeadColors - kolory linijki nad nazwą ("" = przygaszony kolor tekstu
// tematu); wartości kolorów są w scss/ui.scss (.ghead .over.c-<klucz>),
// osobno dla jasnych i ciemnych tematów.
var pageHeadColors = []string{"", "ink", "accent", "red", "orange", "green", "blue", "purple"}

// pageHeadWidths - szerokość nagłówka formularza ("" = jak formularz,
// "page" = formularz + kolumna panelu informacji).
var pageHeadWidths = []string{"", "page"}

// pageHeadSizes - wielkości linijki nad nazwą ("" = najmniejsza).
var pageHeadSizes = []string{"", "m", "l", "xl"}

// PAGEHEADMAX - najdłuższa nazwa / linijka nad nazwą (znaki).
const PAGEHEADMAX = 200

type PageHead struct {
	Name       string `json:"name,omitempty"`
	Align      string `json:"align,omitempty"`
	HideName   bool   `json:"hide_name,omitempty"` // nazwa tylko dla czytników ekranu (np. jest już na bannerze)
	Over       string `json:"over,omitempty"`
	OverColor  string `json:"over_color,omitempty"`
	OverSize   string `json:"over_size,omitempty"`
	OverBold   bool   `json:"over_bold,omitempty"`
	OverItalic bool   `json:"over_italic,omitempty"`
	OverUpper  bool   `json:"over_upper,omitempty"`
	OverAlign  string `json:"over_align,omitempty"`
	Width      string `json:"width,omitempty"` // tylko formularz, pageHeadWidths
}

// ParsePageHead czyta nagłówek zapisany w bazie; zepsuty JSON = domyślny.
func ParsePageHead(s string) PageHead {
	var h PageHead
	if strings.TrimSpace(s) != "" {
		_ = json.Unmarshal([]byte(s), &h)
	}
	return h.clean()
}

// String - postać do zapisu w bazie ("" gdy wszystko domyślne).
func (h PageHead) String() string {
	h = h.clean()
	if h == (PageHead{}) {
		return ""
	}
	b, _ := json.Marshal(h)
	return string(b)
}

func oneOf(v string, allowed []string) string {
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	return ""
}

func headText(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > PAGEHEADMAX {
		s = string([]rune(s)[:PAGEHEADMAX])
	}
	return s
}

// clean - tylko znane wartości, teksty w jednej linii i nie za długie.
func (h PageHead) clean() PageHead {
	h.Name = headText(h.Name)
	h.Over = headText(h.Over)
	h.Align = oneOf(h.Align, pageHeadAligns)
	h.OverAlign = oneOf(h.OverAlign, pageHeadAligns)
	h.OverColor = oneOf(h.OverColor, pageHeadColors)
	h.OverSize = oneOf(h.OverSize, pageHeadSizes)
	h.Width = oneOf(h.Width, pageHeadWidths)
	return h
}

// pageHeadFromPanel składa nagłówek z pól sekcji panelu (szablon "headpick"
// w n_base.html). Nazwa równa nazwie imprezy/formularza zapisuje się jako
// pusta, czyli dalej idzie za nazwą.
func pageHeadFromPanel(val func(string) string, flag func(string) bool, name string) PageHead {
	h := PageHead{
		Name:       val("head_name"),
		Align:      val("head_align"),
		HideName:   flag("head_hide_name"),
		Over:       val("head_over"),
		OverColor:  val("head_over_color"),
		OverSize:   val("head_over_size"),
		OverBold:   flag("head_over_bold"),
		OverItalic: flag("head_over_italic"),
		OverUpper:  flag("head_over_upper"),
		OverAlign:  val("head_over_align"),
		Width:      val("head_width"),
	}.clean()
	if h.Name == headText(name) {
		h.Name = ""
	}
	return h
}

// nHeadPick - dane szablonu "headpick" w panelu.
type nHeadPick struct {
	PageHead
	Name   string // nazwa do pola: wyświetlana albo nazwa imprezy/formularza
	Form   bool   // formularz: wybór szerokości nagłówka
	Colors []string
	Sizes  []string
}

func headPick(h PageHead, name string, form bool) nHeadPick {
	p := nHeadPick{PageHead: h, Name: h.Name, Form: form, Colors: pageHeadColors, Sizes: pageHeadSizes}
	if p.Name == "" {
		p.Name = name
	}
	return p
}

// GuestHead - górna część strony gościa: banner, linijka nad nazwą, nazwa
// i data (szablony "guestwide" i "guesthead" w n_guest_base.html).
type GuestHead struct {
	Banner     template.HTML
	BannerFull bool // banner na pełną szerokość strony, nad nagłówkiem i formularzem
	Over       string
	OverClass  string
	Name       string
	NameClass  string
	HideName   bool // nazwa ukryta wizualnie, zostaje dla czytników ekranu
	Wide       bool // nagłówek formularza na szerokość formularza i panelu informacji
	When       string
}

func guestHead(h PageHead, name, banner, userURL, when string) GuestHead {
	b := ParseBanner(banner)
	g := GuestHead{
		Banner:     b.ImgHTML(userURL),
		BannerFull: b.Full && b.File != "",
		Over:       h.Over,
		Name:       h.Name,
		HideName:   h.HideName,
		Wide:       h.Width == "page",
		When:       when,
	}
	if g.Name == "" {
		g.Name = name
	}
	if h.Align != "" {
		g.NameClass = "ta-" + h.Align
	}
	cls := []string{}
	if h.OverColor != "" {
		cls = append(cls, "c-"+h.OverColor)
	}
	if h.OverSize != "" {
		cls = append(cls, "sz-"+h.OverSize)
	}
	if h.OverBold {
		cls = append(cls, "gh-b")
	}
	if h.OverItalic {
		cls = append(cls, "gh-i")
	}
	if h.OverUpper {
		cls = append(cls, "gh-up")
	}
	if h.OverAlign != "" {
		cls = append(cls, "ta-"+h.OverAlign)
	}
	g.OverClass = strings.Join(cls, " ")
	return g
}

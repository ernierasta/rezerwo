package main

// sponsors.go: sekcja "Sponsorzy" imprezy - logo w wierszach ułożonych
// w panelu przeciąganiem (js/n_sponsors.js), pokazywana gościom na wybranych
// stronach (wybór miejsc, formularz zamówienia, podziękowanie) i w mailach
// pastylką {{.Sponsors}} (OrderFuncs.Sponsors).
//
// W bazie events.sponsors (tools/migration-sponsors.sql) jest JSON Sponsors;
// pliki logo leżą w media/<organizacja>/ jak bannery (banner.go).

import (
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp"
)

// SPONSORSMAX - najwięcej logo w sekcji (ochrona przed śmieciowym zapisem).
const SPONSORSMAX = 60

// sponsorSizes - wysokość logo: "" = średnie, "s" = małe, "l" = duże.
var sponsorSizes = map[string]int{"s": 48, "": 72, "l": 104}

type SponsorLogo struct {
	File string `json:"file"`           // ścieżka względem media/<organizacja>/
	Link string `json:"link,omitempty"` // strona sponsora (http/https) albo pusto
}

type Sponsors struct {
	Title string          `json:"title,omitempty"` // pusto = "Sponsorzy" w języku imprezy
	Size  string          `json:"size,omitempty"`  // klucz sponsorSizes
	Rows  [][]SponsorLogo `json:"rows,omitempty"`
	// gdzie sekcja jest widoczna dla gości
	Seats bool `json:"seats,omitempty"` // strona wyboru miejsc
	Order bool `json:"order,omitempty"` // formularz zamówienia
	Done  bool `json:"done,omitempty"`  // strona z podziękowaniem
}

// ParseSponsors czyta sekcję z bazy; zły JSON = brak sekcji.
func ParseSponsors(s string) Sponsors {
	var sp Sponsors
	if strings.TrimSpace(s) == "" {
		return sp
	}
	if err := json.Unmarshal([]byte(s), &sp); err != nil {
		log.Printf("ParseSponsors: %v", err)
		return Sponsors{}
	}
	return sp.clean()
}

// clean porządkuje dane z panelu: pliki tylko z katalogu organizacji
// i z rozszerzeniem obrazka, linki tylko http(s), bez pustych wierszy.
func (sp Sponsors) clean() Sponsors {
	out := Sponsors{
		Title: strings.TrimSpace(sp.Title),
		Seats: sp.Seats, Order: sp.Order, Done: sp.Done,
	}
	if len([]rune(out.Title)) > 120 {
		out.Title = string([]rune(out.Title)[:120])
	}
	if _, ok := sponsorSizes[sp.Size]; ok {
		out.Size = sp.Size
	}
	n := 0
	for _, row := range sp.Rows {
		var r []SponsorLogo
		for _, l := range row {
			f := strings.TrimPrefix(path.Clean("/"+strings.TrimSpace(l.File)), "/")
			if f == "" || !bannerExts[strings.ToLower(path.Ext(f))] || n >= SPONSORSMAX {
				continue
			}
			r = append(r, SponsorLogo{File: f, Link: cleanSponsorLink(l.Link)})
			n++
		}
		if len(r) > 0 {
			out.Rows = append(out.Rows, r)
		}
	}
	return out
}

// cleanSponsorLink - adres strony sponsora; "firma.cz" dostaje https://,
// inne schematy (javascript: itd.) wypadają.
func cleanSponsorLink(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.String()
}

// String - postać do zapisu w bazie (pusto, gdy nie ma logo ani ustawień).
func (sp Sponsors) String() string {
	sp = sp.clean()
	if len(sp.Rows) == 0 && sp.Title == "" && sp.Size == "" && !sp.Seats && !sp.Order && !sp.Done {
		return ""
	}
	b, _ := json.Marshal(sp)
	return string(b)
}

// Count - liczba logo.
func (sp Sponsors) Count() int {
	n := 0
	for _, r := range sp.Rows {
		n += len(r)
	}
	return n
}

// Height - wysokość logo w px.
func (sp Sponsors) Height() int {
	return sponsorSizes[sp.Size]
}

// TitleOr - tytuł sekcji albo domyślny w języku imprezy.
func (sp Sponsors) TitleOr(lang string) string {
	if sp.Title != "" {
		return sp.Title
	}
	return T(lang, "sponsors.title")
}

// nGuestSponsors - sekcja gotowa do szablonu "sponsors" (n_guest_base.html).
type nGuestSponsors struct {
	Title  string
	Height int
	Rows   []nGuestSponsorRow
}

// nGuestSponsorRow - wiersz logo; Height (px) > 0 nadpisuje wysokość sekcji.
type nGuestSponsorRow struct {
	Height int
	Logos  []nGuestSponsor
}

type nGuestSponsor struct {
	Src  string
	Link string
}

// guestSponsors - sekcja dla strony gościa albo nil, gdy na tej stronie
// (where: "seats", "order", "done") jej nie ma.
func guestSponsors(event Event, userURL, lang, where string) *nGuestSponsors {
	sp := ParseSponsors(event.Sponsors)
	on := map[string]bool{"seats": sp.Seats, "order": sp.Order, "done": sp.Done}[where]
	if !on || sp.Count() == 0 {
		return nil
	}
	return sponsorsView(sp, userURL, lang, sponsorsPageWidth[where])
}

// sponsorsPageWidth - szerokość treści strony gościa (px, scss/ui.scss:
// .guest, .order-step, .order-status bez paddingu). Na stronie wyboru miejsc
// zostaje układ administratora (za szeroki wiersz zwęża flex); na dalszych
// stronach logo mają tę samą wielkość co tam, a wiersze, które się przy niej
// nie mieszczą, są dzielone na kilka (splitSponsorRow).
const SPONSORSSEATSWIDTH = 1128

var sponsorsPageWidth = map[string]int{"order": 608, "done": 648}

// SPONSORSGAP - odstęp między logo w wierszu na stronie (scss: .sp-row gap).
const SPONSORSGAP = 32

func sponsorsView(sp Sponsors, userURL, lang string, maxW int) *nGuestSponsors {
	h := sp.Height()
	v := &nGuestSponsors{Title: sp.TitleOr(lang), Height: h}
	dir := filepath.Join(MEDIAROOT, userURL)
	for _, row := range sp.Rows {
		var r []nGuestSponsor
		var ws []int
		for _, l := range row {
			r = append(r, nGuestSponsor{Src: sponsorSrc(userURL, l.File), Link: l.Link})
			ws = append(ws, sponsorWidth(filepath.Join(dir, filepath.FromSlash(l.File)), h))
		}
		if maxW <= 0 { // strona wyboru miejsc
			v.Rows = append(v.Rows, nGuestSponsorRow{Logos: r})
			continue
		}
		// wysokość jak na stronie wyboru miejsc: flex zwęża logo proporcjonalnie
		// do szerokości, odstępy zostają
		rh := sponsorSeatsHeight(ws, h)
		if rh < h {
			for i := range ws {
				ws[i] = ws[i] * rh / h
			}
		}
		for _, part := range splitSponsorRow(ws, SPONSORSGAP, maxW) {
			row := nGuestSponsorRow{Logos: r[part[0]:part[1]]}
			if rh < h {
				row.Height = rh
			}
			v.Rows = append(v.Rows, row)
		}
	}
	return v
}

// sponsorSeatsHeight - wysokość logo wiersza o szerokościach ws (przy
// wysokości h) na stronie wyboru miejsc.
func sponsorSeatsHeight(ws []int, h int) int {
	sum := 0
	for _, w := range ws {
		sum += w
	}
	room := SPONSORSSEATSWIDTH - SPONSORSGAP*(len(ws)-1)
	if sum <= room || sum == 0 {
		return h
	}
	if room < sum/10 {
		room = sum / 10
	}
	return max(h*room/sum, 1)
}

// sponsorWidth - szerokość logo o wysokości h; gdy wymiarów nie da się
// odczytać (np. SVG), zgadujemy typowe szerokie logo 2:1.
func sponsorWidth(p string, h int) int {
	if iw, ih := imageSize(p); iw > 0 && ih > 0 {
		return iw * h / ih
	}
	return 2 * h
}

// splitSponsorRow dzieli wiersz logo o szerokościach ws (odstęp gap), który
// nie mieści się w maxW, na najmniej wierszy, w których się mieści,
// zachowując kolejność i wyrównując ich szerokości (najszerszy wiersz
// możliwie wąski), żeby wszystkie logo zostały w podobnej wielkości
// zamiast zmniejszać się w jednym długim wierszu. Wynik: przedziały [od, do).
// Gdy nie mieści się nawet jedno logo, każde idzie w osobny wiersz.
func splitSponsorRow(ws []int, gap, maxW int) [][2]int {
	n := len(ws)
	pre := make([]int, n+1)
	for i, w := range ws {
		pre[i+1] = pre[i] + w
	}
	width := func(i, j int) int { return pre[j] - pre[i] + gap*(j-i-1) }
	if n == 0 || maxW <= 0 || width(0, n) <= maxW {
		return [][2]int{{0, n}}
	}
	for k := 2; k <= n; k++ {
		// best[p][j] - najmniejsza szerokość najszerszego wiersza przy
		// podziale pierwszych j logo na p wierszy; cut - gdzie zaczyna się
		// ostatni z nich.
		best := make([][]int, k+1)
		cut := make([][]int, k+1)
		for p := range best {
			best[p] = make([]int, n+1)
			cut[p] = make([]int, n+1)
		}
		for j := 1; j <= n; j++ {
			best[1][j] = width(0, j)
		}
		for p := 2; p <= k; p++ {
			for j := p; j <= n; j++ {
				best[p][j] = -1
				for i := p - 1; i < j; i++ {
					m := max(best[p-1][i], width(i, j))
					if best[p][j] < 0 || m < best[p][j] {
						best[p][j], cut[p][j] = m, i
					}
				}
			}
		}
		if best[k][n] <= maxW || k == n {
			parts := make([][2]int, k)
			j := n
			for p := k; p >= 1; p-- {
				i := 0
				if p > 1 {
					i = cut[p][j]
				}
				parts[p-1] = [2]int{i, j}
				j = i
			}
			return parts
		}
	}
	return [][2]int{{0, n}}
}

func sponsorSrc(userURL, file string) string {
	parts := strings.Split(file, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return path.Join("/", MEDIAROOT, url.PathEscape(userURL)) + "/" + strings.Join(parts, "/")
}

// --- mail ---------------------------------------------------------------------

// SPONSORSMAILWIDTH - najszersza linijka logo w mailu (px); szersze wiersze
// są proporcjonalnie zmniejszane.
const SPONSORSMAILWIDTH = 560

// imageSize - wymiary obrazka (0, 0 gdy nie da się odczytać).
func imageSize(p string) (int, int) {
	f, err := os.Open(p)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// Sponsors wstawia w mail (albo w tekst po zamówieniu) sekcję sponsorów
// imprezy: tabelki zamiast flexa, bo tak rozumieją je programy pocztowe.
// W mailu logo idą jako obrazki cid:, na stronie i w podglądzie jako
// zwykłe adresy. Sekcja jest wstawiana bez względu na to, na których
// stronach gościa jest widoczna.
func (o *OrderFuncs) Sponsors() template.HTML {
	if o.DB == nil || o.EventID == 0 {
		return o.sponsorsPlaceholder()
	}
	event, err := o.DB.EventGetByID(o.EventID)
	if err != nil {
		log.Printf("OrderFuncs.Sponsors: event %d: %v", o.EventID, err)
		return ""
	}
	sp := ParseSponsors(event.Sponsors)
	if sp.Count() == 0 {
		return o.sponsorsPlaceholder()
	}
	lang := KnownLang(o.Lang)
	h := sp.Height()
	dir := filepath.Join(MEDIAROOT, o.User.URL)

	var b strings.Builder
	b.WriteString(`<div style="margin:28px 0 8px;text-align:center">`)
	fmt.Fprintf(&b, `<p style="margin:0 0 12px;font-size:12px;letter-spacing:.08em;text-transform:uppercase;color:#6e7885">%s</p>`,
		template.HTMLEscapeString(sp.TitleOr(lang)))
	type cell struct {
		src, link string
		w         int
	}
	for _, row := range sp.Rows {
		var all []cell
		var ws []int
		for _, l := range row {
			p := filepath.Join(dir, filepath.FromSlash(l.File))
			if _, err := os.Stat(p); err != nil {
				log.Printf("OrderFuncs.Sponsors: event %d: logo %s: %v", o.EventID, l.File, err)
				continue
			}
			w := 0
			if iw, ih := imageSize(p); iw > 0 && ih > 0 {
				w = iw * h / ih
			}
			c := cell{link: l.Link, w: w}
			if w > 0 {
				ws = append(ws, w)
			} else {
				ws = append(ws, 2*h)
			}
			if o.Preview || o.Web {
				c.src = sponsorSrc(o.User.URL, l.File)
			} else {
				c.src = "cid:" + filepath.Base(p)
				if !containsStr(o.EmbeddedImgs, p) {
					o.EmbeddedImgs = append(o.EmbeddedImgs, p)
				}
			}
			all = append(all, c)
		}
		// za szeroki wiersz dzielimy jak na stronie (splitSponsorRow), a co
		// i tak się nie mieści, proporcjonalnie zmniejszamy (komórka ma
		// 24 px paddingu, stąd węższa miara do podziału)
		for _, part := range splitSponsorRow(ws, 24, SPONSORSMAILWIDTH-24) {
			cells := all[part[0]:part[1]]
			if len(cells) == 0 {
				continue
			}
			total := 0
			for _, c := range cells {
				total += c.w + 24
			}
			rh := h
			if total > SPONSORSMAILWIDTH {
				rh = h * SPONSORSMAILWIDTH / total
				for i := range cells {
					cells[i].w = cells[i].w * SPONSORSMAILWIDTH / total
				}
			}
			b.WriteString(`<table role="presentation" cellpadding="0" cellspacing="0" border="0" align="center" style="margin:0 auto 8px;border-collapse:collapse"><tr>`)
			for _, c := range cells {
				size := fmt.Sprintf(`height="%d" style="display:block;border:0;height:%dpx`, rh, rh)
				if c.w > 0 {
					size = fmt.Sprintf(`width="%d" height="%d" style="display:block;border:0;width:%dpx;height:%dpx`, c.w, rh, c.w, rh)
				}
				img := fmt.Sprintf(`<img src="%s" alt="" %s">`, template.HTMLEscapeString(c.src), size)
				if c.link != "" {
					img = fmt.Sprintf(`<a href="%s" target="_blank">%s</a>`, template.HTMLEscapeString(c.link), img)
				}
				fmt.Fprintf(&b, `<td style="padding:6px 12px;vertical-align:middle">%s</td>`, img)
			}
			b.WriteString(`</tr></table>`)
		}
	}
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}

// sponsorsPlaceholder - w podglądzie maila bez ustawionych logo widać,
// gdzie sekcja będzie; w prawdziwym mailu nic.
func (o *OrderFuncs) sponsorsPlaceholder() template.HTML {
	if !o.Preview {
		return ""
	}
	return template.HTML(`<div style="margin:16px 0;padding:14px;border:2px dashed #b9c3cf;color:#6e7885;text-align:center">` +
		template.HTMLEscapeString(T(KnownLang(o.Lang), "sponsors.preview_empty")) + `</div>`)
}

func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// --- panel --------------------------------------------------------------------

// nSponsorsPick - dane edytora sekcji w panelu (szablon "sponsorspick").
type nSponsorsPick struct {
	RowsJSON     string // wiersze logo jako JSON dla js/n_sponsors.js
	Base         string // adres katalogu organizacji, np. /media/mskarwina/
	TitleDefault string // domyślny tytuł w języku imprezy (placeholder)
	Sponsors
}

func sponsorsPick(event Event, user User) nSponsorsPick {
	sp := ParseSponsors(event.Sponsors)
	rows := sp.Rows
	if rows == nil {
		rows = [][]SponsorLogo{}
	}
	js, _ := json.Marshal(rows)
	return nSponsorsPick{
		RowsJSON:     string(js),
		Base:         path.Join("/", MEDIAROOT, user.URL) + "/",
		TitleDefault: T(KnownLang(event.Language.String), "sponsors.title"),
		Sponsors:     sp,
	}
}

// sponsorsFromPanel - sekcja z pól zapisu sekcji "sponsors".
func sponsorsFromPanel(val func(string) string, flag func(string) bool) (Sponsors, error) {
	sp := Sponsors{
		Title: val("sponsors_title"),
		Size:  val("sponsors_size"),
		Seats: flag("sponsors_seats"),
		Order: flag("sponsors_order"),
		Done:  flag("sponsors_done"),
	}
	if rows := val("sponsors_rows"); rows != "" {
		if err := json.Unmarshal([]byte(rows), &sp.Rows); err != nil {
			return sp, err
		}
	}
	return sp.clean(), nil
}

// sponsorsSummary - linijka w nagłówku zwiniętej sekcji.
func sponsorsSummary(sp Sponsors, lang string) string {
	if sp.Count() == 0 {
		return T(lang, "common.none")
	}
	s := TN(lang, "count.logo", int64(sp.Count()))
	if len(sp.Rows) > 1 {
		s += " " + T(lang, "sponsors.in_rows", len(sp.Rows))
	}
	var where []string
	if sp.Seats {
		where = append(where, T(lang, "sponsors.on_seats"))
	}
	if sp.Order {
		where = append(where, T(lang, "sponsors.on_order"))
	}
	if sp.Done {
		where = append(where, T(lang, "sponsors.on_done"))
	}
	if len(where) == 0 {
		return s + " · " + T(lang, "sponsors.nowhere")
	}
	return s + " · " + strings.Join(where, ", ")
}

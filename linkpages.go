package main

// linkpages.go: strony z linkami - jedna strona organizacji z bannerem,
// nagłówkiem i tekstem (jak strona imprezy/formularza) oraz kilkoma dużymi
// przyciskami do rezerwacji, formularzy albo dowolnych adresów. Do strony
// jest kod QR (na plakaty, ulotki), więc gość z jednego skanu trafia
// wszędzie, gdzie trzeba.
//
// Panel: /admin/pages/{id} (sekcje jak przy formularzu), tworzenie
// i lista na stronie startowej. Gość: /p/{userurl}/{pageurl} - zawsze
// w języku strony (linkpages.language), tak jak imprezy i formularze.
// Trasy panelu wymagają org_manage (access.go: brak reguły = needOrg).
// Schemat: tools/migration-linkpages.sql.

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
)

// Rodzaje przycisku.
const (
	LINKEVENT = "event" // rezerwacja miejsc na imprezę
	LINKFORM  = "form"  // formularz
	LINKURL   = "url"   // dowolny adres
)

// LINKBUTTONSMAX - najwięcej przycisków na stronie; zwykle są 2-3.
const LINKBUTTONSMAX = 8

// LINKLABELMAX - najdłuższy napis na przycisku (znaki).
const LINKLABELMAX = 120

// LinkButton - przycisk strony (linkpages.buttons, JSON). Pusty napis =
// nazwa imprezy/formularza, więc zmiana nazwy zmienia też przycisk.
// Wygląd: Color z linkButtonColors ("" = kolor tematu), Size z
// linkButtonSizes ("" = zwykły), NoDate - bez daty imprezy pod napisem,
// Img - obrazek z media/<organizacja>/ (jak banner) nad napisem albo,
// z ImgOnly, zamiast napisu (napis zostaje tekstem alternatywnym).
type LinkButton struct {
	Kind    string `json:"kind"`
	ID      int64  `json:"id,omitempty"`
	URL     string `json:"url,omitempty"`
	Label   string `json:"label,omitempty"`
	Color   string `json:"color,omitempty"`
	Size    string `json:"size,omitempty"`
	NoDate  bool   `json:"nodate,omitempty"`
	Img     string `json:"img,omitempty"`
	ImgOnly bool   `json:"imgonly,omitempty"`
}

// linkButtonColors - kolory przycisku: "" = kolor tematu, "outline" = sama
// ramka w kolorze tematu, reszta jak kolory linijki nad nazwą (pagehead.go).
var linkButtonColors = []string{"", "outline", "ink", "red", "orange", "green", "blue", "purple"}

// linkButtonSizes - "s" mały, "" zwykły, "l" duży (na najważniejsze sprawy).
var linkButtonSizes = []string{"s", "", "l"}

// linkButtonImg - obrazek przycisku: ścieżka w katalogu organizacji bez
// wychodzenia z niego, tylko rozszerzenia obrazków; "" = brak.
func linkButtonImg(s string) string {
	f := ParseBanner(cleanBanner(s)).File
	if f == "" || !bannerExts[strings.ToLower(path.Ext(f))] {
		return ""
	}
	return f
}

// ParseLinkButtons czyta przyciski z bazy; zepsuty JSON = brak przycisków.
func ParseLinkButtons(s string) []LinkButton {
	var bb []LinkButton
	if strings.TrimSpace(s) != "" {
		if err := json.Unmarshal([]byte(s), &bb); err != nil {
			log.Printf("ParseLinkButtons: %v", err)
			return nil
		}
	}
	return bb
}

// linkButtonsString - postać do zapisu w bazie.
func linkButtonsString(bb []LinkButton) string {
	if len(bb) == 0 {
		return ""
	}
	b, _ := json.Marshal(bb)
	return string(b)
}

// cleanLinkURL przyjmuje tylko adresy http(s) i ścieżki tego serwera
// ("/form/…") - żadnych javascript: ani innych schematów. "" = zły adres.
func cleanLinkURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, " \t\r\n\\") {
		return ""
	}
	if strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//") {
		return s
	}
	if !strings.Contains(s, "://") {
		if strings.Contains(strings.SplitN(s, "/", 2)[0], ":") {
			return "" // "mailto:…", "javascript:…" - inny schemat
		}
		s = "https://" + s // "www.zori.cz" - admin zwykle nie pisze schematu
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.String()
}

// linkLabel - napis przycisku w jednej linii i nie za długi.
func linkLabel(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > LINKLABELMAX {
		s = string([]rune(s)[:LINKLABELMAX])
	}
	return s
}

// cleanLinkButtons sprawdza przyciski z panelu: znane rodzaje, imprezy
// i formularze tej organizacji, poprawne adresy. Zwraca błąd do pokazania
// adminowi (już przetłumaczony).
func cleanLinkButtons(db *DB, user User, lang string, in []LinkButton) ([]LinkButton, error) {
	var out []LinkButton
	for i, b := range in {
		n := i + 1
		b.Label = linkLabel(b.Label)
		b.Color = oneOf(b.Color, linkButtonColors)
		b.Size = oneOf(b.Size, linkButtonSizes)
		b.Img = linkButtonImg(b.Img)
		b.ImgOnly = b.ImgOnly && b.Img != ""
		b.NoDate = b.NoDate && b.Kind == LINKEVENT
		switch b.Kind {
		case LINKEVENT:
			e, err := db.EventGetByID(b.ID)
			if err != nil || e.UserID != user.ID {
				return nil, fmt.Errorf("%s", T(lang, "lpage.err_target", n))
			}
			b.URL = ""
		case LINKFORM:
			t, err := db.FormTemplateGetByID(b.ID, user.ID)
			if err != nil || isAddonsTemplate(t) {
				return nil, fmt.Errorf("%s", T(lang, "lpage.err_target", n))
			}
			b.URL = ""
		case LINKURL:
			b.ID = 0
			if b.URL = cleanLinkURL(b.URL); b.URL == "" {
				return nil, fmt.Errorf("%s", T(lang, "lpage.err_url", n))
			}
			if b.Label == "" {
				return nil, fmt.Errorf("%s", T(lang, "lpage.err_label", n))
			}
		default:
			return nil, fmt.Errorf("%s", T(lang, "lpage.err_target", n))
		}
		out = append(out, b)
	}
	if len(out) > LINKBUTTONSMAX {
		return nil, fmt.Errorf("%s", T(lang, "lpage.err_many", LINKBUTTONSMAX))
	}
	return out, nil
}

func linkPagePublicURL(user User, p LinkPage) string {
	return fmt.Sprintf("/p/%s/%s", user.URL, p.URL)
}

// uniqueLinkPageURL dopisuje -2, -3, ... gdy adres jest zajęty.
func uniqueLinkPageURL(db *DB, userID int64, base string) string {
	if base == "" {
		base = "strona"
	}
	u := base
	for i := 2; db.LinkPageURLTaken(u, userID, 0); i++ {
		u = fmt.Sprintf("%s-%d", base, i)
	}
	return u
}

// --- przyciski na stronie gościa i w panelu ---------------------------------

// nLinkButton - przycisk gotowy do pokazania.
type nLinkButton struct {
	Label   string
	Href    string
	Sub     string // druga linijka: data imprezy, "zapisy zamknięte"
	Ext     bool   // adres poza tym serwerem - otwiera się w nowej karcie
	Draft   bool   // szkic imprezy - widzi tylko admin (podgląd)
	Off     bool   // impreza/formularz nie istnieje - przycisk pomijamy
	Class   string // klasy wyglądu: " c-red sz-l img-only"
	Img     string // adres obrazka
	ImgOnly bool   // sam obrazek, napis jako alt
}

// resolveLinkButton zamienia zapisany przycisk na adres i napis w języku
// lang (strony gościa albo panelu).
func resolveLinkButton(db *DB, user User, b LinkButton, lang string) nLinkButton {
	out := nLinkButton{Label: b.Label}
	if c := oneOf(b.Color, linkButtonColors); c != "" {
		out.Class += " c-" + c
	}
	if sz := oneOf(b.Size, linkButtonSizes); sz != "" {
		out.Class += " sz-" + sz
	}
	if f := linkButtonImg(b.Img); f != "" {
		out.Img = path.Join("/", MEDIAROOT, user.URL, f)
		out.ImgOnly = b.ImgOnly
		out.Class += " with-img"
		if b.ImgOnly {
			out.Class += " img-only"
		}
	}
	switch b.Kind {
	case LINKEVENT:
		e, err := db.EventGetByID(b.ID)
		if err != nil || e.UserID != user.ID {
			out.Off = true
			return out
		}
		if out.Label == "" {
			out.Label = e.Name
		}
		out.Href = fmt.Sprintf("/res/%s?event-id=%d", user.URL, e.ID)
		out.Draft = e.Status.String == "draft"
		if e.Date > 0 && !b.NoDate {
			out.Sub = FormatDate(lang, e.Date)
		}
	case LINKFORM:
		t, err := db.FormTemplateGetByID(b.ID, user.ID)
		if err != nil {
			out.Off = true
			return out
		}
		if out.Label == "" {
			out.Label = t.Name
		}
		out.Href = fmt.Sprintf("/form/%s/%s", user.URL, t.URL)
		if !t.IsOpen(nowUnix()) {
			out.Sub = T(lang, "lpage.form_closed")
		}
	case LINKURL:
		out.Href = cleanLinkURL(b.URL)
		out.Ext = !strings.HasPrefix(out.Href, "/")
		out.Off = out.Href == ""
	default:
		out.Off = true
	}
	return out
}

// --- panel -----------------------------------------------------------------

type nLinkTarget struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Sub  string `json:"sub,omitempty"`
}

type nLinkPageVars struct {
	NPage
	ID        int64
	Name      string
	URL       string
	PublicURL string
	FullURL   string // z adresem serwera - to koduje QR
	Language  string
	Theme     string
	ThemeLogo bool
	Themes    []GuestTheme
	Banner    nBannerPick
	Head      nHeadPick
	Text      template.HTML
	Buttons   string // JSON dla js/n_linkpage.js
	Targets   string // JSON: imprezy i formularze do wyboru
	Max       int

	SumBasic, SumLook, SumButtons string
	MissButtons                   bool
}

func adminLinkPage(w http.ResponseWriter, r *http.Request, db *DB, cs *sessions.CookieStore, html bool) (User, LinkPage, bool) {
	return adminOwned(w, r, db, cs, html, "link page", func(id int64, u User) (LinkPage, error) {
		return db.LinkPageGetByID(id, u.ID)
	})
}

// linkTargets - imprezy (najnowsze na górze) i formularze organizacji
// do wyboru w przyciskach.
func linkTargets(db *DB, user User, lang string) string {
	events, err := db.EventGetAllByUserID(user.ID)
	if err != nil {
		log.Printf("linkTargets: events: %v", err)
	}
	evs := []nLinkTarget{}
	sortEventsNewest(events)
	for _, e := range events {
		t := nLinkTarget{ID: e.ID, Name: e.Name}
		if e.Date > 0 {
			t.Sub = FormatDate(lang, e.Date)
		}
		if e.Status.String == "draft" {
			t.Sub = strings.TrimPrefix(t.Sub+" · "+T(lang, "lpage.draft"), " · ")
		}
		evs = append(evs, t)
	}
	templs, err := db.FormTemplateGetAll(user.ID)
	if err != nil {
		log.Printf("linkTargets: forms: %v", err)
	}
	fs := []nLinkTarget{}
	for i := len(templs) - 1; i >= 0; i-- { // najnowsze na górze
		t := templs[i]
		if isAddonsTemplate(t) {
			continue
		}
		ft := nLinkTarget{ID: t.ID, Name: t.Name}
		if !t.IsOpen(nowUnix()) {
			ft.Sub = T(lang, "lpage.form_closed")
		}
		fs = append(fs, ft)
	}
	b, _ := json.Marshal(map[string]interface{}{"events": evs, "forms": fs})
	return string(b)
}

// sortEventsNewest - najpóźniejsza data pierwsza, imprezy bez daty na końcu.
func sortEventsNewest(ee []Event) {
	sort.SliceStable(ee, func(i, j int) bool {
		a, b := ee[i].Date, ee[j].Date
		if a <= 0 || b <= 0 {
			return a > 0 && b <= 0
		}
		return a > b
	})
}

func buildLinkPageVars(r *http.Request, db *DB, user User, p LinkPage, lang string) nLinkPageVars {
	v := nLinkPageVars{
		NPage: NPage{
			Title:   p.Name,
			Crumb:   p.Name,
			OrgName: orgName(user),
			UpURL:   "/admin",
			UpLabel: T(lang, "nav.home"),
		},
		ID:        p.ID,
		Name:      p.Name,
		URL:       p.URL,
		PublicURL: linkPagePublicURL(user, p),
		Language:  KnownLang(p.Language),
		Theme:     ThemeKey(p.Theme),
		ThemeLogo: ThemeLogo(p.Theme),
		Themes:    guestThemes,
		Banner:    mainBannerPick(T(lang, "field.main_banner"), "banner", p.Banner, user),
		Head:      headPick(ParsePageHead(p.PageHead), p.Name, false),
		Text:      template.HTML(p.Text),
		Buttons:   linkButtonsString(ParseLinkButtons(p.Buttons)),
		Targets:   linkTargets(db, user, lang),
		Max:       LINKBUTTONSMAX,
	}
	if v.Buttons == "" {
		v.Buttons = "[]"
	}
	v.FullURL = requestBase(r) + v.PublicURL
	v.SumBasic = v.PublicURL + " · " + languageName(lang, v.Language)
	v.SumLook = ThemeName(lang, v.Theme)
	if ParseBanner(p.Banner).File != "" {
		v.SumLook += " · " + T(lang, "lpage.with_banner")
	}
	v.SumButtons, v.MissButtons = linkButtonsSummary(db, user, p, lang)
	return v
}

// linkButtonsSummary - "Bal 2026, Zgłoszenie" albo "brak przycisków".
func linkButtonsSummary(db *DB, user User, p LinkPage, lang string) (string, bool) {
	var names []string
	broken := 0
	for _, b := range ParseLinkButtons(p.Buttons) {
		lb := resolveLinkButton(db, user, b, lang)
		if lb.Off {
			broken++
			continue
		}
		names = append(names, lb.Label)
	}
	if len(names) == 0 && broken == 0 {
		return T(lang, "lpage.no_buttons"), true
	}
	s := TN(lang, "count.button", int64(len(names)))
	if len(names) > 0 {
		s += ": " + strings.Join(names, ", ")
	}
	if broken > 0 {
		s += " · " + T(lang, "lpage.broken", broken)
	}
	return s, broken > 0 || len(names) == 0
}

// LinkPagePage - /admin/pages/{id}
func LinkPagePage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, p, ok := adminLinkPage(w, r, db, cs, true)
		if !ok {
			return
		}
		lang := PanelLang(r)
		nRender(w, r, lang, buildLinkPageVars(r, db, user, p, lang), "tmpl/n_linkpage.html")
	}
}

type nLinkPageCreateMsg struct {
	Name string `json:"name"`
}

// LinkPageCreateAPI - POST /api/pages {name}: nowa strona z adresem z nazwy,
// w języku panelu.
func LinkPageCreateAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		lang := PanelLang(r)
		var msg nLinkPageCreateMsg
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&msg); err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		name := headText(msg.Name)
		if name == "" {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "lpage.err_name")})
			return
		}
		p := LinkPage{UserID: user.ID, Name: name, Language: KnownLang(lang),
			URL: uniqueLinkPageURL(db, user.ID, slugify(name))}
		id, err := db.LinkPageAdd(&p)
		if err != nil {
			log.Printf("LinkPageCreateAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(lang, "common.save_error")})
			return
		}
		log.Printf("LinkPageCreateAPI: user %d created link page %d %q", user.ID, id, name)
		jsonMsg(w, http.StatusOK, map[string]interface{}{"id": id})
	}
}

// LinkPageDeleteAPI - DELETE /api/pages/{id}
func LinkPageDeleteAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, p, ok := adminLinkPage(w, r, db, cs, false)
		if !ok {
			return
		}
		if err := db.LinkPageDelete(p.ID, user.ID); err != nil {
			log.Printf("LinkPageDeleteAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(PanelLang(r), "del.failed")})
			return
		}
		log.Printf("LinkPageDeleteAPI: user %d deleted link page %d %q", user.ID, p.ID, p.Name)
		jsonMsg(w, http.StatusOK, map[string]string{"msg": "deleted", "next": "/admin"})
	}
}

// LinkPageSectionSave - POST /api/pages/{id}/section (jak sekcje formularza).
func LinkPageSectionSave(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, p, ok := adminLinkPage(w, r, db, cs, false)
		if !ok {
			return
		}
		lang := PanelLang(r)
		var msg nSectionMsg
		if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&msg); err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		val := func(key string) string {
			s, _ := msg.Values[key].(string)
			return strings.TrimSpace(s)
		}
		flag := func(key string) bool {
			b, _ := msg.Values[key].(bool)
			return b
		}
		bad := func(m string) { jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": m}) }

		switch msg.Section {
		case "basic":
			if name := headText(val("name")); name != "" {
				// nagłówek idący za nazwą zostaje przy nazwie
				h := ParsePageHead(p.PageHead)
				if h.Name == headText(name) {
					h.Name = ""
					p.PageHead = h.String()
				}
				p.Name = name
			}
			if u := val("url"); u != "" && u != p.URL {
				if !formURLRe.MatchString(u) {
					bad(T(lang, "newform.bad_url"))
					return
				}
				if db.LinkPageURLTaken(u, user.ID, p.ID) {
					bad(T(lang, "lpage.url_taken", u))
					return
				}
				p.URL = u
			}
			p.Language = KnownLang(val("language"))
		case "look":
			p.Theme = ThemeValue(val("theme"), flag("theme_logo"))
			p.Banner = cleanBanner(val("banner"))
			p.PageHead = pageHeadFromPanel(val, flag, p.Name).String()
			if s, ok := msg.Values["text"].(string); ok {
				p.Text = removeTrailingEmptyParagraphs(s)
				if emptyHTML(p.Text) {
					p.Text = ""
				}
			}
		case "buttons":
			var in []LinkButton
			if err := json.Unmarshal([]byte(val("buttons")), &in); err != nil && val("buttons") != "" {
				bad(err.Error())
				return
			}
			bb, err := cleanLinkButtons(db, user, lang, in)
			if err != nil {
				bad(err.Error())
				return
			}
			p.Buttons = linkButtonsString(bb)
		default:
			bad("unknown section " + msg.Section)
			return
		}

		if err := db.LinkPageMod(&p); err != nil {
			log.Printf("LinkPageSectionSave: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(lang, "common.save_error") + ": " + err.Error()})
			return
		}
		v := buildLinkPageVars(r, db, user, p, lang)
		resp := map[string]interface{}{"msg": "saved", "title": p.Name, "url": v.PublicURL, "full_url": v.FullURL}
		switch msg.Section {
		case "basic":
			resp["summary"] = v.SumBasic
		case "look":
			resp["summary"] = v.SumLook
		case "buttons":
			resp["summary"], resp["missing"] = v.SumButtons, v.MissButtons
		}
		jsonMsg(w, http.StatusOK, resp)
	}
}

// emptyHTML - tekst z edytora bez treści ("<p><br></p>").
func emptyHTML(s string) bool {
	return !strings.Contains(s, "<img") && strings.TrimSpace(reallyEmpty(s)) == ""
}

// QR: wielkość modułu w pikselach - przy ~30 modułach daje obrazek ~800 px,
// wystarczający do druku na plakacie A3.
const LINKQRMODULE = 24

// LINKQRQUIET - cisza wokół kodu w modułach (norma wymaga 4).
const LINKQRQUIET = 4

// LinkPageQRAPI - GET /api/pages/{id}/qr.png|qr.svg[?download=1]: kod QR
// z pełnym adresem strony (adres serwera z żądania, jak linki w mailach).
// PNG ma białe tło, SVG jest przezroczysty (na kolorowe plakaty, skaluje
// się bez utraty ostrości).
func LinkPageQRAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, p, ok := adminLinkPage(w, r, db, cs, false)
		if !ok {
			return
		}
		ext := mux.Vars(r)["ext"]
		if ext != "svg" {
			ext = "png"
		}
		full := requestBase(r) + linkPagePublicURL(user, p)
		qrc, err := qrcode.NewWith(full, qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionQuart))
		if err != nil {
			log.Printf("LinkPageQRAPI: %v", err)
			http.Error(w, "qr error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Query().Get("download") != "" {
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="qr-%s-%s.%s"`, user.URL, p.URL, ext))
		}
		var wr qrcode.Writer
		if ext == "svg" {
			w.Header().Set("Content-Type", "image/svg+xml")
			wr = qrSVGWriter{w}
		} else {
			w.Header().Set("Content-Type", "image/png")
			wr = standard.NewWithWriter(nopCloser{w},
				standard.WithBuiltinImageEncoder(standard.PNG_FORMAT),
				standard.WithFgColorRGBHex("#000000"), // domyślny szary słabo wychodzi w druku
				standard.WithQRWidth(LINKQRMODULE), standard.WithBorderWidth(LINKQRMODULE*2))
		}
		if err := qrc.Save(wr); err != nil {
			log.Printf("LinkPageQRAPI: save: %v", err)
		}
	}
}

// qrSVGWriter rysuje kod QR jako SVG: czarne moduły na przezroczystym tle,
// jeden moduł = 1 jednostka, sąsiednie moduły w rzędzie sklejone w jeden
// prostokąt (mniejszy plik, bez szczelin przy skalowaniu).
type qrSVGWriter struct{ w io.Writer }

func (q qrSVGWriter) Write(mat qrcode.Matrix) error {
	bm := mat.Bitmap()
	size := len(bm) + 2*LINKQRQUIET
	var d strings.Builder
	for y, row := range bm {
		for x := 0; x < len(row); {
			if !row[x] {
				x++
				continue
			}
			start := x
			for x < len(row) && row[x] {
				x++
			}
			fmt.Fprintf(&d, "M%d %dh%dv1h-%dz", start+LINKQRQUIET, y+LINKQRQUIET, x-start, x-start)
		}
	}
	_, err := fmt.Fprintf(q.w, `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" shape-rendering="crispEdges"><path fill="#000" d="%s"/></svg>
`, size, size, size*LINKQRMODULE/2, size*LINKQRMODULE/2, d.String())
	return err
}

func (qrSVGWriter) Close() error { return nil }

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

// --- strona gościa ---------------------------------------------------------

type nGuestLinkVars struct {
	Title   string
	Head    GuestHead
	Text    template.HTML
	Buttons []nLinkButton
	Preview bool // admin ogląda: szkice imprez widać z dopiskiem
}

// GuestLinkPage - /p/{userurl}/{pageurl}
func GuestLinkPage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := DEFAULTLANG
		userURL := mux.Vars(r)["userurl"]
		user, err := db.UserGetByURL(userURL)
		if err != nil {
			log.Printf("GuestLinkPage: unknown organization %q: %v", userURL, err)
			ErrorHTMLNotFound(T(lang, "gform.no_org_title"), T(lang, "gform.no_org", userURL), lang, w, r)
			return
		}
		p, err := db.LinkPageGetByURL(mux.Vars(r)["pageurl"], user.ID)
		if err != nil {
			ol := orgLang(db, user.ID, lang)
			ErrorHTMLNotFound(T(ol, "about.not_found_title"), T(ol, "about.not_found"), ol, w, r)
			return
		}
		lng := KnownLang(p.Language)
		preview := isFormOwnerSession(r, db, cs, user)

		vars := nGuestLinkVars{
			Head:    guestHead(ParsePageHead(p.PageHead), p.Name, p.Banner, user.URL, ""),
			Text:    template.HTML(p.Text),
			Preview: preview,
		}
		vars.Title = vars.Head.Name
		for _, b := range ParseLinkButtons(p.Buttons) {
			lb := resolveLinkButton(db, user, b, lng)
			if lb.Off || (lb.Draft && !preview) {
				continue
			}
			if lb.Draft {
				lb.Sub = strings.TrimPrefix(lb.Sub+" · "+T(lng, "lpage.draft_guest"), " · ")
			}
			vars.Buttons = append(vars.Buttons, lb)
		}
		guestRenderTheme(w, lng, p.Theme, vars, "tmpl/n_linkpage_guest.html")
	}
}

// --- strona startowa ---------------------------------------------------------

// homeLinkPages - wiersze listy stron na stronie startowej.
func homeLinkPages(db *DB, user User, lang string) []nHomeRow {
	pages, err := db.LinkPageGetAll(user.ID)
	if err != nil {
		log.Printf("homeLinkPages: %v", err)
	}
	var rows []nHomeRow
	for i, p := range pages {
		sum, miss := linkButtonsSummary(db, user, p, lang)
		row := nHomeRow{ID: p.ID, Name: p.Name, DateS: linkPagePublicURL(user, p), Info: sum, Old: i >= listFold}
		if miss {
			row.Missing = sum
			row.Info = ""
		}
		rows = append(rows, row)
	}
	return rows
}

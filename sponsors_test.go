package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSponsorsClean(t *testing.T) {
	sp := Sponsors{
		Size: "xl", // nieznana wielkość = średnia
		Rows: [][]SponsorLogo{
			{{File: "../../etc/passwd.png"}, {File: "logo.svg"}, {File: "a.png", Link: "javascript:alert(1)"}},
			{},
			{{File: "/b.JPG", Link: "firma.cz"}, {File: "c.webp", Link: "http://x.pl/a?b=1"}},
		},
	}.clean()
	if sp.Size != "" {
		t.Errorf("size %q", sp.Size)
	}
	if len(sp.Rows) != 2 {
		t.Fatalf("rows %+v", sp.Rows)
	}
	// ścieżka nie wychodzi poza katalog organizacji, svg odpada
	if got := sp.Rows[0]; len(got) != 2 || got[0].File != "etc/passwd.png" || got[1].Link != "" {
		t.Errorf("row 0 %+v", got)
	}
	if got := sp.Rows[1]; got[0].File != "b.JPG" || got[0].Link != "https://firma.cz" || got[1].Link != "http://x.pl/a?b=1" {
		t.Errorf("row 1 %+v", got)
	}
	if ParseSponsors(sp.String()).Count() != 4 {
		t.Errorf("round trip %s", sp.String())
	}
	if (Sponsors{}).String() != "" || ParseSponsors("{zły").Count() != 0 {
		t.Error("empty / bad JSON")
	}
}

// Sekcja jest tylko na stronach zaznaczonych w panelu, w języku imprezy.
func TestSponsorsOnGuestPages(t *testing.T) {
	f := newGuestFixture(t)
	sp := Sponsors{Seats: true, Rows: [][]SponsorLogo{{{File: "logo-a.png", Link: "https://a.cz"}}, {{File: "logo-b.png"}}}}
	must(t)(0, f.db.EventSetSponsors(f.eventID, f.userID, sp.String()))
	for _, lang := range allLangs {
		f.setLang(t, lang)
		body := f.do(t, httptest.NewRequest("GET", "/res/org", nil), lang)
		if !strings.Contains(body, `class="sponsors"`) || !strings.Contains(body, "/media/org/logo-b.png") ||
			!strings.Contains(body, `href="https://a.cz"`) {
			t.Errorf("[%s] seats page without sponsors", lang)
		}
		wantLang(t, "res", body, lang, "sponsors.title")
		if strings.Contains(f.orderPage(t, lang), `class="sponsors"`) {
			t.Errorf("[%s] order page shows sponsors although not enabled", lang)
		}
	}

	sp.Seats, sp.Order, sp.Title = false, true, "Partnerzy"
	must(t)(0, f.db.EventSetSponsors(f.eventID, f.userID, sp.String()))
	if strings.Contains(f.do(t, httptest.NewRequest("GET", "/res/org", nil), "en"), `class="sponsors"`) {
		t.Error("seats page shows sponsors although disabled")
	}
	if body := f.orderPage(t, "en"); !strings.Contains(body, "Partnerzy") {
		t.Error("order page without sponsors / own title")
	}
}

// W mailu logo idą jako cid: z plikami do dołączenia (brakujący plik
// wypada, żeby wysyłka się nie wywróciła), na stronie jako adresy.
func TestSponsorsMail(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	db := newTestDB(t)
	userURL := "sponsors-test-org"
	userID := addUser(t, db, "sp@example.com", userURL)
	eventID := addEvent(t, db, userID, "Bal")
	dir := filepath.Join(MEDIAROOT, userURL)
	must(t)(0, os.MkdirAll(dir, 0o755))
	t.Cleanup(func() { os.RemoveAll(dir) })
	var buf bytes.Buffer
	must(t)(0, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 300, 100))))
	must(t)(0, os.WriteFile(filepath.Join(dir, "logo.png"), buf.Bytes(), 0o644))

	sp := Sponsors{Size: "s", Rows: [][]SponsorLogo{{{File: "logo.png", Link: "https://a.cz"}, {File: "brak.png"}}}}
	must(t)(0, db.EventSetSponsors(eventID, userID, sp.String()))
	user, err := db.UserGetByID(userID)
	if err != nil {
		t.Fatal(err)
	}

	of := &OrderFuncs{EventID: eventID, DB: db, User: user, Lang: "cs"}
	html := string(of.Sponsors())
	if !strings.Contains(html, `src="cid:logo.png"`) || !strings.Contains(html, `width="144" height="48"`) ||
		!strings.Contains(html, "Sponzoři") || !strings.Contains(html, `href="https://a.cz"`) {
		t.Errorf("mail html: %s", html)
	}
	if strings.Contains(html, "brak.png") || len(of.EmbeddedImgs) != 1 {
		t.Errorf("embedded %v, html %s", of.EmbeddedImgs, html)
	}
	of.Sponsors() // druga pastylka nie dokłada tego samego pliku
	if len(of.EmbeddedImgs) != 1 {
		t.Errorf("embedded twice: %v", of.EmbeddedImgs)
	}

	web := &OrderFuncs{EventID: eventID, DB: db, User: user, Lang: "pl", Web: true}
	if html := string(web.Sponsors()); !strings.Contains(html, `src="/media/sponsors-test-org/logo.png"`) || len(web.EmbeddedImgs) != 0 {
		t.Errorf("web html: %s", html)
	}

	// pastylka działa w treści maila
	out := renderOrderTmpl("<p>Dziękujemy</p>{{.Sponsors}}", &OrderFuncs{EventID: eventID, DB: db, User: user, Lang: "pl"})
	if !strings.Contains(out, "cid:logo.png") {
		t.Errorf("template: %s", out)
	}
}

func TestSplitSponsorRow(t *testing.T) {
	for _, c := range []struct {
		ws   []int
		maxW int
		want string
	}{
		{[]int{100, 100, 100}, 400, "[[0 3]]"},                       // mieści się
		{[]int{100, 100, 100, 100, 100, 100}, 500, "[[0 3] [3 6]]"},  // 3+3, nie 4+2
		{[]int{100, 100, 100, 100, 100}, 300, "[[0 1] [1 3] [3 5]]"}, // 1+2+2
		{[]int{300, 100, 100, 100}, 400, "[[0 1] [1 4]]"},            // szerokie logo samo
		{[]int{900, 100}, 500, "[[0 1] [1 2]]"},                      // nie mieści się nawet jedno
		{nil, 500, "[[0 0]]"},
	} {
		if got := fmt.Sprint(splitSponsorRow(c.ws, 20, c.maxW)); got != c.want {
			t.Errorf("splitSponsorRow(%v, %d) = %s, want %s", c.ws, c.maxW, got, c.want)
		}
	}
}

func TestSponsorSeatsHeight(t *testing.T) {
	if h := sponsorSeatsHeight([]int{200, 200}, 48); h != 48 {
		t.Errorf("fitting row shrunk to %d", h)
	}
	// 8 logo, razem 1328 px: na 1128 - 7*32 = 904 px zostaje 48*904/1328
	if h := sponsorSeatsHeight([]int{212, 143, 142, 198, 142, 119, 177, 195}, 48); h != 32 {
		t.Errorf("wide row height %d, want 32", h)
	}
}

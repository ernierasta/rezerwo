package main

// guest.go: nowa strona rezerwacji dla gościa (etap 4 redesignu).
//
// Adres /res/{user}; wybrane miejsca idą do kroku zamówienia /order
// (guest_order.go).

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// nGuestRoom to sala z policzoną geometrią i tekstami dla szablonu.
type nGuestRoom struct {
	svgRoom
	Tables  []nGuestFurniture
	Chairs  []nGuestFurniture
	Objects []nGuestFurniture
	Labels  []nGuestFurniture
}

// nGuestFurniture dokłada do elementu pola wyliczane dla SVG (środek, opis).
type nGuestFurniture struct {
	svgFurniture
	CX, CY float64
	RX, RY float64
	TextY  float64
	Table  int64
	Title  string
	// LabelInk - kolor podpisu na obiekcie, czytelny na jego tle
	LabelInk string
}

type nGuestVars struct {
	Title           string
	EventID         int64
	EventName       string
	When            string
	Banner          template.HTML
	Desc            template.HTML
	HowTo           template.HTML
	Rooms           []nGuestRoom
	DefaultPrice    int64
	DefaultCurrency string
	HideCosts       bool
	UserURL         string
	SoldOut         bool // żadnego wolnego miejsca we wszystkich salach

	// tekst pokazywany, gdy gość klika "Zamawiam" bez wybranych miejsc
	NoSitsTitle string
	NoSitsText  template.HTML
}

// isAdminSession sprawdza, czy żądanie ma ważną sesję admina - bez
// przekierowań, bo na stronie gościa nie chcemy nikogo nigdzie wysyłać.
// Służy tylko do podglądu szkiców imprez z panelu.
func isAdminSession(r *http.Request, cs *sessions.CookieStore) bool {
	return sessionAdmin(r, cs) != ""
}

// orgLang to język strony organizacji, gdy nie ma otwartej imprezy:
// język jej najnowszej imprezy, a bez imprez domyślny.
func orgLang(db *DB, userID int64, fallback string) string {
	events, err := db.EventGetAllByUserID(userID)
	if err != nil {
		return KnownLang(fallback)
	}
	var newest Event
	for _, e := range events {
		if e.ID > newest.ID {
			newest = e
		}
	}
	if newest.Language.String == "" {
		return KnownLang(fallback)
	}
	return KnownLang(newest.Language.String)
}

// GuestReservationPage - /res/{user}
func GuestReservationPage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := DEFAULTLANG // organizacja nieznana albo bez imprez
		userURL := mux.Vars(r)["user"]
		user, err := db.UserGetByURL(userURL)
		if err != nil {
			log.Printf("GuestReservationPage: unknown organization %q: %v", userURL, err)
			// bez organizacji nie ma imprezy, więc nie ma też jej języka
			ErrorHTML(T(lang, "gform.no_org_title"), T(lang, "gform.no_org", userURL), lang, w, r)
			return
		}

		// ?event-id=... pozwala obejrzeć konkretną imprezę tej organizacji.
		// Używa tego podgląd "Strona dla gości" w panelu - bez tego admin
		// zobaczyłby tylko imprezę z otwartą sprzedażą (EventGetCurrent).
		var event Event
		if ids := r.URL.Query().Get("event-id"); ids != "" {
			id, cerr := strconv.ParseInt(ids, 10, 64)
			if cerr == nil {
				event, err = db.EventGetByID(id)
				if err == nil && event.UserID != user.ID {
					err = fmt.Errorf("event %d does not belong to %q", id, userURL)
				}
			} else {
				err = cerr
			}
		} else {
			event, err = EventGetCurrent(db, user.ID)
		}
		if err != nil {
			log.Printf("GuestReservationPage: no event for user %d: %v", user.ID, err)
			ol := orgLang(db, user.ID, lang)
			ErrorHTML(T(ol, "guest.no_event_title"), T(ol, "guest.no_event"), ol, w, r)
			return
		}

		// strona gościa jest zawsze w języku imprezy (rozdz. 10.2 HARMONIJKA.md)
		lng := KnownLang(event.Language.String)

		// szkic widzi tylko zalogowany admin (podgląd z panelu)
		if event.Status.String == "draft" && !isAdminSession(r, cs) {
			log.Printf("GuestReservationPage: draft event %d hidden from guests", event.ID)
			ErrorHTML(T(lng, "guest.draft_title"), T(lng, "guest.draft_text"), lng, w, r)
			return
		}

		rooms, err := db.EventGetRooms(event.ID)
		if err != nil || len(rooms) == 0 {
			log.Printf("GuestReservationPage: no rooms for event %d: %v", event.ID, err)
			ErrorHTML(T(lng, "guest.no_rooms_title"), T(lng, "guest.no_rooms"), lng, w, r)
			return
		}

		vars := nGuestVars{
			Title:           event.Name,
			EventID:         event.ID,
			EventName:       event.Name,
			When:            FormatDate(lng, event.Date),
			Banner:          ParseBanner(event.MainBanner.String).ImgHTML(user.URL),
			Desc:            template.HTML(event.MainDesc.String),
			HowTo:           template.HTML(event.HowTo),
			DefaultPrice:    event.DefaultPrice,
			DefaultCurrency: event.DefaultCurrency,
			HideCosts:       event.HideCosts.Bool,
			UserURL:         user.URL,
			NoSitsTitle:     event.NoSitsSelectedTitle,
			NoSitsText:      template.HTML(event.NoSitsSelectedText),
		}
		if vars.NoSitsTitle == "" {
			vars.NoSitsTitle = T(lng, "guest.no_seats_title")
		}

		roomDescs := []string{event.Room1Desc.String, event.Room2Desc.String, event.Room3Desc.String, event.Room4Desc.String}
		roomBanners := []string{event.Room1Banner.String, event.Room2Banner.String, event.Room3Banner.String, event.Room4Banner.String}

		for i, room := range rooms {
			gr := guestRoom(db, room, event, lng)

			// banner sali pokazujemy tylko wtedy, gdy nie ma głównego bannera
			if i < len(roomDescs) {
				gr.Desc = template.HTML(roomDescs[i])
			}
			if event.MainBanner.String == "" && i < len(roomBanners) {
				gr.Banner = ParseBanner(roomBanners[i]).ImgHTML(user.URL)
			}
			vars.Rooms = append(vars.Rooms, gr)
		}
		free, total := 0, 0
		for _, gr := range vars.Rooms {
			free += gr.SeatsFree
			total += gr.SeatsTotal
		}
		vars.SoldOut = total > 0 && free == 0

		t, err := template.New("n_reservation.html").Funcs(GuestFuncs(lng, event.Theme.String)).
			ParseFiles("tmpl/n_reservation.html", "tmpl/n_guest_base.html")
		if err != nil {
			log.Printf("GuestReservationPage: template parse: %v", err)
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}
		if err := t.ExecuteTemplate(w, "base", vars); err != nil {
			log.Printf("GuestReservationPage: template exec: %v", err)
		}
	}
}

// guestRoom liczy geometrię sali i dokłada dane potrzebne w szablonie.
func guestRoom(db *DB, room Room, event Event, lang string) nGuestRoom {
	sr := buildSVGRoom(db, room, event)
	gr := nGuestRoom{svgRoom: sr}

	for _, t := range sr.Tables {
		gr.Tables = append(gr.Tables, guestFurniture(t, ""))
	}
	for _, o := range sr.Objects {
		gf := guestFurniture(o, "")
		gf.LabelInk = contrastInk(o.Color)
		gr.Objects = append(gr.Objects, gf)
	}
	for _, l := range sr.Labels {
		gf := guestFurniture(l, "")
		// napisy rysujemy od lewej krawędzi, więc tekst siada na linii bazowej
		gf.TextY = l.Y + svgChairSize*0.8
		gr.Labels = append(gr.Labels, gf)
	}

	for _, c := range sr.Chairs {
		table := tableOfChair(c, sr.Tables)
		title := ""
		switch c.Status {
		case "free":
			if event.HideCosts.Bool || c.Price == 0 {
				title = seatTitle(lang, table, c.Number)
			} else {
				title = seatTitle(lang, table, c.Number) + ", " + FormatMoney(c.Price, c.Currency)
			}
		case "taken":
			title = seatTitle(lang, table, c.Number) + ", " + T(lang, "guest.taken")
		default:
			title = seatTitle(lang, table, c.Number) + ", " + T(lang, "guest.unavailable")
		}

		gf := guestFurniture(c, title)
		gf.Table = table
		gr.Chairs = append(gr.Chairs, gf)
	}

	return gr
}

func seatTitle(lang string, table, number int64) string {
	if table > 0 {
		return T(lang, "guest.seat", table, number)
	}
	return fmt.Sprintf("%d", number)
}

func guestFurniture(f svgFurniture, title string) nGuestFurniture {
	return nGuestFurniture{
		svgFurniture: f,
		CX:           f.X + f.W/2,
		CY:           f.Y + f.H/2,
		RX:           f.W / 2,
		RY:           f.H / 2,
		// tekst wyśrodkowany w pionie: linia bazowa nieco poniżej środka
		TextY: f.Y + f.H/2 + 4,
		Title: title,
	}
}

// tableOfChair znajduje stół, przy którym stoi krzesło - po najbliższym
// stole w promieniu jednego "kroku" krzesła. Numer stołu jest tylko opisem
// dla gościa ("stół 7, miejsce 5"), nie ma go w bazie przy krześle.
func tableOfChair(chair svgFurniture, tables []svgFurniture) int64 {
	const reach = svgChairSize + svgTableGap + svgChairSize // krzesło może stać za sąsiednim krzesłem

	cx, cy := chair.X+chair.W/2, chair.Y+chair.H/2
	var best int64
	bestDist := -1.0

	for _, t := range tables {
		dx := 0.0
		if cx < t.X {
			dx = t.X - cx
		} else if cx > t.X+t.W {
			dx = cx - (t.X + t.W)
		}
		dy := 0.0
		if cy < t.Y {
			dy = t.Y - cy
		} else if cy > t.Y+t.H {
			dy = cy - (t.Y + t.H)
		}
		dist := dx + dy
		if dist > reach {
			continue
		}
		if bestDist < 0 || dist < bestDist {
			bestDist = dist
			best = t.Number
		}
	}
	return best
}

// TODO: add support for multiple active events
func EventGetCurrent(db *DB, userID int64) (Event, error) {
	events, err := db.EventGetAllByUserID(userID)
	if err != nil {
		return Event{}, err
	}
	now := time.Now().Unix()
	for i := range events {
		// szkice nie są widoczne dla gości
		if events[i].Status.String == "draft" {
			continue
		}
		// return first active user event
		if events[i].FromDate < now && events[i].ToDate > now {
			return events[i], nil
		}
	}
	return Event{}, fmt.Errorf("no active event found for userID: %d", userID)
}

package main

// guest_order.go: krok zamówienia na nowej stronie gościa (/n/order).
//
// Robi to samo co stare /order: zapisuje wybrane miejsca ze statusem "marked"
// (ticker.go kasuje je po 5 minutach) i pokazuje formularz danych osoby.
// Formularz idzie dalej do istniejącego /order/status, więc reszta procesu
// (klient, maile, bilety) zostaje bez zmian. Hasła nie ma - logowanie gościa
// jest odłożone (rozdz. 8.3 HARMONIJKA.md).

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// MARKEDMINUTES musi zgadzać się z czasem kasowania blokad w ticker.go.
const MARKEDMINUTES = 5

type nOrderVars struct {
	Title     string
	EventID   int64
	EventName string
	UserURL   string
	Minutes   int

	Sits            string
	SitsLabel       string
	Rooms           string
	Prices          string
	TotalPrice      string
	TotalPriceLabel string
	HideCosts       bool

	HowTo     template.HTML
	NotesHelp string

	// dodatki do zamówienia (rozdz. 9 HARMONIJKA.md)
	Addons   []nOrderAddon
	Currency string
}

// nOrderAddon to jedno pole dodatków gotowe do wyświetlenia: pola powtarzane
// wg liczby miejsc pojawiają się tyle razy, ile miejsc ("Kolacja 1", "Kolacja 2").
type nOrderAddon struct {
	AddonField
	Repeat int64
	Input  string // nazwa pola w formularzu
	Title  string // etykieta z numerem powtórzenia
}

// orderAddonFields rozwija definicję dodatków na pola do wyświetlenia.
func orderAddonFields(fields []AddonField, seats int64) []nOrderAddon {
	var out []nOrderAddon
	for _, f := range fields {
		if f.PerSeat && seats > 0 {
			for i := int64(1); i <= seats; i++ {
				out = append(out, nOrderAddon{
					AddonField: f,
					Repeat:     i,
					Input:      AddonInputName(f, i),
					Title:      fmt.Sprintf("%s %d", f.Label, i),
				})
			}
			continue
		}
		out = append(out, nOrderAddon{
			AddonField: f,
			Input:      AddonInputName(f, 0),
			Title:      f.Label,
		})
	}
	return out
}

// GuestOrderPage - POST /n/order
func GuestOrderPage(db *DB, lang string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if err := r.ParseForm(); err != nil {
			log.Printf("GuestOrderPage: parse form: %v", err)
		}

		eventID, err := strconv.ParseInt(r.FormValue("event-id"), 10, 64)
		if err != nil {
			log.Printf("GuestOrderPage: bad event id %q: %v", r.FormValue("event-id"), err)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		sits := r.FormValue("sits")
		rooms := r.FormValue("rooms")
		prices := r.FormValue("prices")
		totalPrice := r.FormValue("total-price")
		currency := r.FormValue("default-currency")

		event, err := db.EventGetByID(eventID)
		if err != nil {
			log.Printf("GuestOrderPage: event %d: %v", eventID, err)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		lng := KnownLang(event.Language.String)

		user, err := db.UserGetByID(event.UserID)
		if err != nil {
			log.Printf("GuestOrderPage: user of event %d: %v", eventID, err)
		}

		ss, rr, pp, err := SplitSitsRoomsPrices(sits, rooms, prices)
		if err != nil {
			log.Printf("GuestOrderPage: %v", err)
			ErrorHTML(T(lng, "guest.no_seats_title"), T(lng, "guest.nothing_selected"), lng, w, r)
			return
		}

		// blokada miejsc: status "marked" do czasu potwierdzenia zamówienia
		var labels []string
		for i := range ss {
			chair, err := db.FurnitureGetByTypeNumberRoom("chair", ss[i], rr[i])
			if err != nil {
				log.Printf("GuestOrderPage: chair %d in room %d: %v", ss[i], rr[i], err)
			}
			_, err = db.ReservationAdd(&Reservation{
				OrderedDate: ToNI(time.Now().Unix()),
				Price:       ToNI(pp[i]),
				Currency:    ToNS(currency),
				Status:      "marked",
				FurnitureID: chair.ID,
				EventID:     eventID,
				CustomerID:  -1, // uzupełniane przy tworzeniu klienta
			})
			if err != nil {
				log.Printf("GuestOrderPage: seat %d already taken (event %d): %v", ss[i], eventID, err)
				ErrorHTML(T(lng, "order.seats_taken"), T(lng, "order.seats_taken_text"), lng, w, r)
				return
			}
			labels = append(labels, strconv.FormatInt(ss[i], 10))
		}

		vars := nOrderVars{
			Title:      event.Name,
			EventID:    event.ID,
			EventName:  event.Name,
			UserURL:    user.URL,
			Minutes:    MARKEDMINUTES,
			Sits:       sits,
			SitsLabel:  strings.Join(labels, ", "),
			Rooms:      rooms,
			Prices:     prices,
			TotalPrice: totalPrice,
			HideCosts:  event.HideCosts.Bool,
			HowTo:      template.HTML(event.OrderHowto),
			NotesHelp:  event.OrderNotesDescription,
			Addons:     orderAddonFields(EventAddonFields(db, event), int64(len(ss))),
			Currency:   currency,
		}

		if n, err := strconv.ParseInt(totalPrice, 10, 64); err == nil {
			vars.TotalPriceLabel = FormatMoney(n, currency)
		} else {
			vars.TotalPriceLabel = fmt.Sprintf("%s %s", totalPrice, currency)
		}

		t, err := template.New("n_order.html").Funcs(TmplFuncs(lng)).
			ParseFiles("tmpl/n_order.html", "tmpl/n_guest_base.html")
		if err != nil {
			log.Printf("GuestOrderPage: template parse: %v", err)
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}
		if err := t.ExecuteTemplate(w, "base", vars); err != nil {
			log.Printf("GuestOrderPage: template exec: %v", err)
		}
	}
}

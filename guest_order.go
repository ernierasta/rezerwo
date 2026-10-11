package main

// guest_order.go: krok zamówienia na nowej stronie gościa (/order).
//
// Robi to samo co stare /order: zapisuje wybrane miejsca ze statusem "marked"
// (ticker.go kasuje je po 5 minutach) i pokazuje formularz danych osoby.
// Formularz idzie dalej do /order/status (ReservationOrderStatusHTML): klient,
// zamówienie, maile i strona z podziękowaniem. Hasła nie ma - logowanie gościa
// jest odłożone (rozdz. 8.3 HARMONIJKA.md).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/sessions"
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
	// Lock - podpisane numery zablokowanych rezerwacji (orderLock)
	Lock      string
	HideCosts bool

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

// GuestOrderPage - POST /order
func GuestOrderPage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
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

		// ceny i suma z formularza są tylko do porównania - liczymy je z bazy
		sits := r.FormValue("sits")
		rooms := r.FormValue("rooms")

		event, err := db.EventGetByID(eventID)
		if err != nil {
			log.Printf("GuestOrderPage: event %d: %v", eventID, err)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		lng := KnownLang(event.Language.String)

		// szkic albo sprzedaż zamknięta: tylko admin (np. sprzedaż przy wejściu)
		if !eventOnSale(event, nowUnix()) && !isAdminSession(r, cs) {
			log.Printf("GuestOrderPage: event %d not on sale", eventID)
			ErrorHTML(T(lng, "guest.no_event_title"), T(lng, "guest.no_event"), lng, w, r)
			return
		}

		user, err := db.UserGetByID(event.UserID)
		if err != nil {
			log.Printf("GuestOrderPage: user of event %d: %v", eventID, err)
		}

		ss, rr, err := SplitSitsRooms(sits, rooms)
		if err != nil || len(ss) == 0 {
			log.Printf("GuestOrderPage: sits %q rooms %q: %v", sits, rooms, err)
			ErrorHTML(T(lng, "guest.no_seats_title"), T(lng, "guest.nothing_selected"), lng, w, r)
			return
		}

		// miejsca muszą być w salach tej imprezy i wolne
		chairs := eventChairs(db, event)
		var picked []FurnitureFull
		seen := map[seatKey]bool{}
		for i := range ss {
			k := seatKey{rr[i], ss[i]}
			c, ok := chairs[k]
			if !ok || seen[k] || chairStatus(c) != "free" {
				log.Printf("GuestOrderPage: seat %d in room %d not available for event %d (known %v)", ss[i], rr[i], eventID, ok)
				ErrorHTML(T(lng, "order.seats_taken"), T(lng, "order.seats_taken_text"), lng, w, r)
				return
			}
			seen[k] = true
			picked = append(picked, c)
		}

		// blokada miejsc: status "marked" do czasu potwierdzenia zamówienia
		var labels, prices []string
		var resIDs []int64
		var total int64
		for i, c := range picked {
			price, currency := chairPrice(c, event)
			id, err := db.ReservationAdd(&Reservation{
				OrderedDate: ToNI(time.Now().Unix()),
				Price:       ToNI(price),
				Currency:    ToNS(currency),
				Status:      "marked",
				FurnitureID: c.ID,
				EventID:     eventID,
				CustomerID:  -1, // uzupełniane przy tworzeniu klienta
			})
			if err != nil {
				log.Printf("GuestOrderPage: seat %d already taken (event %d): %v", ss[i], eventID, err)
				// zdejmujemy blokady założone przed chwilą, żeby nie wisiały 5 minut
				for _, id := range resIDs {
					if err := db.ReservationDel(id); err != nil {
						log.Printf("GuestOrderPage: release reservation %d: %v", id, err)
					}
				}
				ErrorHTML(T(lng, "order.seats_taken"), T(lng, "order.seats_taken_text"), lng, w, r)
				return
			}
			resIDs = append(resIDs, id)
			total += price
			prices = append(prices, strconv.FormatInt(price, 10))
			labels = append(labels, strconv.FormatInt(ss[i], 10))
		}
		totalPrice := strconv.FormatInt(total, 10)

		vars := nOrderVars{
			Title:           event.Name,
			EventID:         event.ID,
			EventName:       event.Name,
			UserURL:         user.URL,
			Minutes:         MARKEDMINUTES,
			Sits:            sits,
			SitsLabel:       strings.Join(labels, ", "),
			Rooms:           rooms,
			Prices:          strings.Join(prices, ","),
			TotalPrice:      totalPrice,
			TotalPriceLabel: FormatMoney(total, event.DefaultCurrency),
			Lock:            orderLock(event.ID, resIDs),
			HideCosts:       event.HideCosts.Bool,
			HowTo:           template.HTML(event.OrderHowto),
			NotesHelp:       event.OrderNotesDescription,
			Addons:          orderAddonFields(EventAddonFields(db, event), int64(len(ss))),
			Currency:        event.DefaultCurrency,
		}

		guestRenderTheme(w, lng, event.Theme.String, vars, "tmpl/n_order.html")
	}
}

type ReservationOrderStatusVars struct {
	LBLLang       string
	LBLTitle      string
	LBLStatus     string
	LBLStatusText template.HTML
	BTNOk         string
	Theme         string // atrybut data-theme (themes.go)
}

// Order - dane zamówienia z formularza gościa (/order/status)
type Order struct {
	EventID             int64
	TotalPrice          string
	Sits, Prices, Rooms string
	Email               string
	Name, Surname       string
	Phone, Notes        string
}

// ReservationOrderStatusHTML - POST /order/status: zablokowane miejsca
// stają się zamówieniem gościa, idą maile i strona z podziękowaniem.
func ReservationOrderStatusHTML(db *DB, mailConf MailConfig) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if err := r.ParseForm(); err != nil {
			log.Printf("ReservationOrderStatusHTML: parse form: %v", err)
		}
		o := Order{
			Sits:       r.FormValue("sits"),
			Prices:     r.FormValue("prices"),
			Rooms:      r.FormValue("rooms"),
			TotalPrice: r.FormValue("total-price"),
			Email:      r.FormValue("email"),
			Name:       r.FormValue("name"),
			Surname:    r.FormValue("surname"),
			Phone:      r.FormValue("phone"),
			Notes:      r.FormValue("notes"),
		}
		var err error
		if o.EventID, err = strconv.ParseInt(r.FormValue("event-id"), 10, 64); err != nil {
			log.Printf("ReservationOrderStatusHTML: bad event id %q: %v", r.FormValue("event-id"), err)
		}
		event, err := db.EventGetByID(o.EventID)
		if err != nil {
			log.Printf("ReservationOrderStatusHTML: event %d: %v", o.EventID, err)
		}
		lng := KnownLang(event.Language.String)
		fail := func(key string) {
			ErrorHTML(T(lng, key+"_title"), T(lng, key), lng, w, r)
		}
		if err != nil {
			fail("order.err_expired")
			return
		}

		// tylko blokady założone przez tego gościa w kroku /order (ticker.go
		// kasuje je po 5 minutach)
		lock, ok := parseOrderLock(o.EventID, r.FormValue("lock"))
		if !ok {
			log.Printf("ReservationOrderStatusHTML: bad lock %q for event %d", r.FormValue("lock"), o.EventID)
			fail("order.err_expired")
			return
		}
		ss, rr, err := SplitSitsRooms(o.Sits, o.Rooms)
		if err != nil {
			log.Printf("ReservationOrderStatusHTML: %v", err)
			fail("order.err_expired")
			return
		}
		var marked []Reservation
		var seatsTotal int64
		var prices []string
		for i := range ss {
			chair, err := db.FurnitureGetByTypeNumberRoom("chair", ss[i], rr[i])
			if err != nil {
				log.Printf("ReservationOrderStatusHTML: chair %d in room %d: %v", ss[i], rr[i], err)
			}
			res, err := db.ReservationGet(chair.ID, o.EventID)
			if err != nil || res.Status != "marked" || !lock[res.ID] {
				log.Printf("ReservationOrderStatusHTML: no marked reservation of this guest for chair %d, event %d (status %q): %v", chair.ID, o.EventID, res.Status, err)
				fail("order.err_expired")
				return
			}
			marked = append(marked, res)
			seatsTotal += res.Price.Int64
			prices = append(prices, strconv.FormatInt(res.Price.Int64, 10))
		}
		o.Prices = strings.Join(prices, ",")

		cID, err := db.CustomerAdd(&Customer{
			Email:   o.Email,
			Name:    ToNS(o.Name),
			Surname: ToNS(o.Surname),
			Phone:   ToNS(o.Phone),
		})
		if err != nil {
			log.Printf("ReservationOrderStatusHTML: add customer %q: %v", o.Email, err)
			fail("order.err_save")
			return
		}
		user, err := db.UserGetByID(event.UserID)
		if err != nil {
			log.Printf("ReservationOrderStatusHTML: user %d of event %d: %v", event.UserID, event.ID, err)
		}
		if err := db.CustomerAppendToUser(user.ID, cID); err != nil {
			log.Printf("ReservationOrderStatusHTML: append customer to user (returning customer?): %v", err)
		}

		var noteID int64
		if o.Notes != "" {
			if noteID, err = db.NoteAdd(o.Notes); err != nil {
				log.Printf("ReservationOrderStatusHTML: add note: %v", err)
			}
		}

		// zebrane po drodze, żeby po pętli spiąć je w jedno zamówienie
		var reservationIDs []int64
		for _, res := range marked {
			res.Status = "ordered"
			res.CustomerID = cID
			if noteID != 0 {
				res.NoteID = ToNI(noteID)
			}
			if err := db.ReservationMod(&res); err != nil {
				log.Printf("ReservationOrderStatusHTML: reservation %d, event %d: %v", res.ID, o.EventID, err)
				fail("order.err_status")
				return
			}
			reservationIDs = append(reservationIDs, res.ID)
		}

		// Zamówienie jako byt: spina rezerwacje z klientem, a przy okazji
		// zapisuje dodatki (odpowiedzi i dopłaty). Suma = ceny miejsc z bazy
		// + dopłaty policzone na serwerze; przy ukrytych cenach formularz
		// sumy nie wysyła i zostaje pusta, jak dotąd.
		_, addonsTotal := SaveOrderWithAddons(db, event, user, cID, reservationIDs, o.Notes, r.Form)
		if o.TotalPrice != "" {
			o.TotalPrice = strconv.FormatInt(seatsTotal+addonsTotal, 10)
		}

		sendOrderMails(db, mailConf, event, user, o)

		// tekst po zamówieniu też jest szablonem (np. z kodem QR)
		statusTitle, statusText := orderedNoteOrDefault(db, event, user, o, lng)
		p := ReservationOrderStatusVars{
			LBLLang:       lng,
			LBLTitle:      T(lng, "order.page_title"),
			LBLStatus:     statusTitle,
			LBLStatusText: template.HTML(renderOrderTmpl(statusText, orderFuncsFromOrder(o, db, user))),
			BTNOk:         "OK",
			Theme:         themeAttr(event.Theme.String),
		}
		t, err := template.ParseFiles("tmpl/n_order_status.html")
		if err != nil {
			log.Printf("ReservationOrderStatusHTML: template parse: %v", err)
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}
		if err := t.Execute(w, p); err != nil {
			log.Printf("ReservationOrderStatusHTML: template exec: %v", err)
		}
	}
}

// mailHTMLHead - początek maili z zamówieniem; "<html" oznacza mail HTML,
// a styl usuwa odstępy akapitów z edytora
const mailHTMLHead = "<html><head><style>p {margin:0;}</style></head>"

// sendOrderMails wysyła podziękowanie gościowi i powiadomienie organizatorowi
// (o ile impreza ma ustawione te maile).
func sendOrderMails(db *DB, conf MailConfig, event Event, user User, o Order) {
	if id := event.ThankYouNotificationsID.Int64; id > 0 {
		if n, err := db.NotificationGetByID(id, user.ID); err != nil {
			log.Printf("sendOrderMails: guest mail %d: %v", id, err)
		} else {
			of := orderFuncsFromOrder(o, db, user)
			text := renderOrderTmpl(n.Text, of)
			if err := conf.SendAsOrg(user, o.Email, OrderSubject(n.Title.String, of), mailHTMLHead+text+"</html>",
				append(n.Files(user), of.AttachedFiles...), append(n.Images(user), of.EmbeddedImgs...)); err != nil {
				log.Printf("sendOrderMails: guest mail: %v", err)
			}
		}
	}

	if id := event.AdminNotificationsID.Int64; id > 0 {
		n, err := db.NotificationGetByID(id, user.ID)
		if err != nil {
			log.Printf("sendOrderMails: admin mail %d: %v", id, err)
			return
		}
		admins, err := db.AdminGetEmails(user.ID)
		if err != nil {
			log.Printf("sendOrderMails: admin addresses: %v", err)
		}
		of := orderFuncsFromOrder(o, db, user)
		text := renderOrderTmpl(n.Text, of)
		if err := conf.Send(Mail{
			From:    conf.From,
			To:      append(admins, user.Email),
			Subject: OrderSubject(n.Title.String, of),
			Text:    mailHTMLHead + text + "</html>",
		}); err != nil {
			log.Printf("sendOrderMails: admin mail: %v", err)
		}
	}
}

// renderOrderTmpl wypełnia szablon (treść maila, tekst po zamówieniu) danymi
// zamówienia; of zbiera przy tym obrazki QR i bilety PDF. Zepsuty szablon
// zostaje pokazany tak, jak jest.
func renderOrderTmpl(t string, of *OrderFuncs) string {
	tmpl, err := template.New("ordermail").Parse(t)
	if err != nil {
		log.Printf("renderOrderTmpl: parse %q: %v", t, err)
		return t
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, of); err != nil {
		log.Printf("renderOrderTmpl: exec %q: %v", t, err)
		of.EmbeddedImgs, of.AttachedFiles = nil, nil
		return t
	}
	return buf.String()
}

type OrderCancelMsg struct {
	Sits    string `json:"sits"`
	Rooms   string `json:"rooms"`
	EventID int64  `json:"event-id"`
	Lock    string `json:"lock"`
}

// OrderCancel - POST /api/ordercancel: gość rezygnuje w kroku /order,
// zdejmujemy jego blokady miejsc (tylko "marked" i tylko z jego orderLock)
func OrderCancel(db *DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			return
		}
		var m OrderCancelMsg
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			log.Printf("OrderCancel: %v", err)
			return
		}
		lock, ok := parseOrderLock(m.EventID, m.Lock)
		if !ok {
			log.Printf("OrderCancel: bad lock %q for event %d", m.Lock, m.EventID)
			return
		}
		ss, rr, err := SplitSitsRooms(m.Sits, m.Rooms)
		if err != nil {
			log.Printf("OrderCancel: %v", err)
			return
		}
		for i := range ss {
			chair, err := db.FurnitureGetByTypeNumberRoom("chair", ss[i], rr[i])
			if err != nil {
				log.Printf("OrderCancel: chair %d in room %d: %v", ss[i], rr[i], err)
				continue
			}
			res, err := db.ReservationGet(chair.ID, m.EventID)
			if err != nil || res.Status != "marked" || !lock[res.ID] {
				continue
			}
			if err := db.ReservationDel(res.ID); err != nil {
				log.Printf("OrderCancel: delete reservation %d: %v", res.ID, err)
			}
		}
	}
}

// orderedNoteOrDefault - tytuł i treść strony po zamówieniu. Nowa impreza
// ma je puste, a wtedy gość widziałby sam przycisk OK - podstawiamy więc
// podziękowanie i, gdy jest konto i coś do zapłaty, dane do przelewu z QR.
func orderedNoteOrDefault(db *DB, event Event, user User, o Order, lng string) (string, string) {
	title, text := event.OrderedNoteTitle, event.OrderedNoteText
	if strings.TrimSpace(title) == "" {
		title = T(lng, "order.done_title")
	}
	if strings.TrimSpace(removeTrailingEmptyParagraphs(text)) != "" {
		return title, text
	}
	text = ""
	if event.ThankYouNotificationsID.Valid && event.ThankYouNotificationsID.Int64 > 0 {
		text = "<p>" + template.HTMLEscapeString(T(lng, "order.done_text")) + "</p>"
	}
	total, _ := strconv.ParseInt(strings.TrimSpace(o.TotalPrice), 10, 64)
	if event.BankAccountsID.Valid && total > 0 {
		if ba, err := db.BankAccountGetByID(event.BankAccountsID.Int64, user.ID); err == nil && ba.IBAN != "" {
			name := strconv.Quote(ba.Name)
			text += "<p>" + template.HTMLEscapeString(T(lng, "order.done_pay")) + "</p>" +
				"{{.PayInfo " + name + "}}<p>{{.QRPay " + name + "}}</p>"
		}
	}
	return title, text
}

// parseIDs - "1, 2,3" -> [1 2 3]
func parseIDs(s string) ([]int64, error) {
	var out []int64
	for _, f := range strings.Split(s, ",") {
		n, err := strconv.ParseInt(strings.TrimSpace(f), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("can not convert %q to number: %v", f, err)
		}
		out = append(out, n)
	}
	return out, nil
}

// SplitSitsRooms - numery krzeseł i ID sal z formularza zamówienia
func SplitSitsRooms(sits, rooms string) ([]int64, []int64, error) {
	ss, err := parseIDs(sits)
	if err != nil {
		return nil, nil, fmt.Errorf("sits: %v", err)
	}
	rr, err := parseIDs(rooms)
	if err != nil {
		return nil, nil, fmt.Errorf("rooms: %v", err)
	}
	if len(ss) != len(rr) {
		return nil, nil, fmt.Errorf("sits %q and rooms %q differ in length", sits, rooms)
	}
	return ss, rr, nil
}

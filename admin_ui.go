package main

// admin_ui.go: nowy panel administracyjny (redesign "Harmonijka").
//
// Strony: /admin/start (lista imprez i formularzy), /admin/events/{id} (impreza
// jako sekcje harmonijki). Zapis sekcji: POST /api/events/{id}/section.
// Stary panel (/admin, /admin/event, ...) działa bez zmian do czasu przełączenia.

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// NPage to wspólne zmienne nowego layoutu (tmpl/n_base.html).
type NPage struct {
	Title   string
	Crumb   string
	OrgName string
}

// --- pomoce ----------------------------------------------------------------

// adminUser sprawdza sesję i zwraca organizację zalogowanego admina.
func adminUser(w http.ResponseWriter, r *http.Request, db *DB, cs *sessions.CookieStore) (User, bool) {
	_, _, email, err := InitSession(w, r, cs, "/admin/login", true)
	if err != nil {
		log.Printf("admin_ui: session: %v", err)
		return User{}, false
	}
	user, err := db.UserGetByEmail(email)
	if err != nil {
		log.Printf("admin_ui: can not get user %q: %v", email, err)
		http.Error(w, "unknown user", http.StatusForbidden)
		return User{}, false
	}
	return user, true
}

// nTemplate parsuje szablon strony razem z nowym layoutem i funkcjami języka.
func nTemplate(lang string, files ...string) (*template.Template, error) {
	all := append([]string{}, files...)
	all = append(all, "tmpl/n_base.html")
	name := strings.TrimPrefix(files[0], "tmpl/")
	return template.New(name).Funcs(TmplFuncs(lang)).ParseFiles(all...)
}

func nRender(w http.ResponseWriter, lang string, vars interface{}, files ...string) {
	t, err := nTemplate(lang, files...)
	if err != nil {
		log.Printf("admin_ui: template parse %v: %v", files, err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	if err := t.ExecuteTemplate(w, "base", vars); err != nil {
		log.Printf("admin_ui: template exec %v: %v", files, err)
	}
}

// FormatDate formatuje datę wg języka: pl 21.11.2026, cs 21. 11. 2026, en 21 Nov 2026.
func FormatDate(lang string, unix int64) string {
	if unix <= 0 {
		return ""
	}
	t := time.Unix(unix, 0)
	switch lang {
	case "cs":
		return fmt.Sprintf("%d. %d. %d", t.Day(), int(t.Month()), t.Year())
	case "en":
		return t.Format("2 Jan 2006")
	default:
		return fmt.Sprintf("%d.%d.%d", t.Day(), int(t.Month()), t.Year())
	}
}

// FormatDateShort - dzień i miesiąc, do podsumowań ("1.10.").
func FormatDateShort(lang string, unix int64) string {
	if unix <= 0 {
		return ""
	}
	t := time.Unix(unix, 0)
	if lang == "en" {
		return t.Format("2 Jan")
	}
	return fmt.Sprintf("%d.%d.", t.Day(), int(t.Month()))
}

func dateInputValue(unix int64) string {
	if unix <= 0 {
		return ""
	}
	return time.Unix(unix, 0).Format("2006-01-02")
}

// --- strona startowa -------------------------------------------------------

type nHomeRow struct {
	ID   int64
	Name string
	// DateS - data imprezy w języku panelu, Info - prawa kolumna wiersza.
	DateS string
	Info  string
}

type nHomeVars struct {
	NPage
	Events []nHomeRow
	Forms  []nHomeRow
}

// AdminStartPage - lista imprez i formularzy (nowy panel).
func AdminStartPage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		lang := PanelLang(r)

		events, err := db.EventGetAllByUserID(user.ID)
		if err != nil {
			log.Printf("AdminStartPage: events: %v", err)
		}
		// najbliższe imprezy na górze, bez daty na końcu
		sort.Slice(events, func(i, j int) bool {
			if events[i].Date == events[j].Date {
				return events[i].Name < events[j].Name
			}
			if events[i].Date <= 0 {
				return false
			}
			if events[j].Date <= 0 {
				return true
			}
			return events[i].Date > events[j].Date
		})

		vars := nHomeVars{
			NPage: NPage{Title: T(lang, "nav.events"), OrgName: orgName(user)},
		}

		for _, e := range events {
			seats, payed, ordered := eventSeatStats(db, user.ID, e.ID)
			info := ""
			if seats > 0 {
				info = T(lang, "home.seats_of", payed+ordered, seats)
			} else if e.FromDate > 0 {
				info = T(lang, "home.sale_from", FormatDateShort(lang, e.FromDate))
			}
			vars.Events = append(vars.Events, nHomeRow{
				ID:    e.ID,
				Name:  e.Name,
				DateS: FormatDate(lang, e.Date),
				Info:  info,
			})
		}

		templs, err := db.FormTemplateGetAll(user.ID)
		if err != nil {
			log.Printf("AdminStartPage: forms: %v", err)
		}
		for _, ft := range templs {
			n, err := db.FormGetAmmount(user.ID, ft.ID)
			if err != nil {
				log.Printf("AdminStartPage: form answers count: %v", err)
			}
			vars.Forms = append(vars.Forms, nHomeRow{
				ID:   ft.ID,
				Name: ft.Name,
				Info: TN(lang, "count.answer", n),
			})
		}

		nRender(w, lang, vars, "tmpl/n_home.html")
	}
}

func orgName(u User) string {
	if u.Organization.Valid && u.Organization.String != "" {
		return u.Organization.String
	}
	return u.Email
}

// eventSeatStats zwraca liczbę miejsc w salach imprezy oraz miejsca zapłacone i zamówione.
func eventSeatStats(db *DB, userID, eventID int64) (seats, payed, ordered int64) {
	rooms, err := db.EventGetRooms(eventID)
	if err != nil {
		log.Printf("eventSeatStats: rooms: %v", err)
	}
	for _, room := range rooms {
		chairs, err := db.FurnitureGetAllByRoomIDOfType(room.ID, "chair")
		if err != nil {
			log.Printf("eventSeatStats: chairs: %v", err)
			continue
		}
		seats += int64(len(chairs))
	}
	rr, err := db.ReservationFullGetAll(userID, eventID)
	if err != nil {
		log.Printf("eventSeatStats: reservations: %v", err)
		return
	}
	for _, r := range rr {
		switch r.Status {
		case "payed":
			payed++
		case "ordered":
			ordered++
		}
	}
	return
}

// --- strona imprezy --------------------------------------------------------

type nRoomText struct {
	DescLabel, DescField string
	Desc                 template.HTML
	BannerLabel          string
	BannerField, Banner  string
}

type nResRow struct {
	Person      string
	Seats       string
	StatusLabel string
	StatusClass string
	Amount      string
}

type nEventVars struct {
	NPage
	ID       int64
	Name     string
	Language string
	SubTitle string
	GuestURL string

	DateVal, FromVal, ToVal string
	Price                   int64
	Currency                string

	RoomsVal   string
	RoomsNames string
	UserRooms  []Room
	EventRooms []Room

	MainDesc       template.HTML
	MainBanner     string
	RoomTexts      []nRoomText
	OrderHowto     template.HTML
	OrderNotesDesc string
	NoSitsTitle    string
	NoSitsText     template.HTML
	OrderedTitle   string
	OrderedText    template.HTML
	HowTo          template.HTML

	Notifications  []Notification
	ThankYouMailID int64
	AdminMailID    int64
	BankAccounts   []BankAccount
	BankAccountID  int64

	Forms []FormTemplate

	Sharable  bool
	HideCosts bool

	SumBasic, SumDates, SumRoom, SumGuest string
	SumMailGuest, SumMailAdmin, SumBank   string
	SumForms, SumRes, SumSettings         string
	MissMailGuest, MissMailAdmin          bool
	MissBank                              bool

	SeatsPayed, SeatsOrdered, SeatsFree int64
	PctPayed, PctOrdered                int64
	ResRows                             []nResRow
}

// EventPage - strona jednej imprezy: /admin/events/{id}
func EventPage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		lang := PanelLang(r)

		id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		event, err := db.EventGetByID(id)
		if err != nil || event.UserID != user.ID {
			log.Printf("EventPage: event %d not found for user %d: %v", id, user.ID, err)
			http.NotFound(w, r)
			return
		}

		vars := buildEventVars(db, user, event, lang)
		nRender(w, lang, vars, "tmpl/n_event.html")
	}
}

func buildEventVars(db *DB, user User, event Event, lang string) nEventVars {
	evRooms, err := db.EventGetRooms(event.ID)
	if err != nil {
		log.Printf("buildEventVars: event rooms: %v", err)
	}
	userRooms, err := db.RoomGetAllByUserID(user.ID)
	if err != nil {
		log.Printf("buildEventVars: user rooms: %v", err)
	}
	notifs, err := db.NotificationGetAllRelatedToEventsForUser(user.ID)
	if err != nil {
		log.Printf("buildEventVars: notifications: %v", err)
	}
	bankAccs, err := db.BankAccountGetAll(user.ID)
	if err != nil {
		log.Printf("buildEventVars: bank accounts: %v", err)
	}
	templs, err := db.FormTemplateGetAll(user.ID)
	if err != nil {
		log.Printf("buildEventVars: form templates: %v", err)
	}
	var eventForms []FormTemplate
	for _, ft := range templs {
		if ft.EventID.Valid && ft.EventID.Int64 == event.ID {
			eventForms = append(eventForms, ft)
		}
	}

	language := event.Language.String
	if language == "" {
		language = "pl"
	}

	v := nEventVars{
		NPage: NPage{
			Title:   event.Name,
			Crumb:   event.Name,
			OrgName: orgName(user),
		},
		ID:       event.ID,
		Name:     event.Name,
		Language: language,
		// strona gościa sama wybiera aktualną imprezę organizacji (EventGetCurrent)
		GuestURL:       "/res/" + user.URL,
		DateVal:        dateInputValue(event.Date),
		FromVal:        dateInputValue(event.FromDate),
		ToVal:          dateInputValue(event.ToDate),
		Price:          event.DefaultPrice,
		Currency:       event.DefaultCurrency,
		RoomsVal:       getRoomsString(evRooms),
		UserRooms:      userRooms,
		EventRooms:     evRooms,
		MainDesc:       template.HTML(event.MainDesc.String),
		MainBanner:     event.MainBanner.String,
		OrderHowto:     template.HTML(event.OrderHowto),
		OrderNotesDesc: event.OrderNotesDescription,
		NoSitsTitle:    event.NoSitsSelectedTitle,
		NoSitsText:     template.HTML(event.NoSitsSelectedText),
		OrderedTitle:   event.OrderedNoteTitle,
		OrderedText:    template.HTML(event.OrderedNoteText),
		HowTo:          template.HTML(event.HowTo),
		Notifications:  notifs,
		ThankYouMailID: event.ThankYouNotificationsID.Int64,
		AdminMailID:    event.AdminNotificationsID.Int64,
		BankAccounts:   bankAccs,
		BankAccountID:  event.BankAccountsID.Int64,
		Forms:          eventForms,
		Sharable:       event.Sharable.Bool,
		HideCosts:      event.HideCosts.Bool,
	}

	// opisy i bannery sal - tyle pól, ile sal ma impreza (minimum jedno)
	descs := []string{event.Room1Desc.String, event.Room2Desc.String, event.Room3Desc.String, event.Room4Desc.String}
	banners := []string{event.Room1Banner.String, event.Room2Banner.String, event.Room3Banner.String, event.Room4Banner.String}
	roomFields := len(evRooms)
	if roomFields > 4 {
		roomFields = 4
	}
	for i := 0; i < roomFields; i++ {
		name := ""
		if i < len(evRooms) {
			name = " (" + evRooms[i].Name + ")"
		}
		v.RoomTexts = append(v.RoomTexts, nRoomText{
			DescLabel:   T(lang, "field.room_desc", i+1) + name,
			DescField:   fmt.Sprintf("room%ddesc", i+1),
			Desc:        template.HTML(descs[i]),
			BannerLabel: T(lang, "field.room_banner", i+1) + name,
			BannerField: fmt.Sprintf("room%dbanner", i+1),
			Banner:      banners[i],
		})
	}

	var roomNames []string
	for _, rm := range evRooms {
		roomNames = append(roomNames, rm.Name)
	}
	v.RoomsNames = strings.Join(roomNames, ", ")

	fillEventSummaries(db, user, event, lang, &v)
	return v
}

// fillEventSummaries liczy jednolinijkowe podsumowania sekcji i dane rezerwacji.
func fillEventSummaries(db *DB, user User, event Event, lang string, v *nEventVars) {
	v.SubTitle = eventSubtitle(lang, event)

	v.SumBasic = event.Name + ", " + languageName(lang, v.Language)

	dates := FormatDate(lang, event.Date)
	if event.FromDate > 0 || event.ToDate > 0 {
		dates = strings.TrimSpace(dates + ", " + T(lang, "event.reservation_open",
			FormatDateShort(lang, event.FromDate), FormatDateShort(lang, event.ToDate)))
	}
	if dates == "" {
		dates = T(lang, "common.not_set")
	}
	v.SumDates = dates

	room := v.RoomsNames
	if room == "" {
		room = T(lang, "common.not_set")
	}
	if event.DefaultPrice > 0 {
		room += ", " + FormatMoney(event.DefaultPrice, event.DefaultCurrency)
	}
	v.SumRoom = room

	texts := 0
	for _, s := range []string{event.MainDesc.String, event.OrderHowto, event.NoSitsSelectedText, event.OrderedNoteText, event.HowTo} {
		if strings.TrimSpace(s) != "" {
			texts++
		}
	}
	guest := TN(lang, "count.text", int64(texts))
	if event.MainBanner.Valid && event.MainBanner.String != "" {
		guest = T(lang, "field.main_banner") + ", " + guest
	}
	v.SumGuest = guest

	v.SumMailGuest, v.MissMailGuest = notificationName(db, user, event.ThankYouNotificationsID.Int64, lang)
	v.SumMailAdmin, v.MissMailAdmin = notificationName(db, user, event.AdminNotificationsID.Int64, lang)

	if event.BankAccountsID.Valid && event.BankAccountsID.Int64 != 0 {
		if ba, err := db.BankAccountGetByID(event.BankAccountsID.Int64, user.ID); err == nil {
			v.SumBank = ba.Name + ", " + ba.IBAN
		} else {
			v.SumBank = T(lang, "common.not_set")
			v.MissBank = true
		}
	} else {
		v.SumBank = T(lang, "common.not_set")
		v.MissBank = true
	}

	if len(v.Forms) == 0 {
		v.SumForms = T(lang, "common.none")
	} else {
		var parts []string
		for _, ft := range v.Forms {
			n, _ := db.FormGetAmmount(user.ID, ft.ID)
			parts = append(parts, fmt.Sprintf("%s (%s)", ft.Name, TN(lang, "count.answer", n)))
		}
		v.SumForms = strings.Join(parts, ", ")
	}

	settings := []string{}
	if event.Sharable.Bool {
		settings = append(settings, T(lang, "field.sharable"))
	}
	if event.HideCosts.Bool {
		settings = append(settings, T(lang, "field.hidecosts"))
	}
	if len(settings) == 0 {
		settings = append(settings, T(lang, "common.none"))
	}
	v.SumSettings = strings.Join(settings, ", ")

	fillReservationSummary(db, user, event, lang, v)
}

func eventSubtitle(lang string, event Event) string {
	parts := []string{}
	if d := FormatDate(lang, event.Date); d != "" {
		parts = append(parts, d)
	}
	if event.FromDate > 0 || event.ToDate > 0 {
		parts = append(parts, T(lang, "event.reservation_open",
			FormatDateShort(lang, event.FromDate), FormatDateShort(lang, event.ToDate)))
	}
	if event.Date > 0 && time.Now().Unix() > event.Date {
		parts = append(parts, T(lang, "event.finished"))
	}
	return strings.Join(parts, ", ")
}

func languageName(panelLang, code string) string {
	switch code {
	case "cs":
		return "čeština"
	case "en":
		return "english"
	default:
		return "polski"
	}
}

func notificationName(db *DB, user User, id int64, lang string) (string, bool) {
	if id == 0 {
		return T(lang, "common.not_set"), true
	}
	n, err := db.NotificationGetByID(id, user.ID)
	if err != nil {
		return T(lang, "common.not_set"), true
	}
	return n.Name, false
}

// fillReservationSummary grupuje rezerwacje po kliencie (jeden wiersz = zamówienie).
func fillReservationSummary(db *DB, user User, event Event, lang string, v *nEventVars) {
	rr, err := db.ReservationFullGetAll(user.ID, event.ID)
	if err != nil {
		log.Printf("fillReservationSummary: %v", err)
	}

	type group struct {
		person string
		seats  []string
		amount int64
		curr   string
		payed  bool
		any    bool
	}
	order := []string{}
	groups := map[string]*group{}

	var waiting int64
	for _, r := range rr {
		switch r.Status {
		case "payed":
			v.SeatsPayed++
		case "ordered":
			v.SeatsOrdered++
		default:
			continue // marked, disabled, free - nie pokazujemy w podsumowaniu
		}

		key := r.CustEmail.String
		if key == "" {
			key = fmt.Sprintf("res-%d", r.ID)
		}
		g, ok := groups[key]
		if !ok {
			g = &group{
				person: strings.TrimSpace(r.CustName.String + " " + r.CustSurname.String),
				payed:  true,
			}
			if g.person == "" {
				g.person = r.CustEmail.String
			}
			groups[key] = g
			order = append(order, key)
		}
		g.any = true
		g.seats = append(g.seats, fmt.Sprintf("%s: %d", r.RoomName, r.ChairNumber))
		g.amount += r.Price.Int64
		if r.Currency.Valid && r.Currency.String != "" {
			g.curr = r.Currency.String
		}
		if r.Status != "payed" {
			g.payed = false
			waiting += r.Price.Int64
		}
	}

	seats, _, _ := eventSeatStats(db, user.ID, event.ID)
	v.SeatsFree = seats - v.SeatsPayed - v.SeatsOrdered
	if v.SeatsFree < 0 {
		v.SeatsFree = 0
	}
	if seats > 0 {
		v.PctPayed = v.SeatsPayed * 100 / seats
		v.PctOrdered = v.SeatsOrdered * 100 / seats
	}

	currency := event.DefaultCurrency
	for _, key := range order {
		g := groups[key]
		if g.curr != "" {
			currency = g.curr
		}
		label, class := statusLabel(lang, g.payed)
		v.ResRows = append(v.ResRows, nResRow{
			Person:      g.person,
			Seats:       strings.Join(g.seats, ", "),
			StatusLabel: label,
			StatusClass: class,
			Amount:      FormatMoney(g.amount, currency),
		})
		if len(v.ResRows) >= 10 {
			break
		}
	}

	sum := TN(lang, "count.reservation", int64(len(order))) + ", " +
		TN(lang, "count.seat", v.SeatsPayed+v.SeatsOrdered)
	if waiting > 0 {
		sum += ", " + FormatMoney(waiting, event.DefaultCurrency) + " " + T(lang, "res.awaiting")
	}
	v.SumRes = sum
}

func statusLabel(lang string, payed bool) (string, string) {
	if payed {
		return T(lang, "status.payed"), "paid"
	}
	return T(lang, "status.ordered"), "ord"
}

// --- API: zapis sekcji -----------------------------------------------------

type nSectionMsg struct {
	Section string                 `json:"section"`
	Values  map[string]interface{} `json:"values"`
}

type nSectionResp struct {
	Msg      string `json:"msg"`
	Summary  string `json:"summary,omitempty"`
	Missing  bool   `json:"missing,omitempty"`
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
}

// EventSectionSave zapisuje jedną sekcję strony imprezy: POST /api/events/{id}/section
func EventSectionSave(db *DB, loc *time.Location, dF string, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonMsg(w, http.StatusMethodNotAllowed, nSectionResp{Msg: "POST only"})
			return
		}
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		lang := PanelLang(r)

		id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
		if err != nil {
			jsonMsg(w, http.StatusBadRequest, nSectionResp{Msg: "bad event id"})
			return
		}
		event, err := db.EventGetByID(id)
		if err != nil || event.UserID != user.ID {
			jsonMsg(w, http.StatusNotFound, nSectionResp{Msg: "event not found"})
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			jsonMsg(w, http.StatusBadRequest, nSectionResp{Msg: err.Error()})
			return
		}
		var msg nSectionMsg
		if err := json.Unmarshal(body, &msg); err != nil {
			jsonMsg(w, http.StatusBadRequest, nSectionResp{Msg: err.Error()})
			return
		}

		val := func(key string) string {
			if s, ok := msg.Values[key].(string); ok {
				return strings.TrimSpace(s)
			}
			return ""
		}
		html := func(key string) string {
			if s, ok := msg.Values[key].(string); ok {
				return removeTrailingEmptyParagraphs(s)
			}
			return ""
		}
		flag := func(key string) bool {
			b, _ := msg.Values[key].(bool)
			return b
		}
		date := func(key string) int64 {
			s := val(key)
			if s == "" {
				return 0
			}
			u, err := ToUnix(s, loc, dF)
			if err != nil {
				log.Printf("EventSectionSave: date %q: %v", s, err)
				return 0
			}
			return u
		}
		id64 := func(key string) int64 {
			n, _ := strconv.ParseInt(val(key), 10, 64)
			return n
		}

		switch msg.Section {
		case "basic":
			if name := val("name"); name != "" {
				event.Name = name
			}
			event.Language = ToNS(val("language"))
		case "dates":
			event.Date = date("date")
			event.FromDate = date("from_date")
			event.ToDate = date("to_date")
		case "room":
			event.DefaultPrice = id64("price")
			event.DefaultCurrency = val("currency")
			if err := setEventRooms(db, event.ID, val("rooms")); err != nil {
				jsonMsg(w, http.StatusTeapot, nSectionResp{Msg: err.Error()})
				return
			}
		case "guest":
			event.MainDesc = ToNS(html("maindesc"))
			event.MainBanner = ToNS(val("mainbanner"))
			event.Room1Desc = ToNS(html("room1desc"))
			event.Room1Banner = ToNS(val("room1banner"))
			event.Room2Desc = ToNS(html("room2desc"))
			event.Room2Banner = ToNS(val("room2banner"))
			event.Room3Desc = ToNS(html("room3desc"))
			event.Room3Banner = ToNS(val("room3banner"))
			event.Room4Desc = ToNS(html("room4desc"))
			event.Room4Banner = ToNS(val("room4banner"))
			event.OrderHowto = html("order_howto")
			event.OrderNotesDescription = val("order_notes_desc")
			event.NoSitsSelectedTitle = val("no_sits_selected_title")
			event.NoSitsSelectedText = html("no_sits_selected_text")
			event.OrderedNoteTitle = val("ordered_note_title")
			event.OrderedNoteText = html("ordered_note_text")
			event.HowTo = html("how_to")
		case "mail_guest":
			event.ThankYouNotificationsID = ToNI(id64("thankyou_notifications_id_fk"))
		case "mail_admin":
			event.AdminNotificationsID = ToNI(id64("admin_notifications_id_fk"))
		case "bank":
			event.BankAccountsID = ToNI(id64("bank_account_id"))
		case "settings":
			event.Sharable = ToNB(flag("sharable"))
			event.HideCosts = ToNB(flag("hidecosts"))
			if err := db.EventSetHideCosts(event.ID, user.ID, flag("hidecosts")); err != nil {
				log.Printf("EventSectionSave: hidecosts: %v", err)
			}
		default:
			jsonMsg(w, http.StatusBadRequest, nSectionResp{Msg: "unknown section " + msg.Section})
			return
		}

		if err := db.EventModByID(&event, user.ID); err != nil {
			log.Printf("EventSectionSave: update event %d: %v", event.ID, err)
			jsonMsg(w, http.StatusTeapot, nSectionResp{Msg: T(lang, "common.save_error") + ": " + err.Error()})
			return
		}

		// świeże podsumowanie tej sekcji, żeby nagłówek zwiniętej sekcji był aktualny
		event, err = db.EventGetByID(event.ID)
		if err != nil {
			jsonMsg(w, http.StatusOK, nSectionResp{Msg: "saved"})
			return
		}
		v := buildEventVars(db, user, event, lang)
		resp := nSectionResp{Msg: "saved", Title: event.Name, Subtitle: v.SubTitle}
		switch msg.Section {
		case "basic":
			resp.Summary = v.SumBasic
		case "dates":
			resp.Summary = v.SumDates
		case "room":
			resp.Summary = v.SumRoom
		case "guest":
			resp.Summary = v.SumGuest
		case "mail_guest":
			resp.Summary, resp.Missing = v.SumMailGuest, v.MissMailGuest
		case "mail_admin":
			resp.Summary, resp.Missing = v.SumMailAdmin, v.MissMailAdmin
		case "bank":
			resp.Summary, resp.Missing = v.SumBank, v.MissBank
		case "settings":
			resp.Summary = v.SumSettings
		}
		jsonMsg(w, http.StatusOK, resp)
	}
}

// setEventRooms ustawia listę sal imprezy na dokładnie te z listy "1,2,3".
func setEventRooms(db *DB, eventID int64, csv string) error {
	if err := db.EventClearRooms(eventID); err != nil {
		return err
	}
	for _, s := range strings.Split(csv, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		roomID, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("room id %q: %w", s, err)
		}
		if err := db.RoomAddToEventUnsafe(roomID, eventID); err != nil {
			return err
		}
	}
	return nil
}

// --- API: nowa impreza (szkic) ---------------------------------------------

type nNewEventMsg struct {
	Name string `json:"name"`
}

// EventCreateAPI tworzy szkic imprezy z samą nazwą: POST /api/events
func EventCreateAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonMsg(w, http.StatusMethodNotAllowed, nSectionResp{Msg: "POST only"})
			return
		}
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}

		body, _ := io.ReadAll(r.Body)
		var msg nNewEventMsg
		if err := json.Unmarshal(body, &msg); err != nil {
			jsonMsg(w, http.StatusBadRequest, nSectionResp{Msg: err.Error()})
			return
		}
		name := strings.TrimSpace(msg.Name)
		if name == "" {
			jsonMsg(w, http.StatusBadRequest, nSectionResp{Msg: "empty name"})
			return
		}

		exists, err := db.EventNameExists(name, user.ID)
		if err != nil {
			log.Printf("EventCreateAPI: name check: %v", err)
		}
		if exists {
			jsonMsg(w, http.StatusTeapot, nSectionResp{Msg: "name already used"})
			return
		}

		ev := &Event{
			Name:            name,
			UserID:          user.ID,
			Language:        ToNS("pl"),
			DefaultCurrency: "Kč",
		}
		id, err := db.EventAdd(ev)
		if err != nil {
			log.Printf("EventCreateAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, nSectionResp{Msg: err.Error()})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "created", "id": id})
	}
}

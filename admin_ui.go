package main

// admin_ui.go: nowy panel administracyjny (redesign "Harmonijka").
//
// Strony: /admin (lista imprez i formularzy), /admin/events/{id} (impreza
// jako sekcje harmonijki). Zapis sekcji: POST /api/events/{id}/section.

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
	// UpURL, UpLabel - strona nadrzędna: link "← ..." nad tytułem (szablon
	// "up" w n_base.html) i ogniwo ścieżki w górnym pasku
	UpURL, UpLabel string
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
	// Missing - czego brakuje, żeby impreza działała ("brak ceny, brak konta").
	Missing string
	// HasSeats - czy impreza ma sale z krzesłami (wtedy pokazujemy pasek i kwoty).
	HasSeats   bool
	PctPayed   int64
	PctOrdered int64
	Money      string
	// Closed - formularz nie przyjmuje odpowiedzi
	Closed bool
}

type nHomeVars struct {
	NPage
	Selling  []nHomeRow
	Soon     []nHomeRow
	Finished []nHomeRow
	Forms    []nHomeRow
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

		now := time.Now().Unix()
		const day = int64(24 * 3600)

		for _, e := range events {
			st := eventStats(db, user.ID, e.ID)

			// braki ("brak konta", "brak ceny") mają sens tylko przed imprezą -
			// przy zakończonych zaśmiecałyby listę
			missing := ""
			if !(e.Date > 0 && e.Date < now) {
				missing = strings.Join(eventMissing(db, user, e, st, lang), ", ")
			}

			row := nHomeRow{
				ID:       e.ID,
				Name:     e.Name,
				DateS:    FormatDate(lang, e.Date),
				Missing:  missing,
				HasSeats: st.Seats > 0,
			}

			switch {
			case st.Seats > 0:
				row.Info = T(lang, "home.seats_of", st.Payed+st.Ordered, st.Seats)
				row.PctPayed = st.Payed * 100 / st.Seats
				row.PctOrdered = st.Ordered * 100 / st.Seats
				money := T(lang, "home.paid", FormatMoney(st.AmountPayed, e.DefaultCurrency))
				if st.AmountWaiting > 0 {
					money += " · " + T(lang, "home.waiting", FormatMoney(st.AmountWaiting, e.DefaultCurrency))
				}
				row.Money = money
			case e.FromDate > 0:
				row.Info = T(lang, "home.sale_from", FormatDateShort(lang, e.FromDate))
			}

			// przed imprezą pokazujemy, ile dni zostało
			if e.Date > now {
				left := T(lang, "home.days_left", TN(lang, "count.day", (e.Date-now)/day))
				if row.Info != "" {
					left += ", " + row.Info
				}
				row.Info = left
			}

			switch {
			case e.Date > 0 && e.Date < now:
				vars.Finished = append(vars.Finished, row)
			case e.FromDate > now: // sprzedaż jeszcze się nie zaczęła
				vars.Soon = append(vars.Soon, row)
			default:
				vars.Selling = append(vars.Selling, row)
			}
		}

		templs, err := db.FormTemplateGetAll(user.ID)
		if err != nil {
			log.Printf("AdminStartPage: forms: %v", err)
		}
		// najnowsze formularze na górze - to zwykle te bieżące
		sort.Slice(templs, func(i, j int) bool { return templs[i].ID > templs[j].ID })
		for _, ft := range templs {
			if isAddonsTemplate(ft) {
				continue // dodatki edytuje się w sekcji imprezy
			}
			n, err := db.FormGetAmmount(user.ID, ft.ID)
			if err != nil {
				log.Printf("AdminStartPage: form answers count: %v", err)
			}
			vars.Forms = append(vars.Forms, nHomeRow{
				ID:     ft.ID,
				Name:   ft.Name,
				DateS:  formStatusLabel(ft, lang),
				Info:   TN(lang, "count.answer", n),
				Closed: !ft.IsOpen(nowUnix()),
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

// nEventStats to liczby imprezy: miejsca w salach, sprzedane miejsca i kwoty.
type nEventStats struct {
	Seats         int64
	Payed         int64
	Ordered       int64
	AmountPayed   int64
	AmountWaiting int64
	Rooms         int
}

// eventStats liczy zajętość i kwoty imprezy (jedno miejsce w kodzie na te sumy).
func eventStats(db *DB, userID, eventID int64) nEventStats {
	var st nEventStats

	rooms, err := db.EventGetRooms(eventID)
	if err != nil {
		log.Printf("eventStats: rooms: %v", err)
	}
	st.Rooms = len(rooms)
	for _, room := range rooms {
		chairs, err := db.FurnitureGetAllByRoomIDOfType(room.ID, "chair")
		if err != nil {
			log.Printf("eventStats: chairs: %v", err)
			continue
		}
		st.Seats += int64(len(chairs))
	}

	rr, err := db.ReservationFullGetAll(userID, eventID)
	if err != nil {
		log.Printf("eventStats: reservations: %v", err)
		return st
	}
	for _, r := range rr {
		switch r.Status {
		case "payed":
			st.Payed++
			st.AmountPayed += r.Price.Int64
		case "ordered":
			st.Ordered++
			st.AmountWaiting += r.Price.Int64
		}
	}
	return st
}

// eventMissing to cicha lista braków: bez ostrzegawczych kolorów, po prostu
// widać w podsumowaniu, czego brakuje (rozdz. 4.1 HARMONIJKA.md).
func eventMissing(db *DB, user User, e Event, st nEventStats, lang string) []string {
	var missing []string
	if st.Rooms == 0 {
		missing = append(missing, T(lang, "home.no_room"))
	}
	if e.DefaultPrice == 0 {
		missing = append(missing, T(lang, "home.no_price"))
	}
	if !e.ThankYouNotificationsID.Valid || e.ThankYouNotificationsID.Int64 == 0 {
		missing = append(missing, T(lang, "home.no_mail"))
	}
	if e.DefaultPrice > 0 && (!e.BankAccountsID.Valid || e.BankAccountsID.Int64 == 0) {
		missing = append(missing, T(lang, "home.no_bank"))
	}
	return missing
}

// --- katalogi (sale, maile, konta) ----------------------------------------

type nCatalogRoom struct {
	ID     int64
	Name   string
	Size   string
	Seats  string
	Shared bool
}

type nCatalogMail struct {
	ID        int64
	Name      string
	RelatedTo string
	Title     string
	Shared    bool
}

type nCatalogAccount struct {
	ID       int64
	Name     string
	IBAN     string
	Currency string
}

type nCatalogsVars struct {
	NPage
	Rooms    []nCatalogRoom
	Mails    []nCatalogMail
	Accounts []nCatalogAccount
}

// CatalogsPage - /admin/catalogs: wspólne sale, maile i konta.
func CatalogsPage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		lang := PanelLang(r)

		vars := nCatalogsVars{
			NPage: NPage{Title: T(lang, "catalog.title"), OrgName: orgName(user)},
		}

		rooms, err := db.RoomGetAllByUserID(user.ID)
		if err != nil {
			log.Printf("CatalogsPage: rooms: %v", err)
		}
		for _, room := range rooms {
			seats := ""
			if chairs, err := db.FurnitureGetAllByRoomIDOfType(room.ID, "chair"); err == nil {
				seats = TN(lang, "count.seat", int64(len(chairs)))
			}
			vars.Rooms = append(vars.Rooms, nCatalogRoom{
				ID:     room.ID,
				Name:   room.Name,
				Size:   T(lang, "catalog.size", room.Width, room.Height),
				Seats:  seats,
				Shared: room.Sharable.Valid && room.Sharable.Int64 != 0,
			})
		}

		mails, err := db.NotificationGetAllForUser(user.ID)
		if err != nil {
			log.Printf("CatalogsPage: mails: %v", err)
		}
		for _, m := range mails {
			related := m.RelatedTo
			switch m.RelatedTo {
			case "events":
				related = T(lang, "catalog.for_events")
			case "forms":
				related = T(lang, "catalog.for_forms")
			}
			vars.Mails = append(vars.Mails, nCatalogMail{
				ID:        m.ID,
				Name:      m.Name,
				RelatedTo: related,
				Title:     m.Title.String,
				Shared:    m.Sharable,
			})
		}

		accounts, err := db.BankAccountGetAll(user.ID)
		if err != nil {
			log.Printf("CatalogsPage: bank accounts: %v", err)
		}
		for _, a := range accounts {
			vars.Accounts = append(vars.Accounts, nCatalogAccount{
				ID:       a.ID,
				Name:     a.Name,
				IBAN:     a.IBAN,
				Currency: a.Currency,
			})
		}

		nRender(w, lang, vars, "tmpl/n_catalogs.html")
	}
}

// nearestEventID zwraca imprezę najbliższą dziś (przyszłą, a jak nie ma, to
// ostatnią). Używane tam, gdzie stary ekran wymaga podania imprezy.
func nearestEventID(db *DB, userID int64) int64 {
	events, err := db.EventGetAllByUserID(userID)
	if err != nil {
		log.Printf("nearestEventID: %v", err)
		return 0
	}
	now := time.Now().Unix()
	var future, past Event
	for _, e := range events {
		if e.Date >= now && (future.ID == 0 || e.Date < future.Date) {
			future = e
		}
		if e.Date < now && e.Date > past.Date {
			past = e
		}
	}
	if future.ID != 0 {
		return future.ID
	}
	return past.ID
}

// --- strona imprezy --------------------------------------------------------

type nRoomText struct {
	Title                string
	DescLabel, DescField string
	Desc                 template.HTML
	Banner               nBannerPick
}

type nResRow struct {
	Person      string
	Seats       string
	StatusLabel string
	StatusClass string
	Amount      string
}

// nEventFormChoice - formularz na liście w sekcji "Formularze" imprezy.
// OtherEvent - nazwa innej imprezy, do której formularz jest już podpięty
// (zaznaczenie go tutaj przepina go do tej imprezy).
type nEventFormChoice struct {
	ID           int64
	Name         string
	Checked      bool
	OtherEventID int64
	OtherEvent   string
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
	MainBanner     nBannerPick
	RoomTexts      []nRoomText
	OrderHowto     template.HTML
	OrderNotesDesc string
	NoSitsTitle    string
	NoSitsText     template.HTML
	OrderedTitle   string
	OrderedText    template.HTML
	HowTo          template.HTML
	// OrderVars - zmienne zamówienia dla pastylek w "Po zamówieniu – tekst"
	// (JSON jak w edytorze maila, mailEditorVars)
	OrderVars string

	Notifications  []Notification
	ThankYouMailID int64
	AdminMailID    int64
	BankAccounts   []BankAccount
	BankAccountID  int64
	// PayMessage - początek wiadomości dla odbiorcy przelewu
	PayMessage string

	Forms []FormTemplate
	// FormChoices - wszystkie formularze organizacji do podpięcia w sekcji
	FormChoices []nEventFormChoice

	Sharable  bool
	HideCosts bool
	// Draft - impreza jeszcze nieopublikowana (goście jej nie widzą)
	Draft bool
	// PreviousEventID i OtherEvents - powiązanie z poprzednią edycją
	PreviousEventID int64
	OtherEvents     []Event

	// Theme - temat strony gościa, Themes - lista do wyboru (themes.go)
	Theme  string
	Themes []GuestTheme

	SumBasic, SumDates, SumRoom, SumGuest string
	SumMailGuest, SumMailAdmin, SumBank   string
	SumForms, SumRes, SumSettings         string
	MissMailGuest, MissMailAdmin          bool
	MissBank                              bool

	SeatsPayed, SeatsOrdered, SeatsFree int64
	PctPayed, PctOrdered                int64
	ResRows                             []nResRow

	SumReport   string
	Report      []nReportRow
	ReportTotal nReportRow
	Occupancy   int64

	SumAddons        string
	AddonsTemplateID int64
	AddonFields      []nAddonField

	// porównanie z poprzednią edycją (rozdz. 4.2 HARMONIJKA.md)
	CompareName  string
	CompareLine  string
	CompareNow   int64
	CompareThen  int64
	CompareMoney string

	// podsumowanie dodatków w zamówieniach (lista dla kuchni, rozdz. 9.2)
	AddonSummary   []nAddonSummaryRow
	SurchargeRows  []nSurchargeRow
	SurchargeTotal string
}

// nAddonSummaryRow - ile razy wybrano daną odpowiedź w polu dodatków.
type nAddonSummaryRow struct {
	Field string
	Value string
	Count int64
}

// nSurchargeRow - ile sztuk dopłaty zamówiono i za ile.
type nSurchargeRow struct {
	Description string
	Amount      int64
	Sum         string
}

// nReportRow to jedna linia rozliczenia: ile miejsc sprzedano po danej cenie
// i ile z tego już wpłynęło.
type nReportRow struct {
	Price    string
	Seats    int64
	Sum      string
	Payed    string
	Waiting  string
	rawPrice int64
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
	var formChoices []nEventFormChoice
	for _, ft := range templs {
		if isAddonsTemplate(ft) {
			continue // dodatki mają własną sekcję
		}
		c := nEventFormChoice{ID: ft.ID, Name: ft.Name}
		if ft.EventID.Valid && ft.EventID.Int64 == event.ID {
			eventForms = append(eventForms, ft)
			c.Checked = true
		} else if ft.EventID.Valid && ft.EventID.Int64 != 0 {
			c.OtherEventID = ft.EventID.Int64
		}
		formChoices = append(formChoices, c)
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
			UpURL:   "/admin",
			UpLabel: T(lang, "nav.home"),
		},
		ID:       event.ID,
		Name:     event.Name,
		Language: language,
		// podgląd nowej strony gościa dla TEJ imprezy (bez ?event-id strona
		// pokazuje imprezę z otwartą sprzedażą)
		GuestURL:        fmt.Sprintf("/res/%s?event-id=%d", user.URL, event.ID),
		DateVal:         dateInputValue(event.Date),
		FromVal:         dateInputValue(event.FromDate),
		ToVal:           dateInputValue(event.ToDate),
		Price:           event.DefaultPrice,
		Currency:        event.DefaultCurrency,
		RoomsVal:        getRoomsString(evRooms),
		UserRooms:       userRooms,
		EventRooms:      evRooms,
		MainDesc:        template.HTML(event.MainDesc.String),
		MainBanner:      bannerPick(T(lang, "field.main_banner"), "mainbanner", event.MainBanner.String, user),
		OrderHowto:      template.HTML(event.OrderHowto),
		OrderNotesDesc:  event.OrderNotesDescription,
		NoSitsTitle:     event.NoSitsSelectedTitle,
		NoSitsText:      template.HTML(event.NoSitsSelectedText),
		OrderedTitle:    event.OrderedNoteTitle,
		OrderedText:     template.HTML(event.OrderedNoteText),
		HowTo:           template.HTML(event.HowTo),
		OrderVars:       mailEditorVars(db, user),
		Notifications:   notifs,
		ThankYouMailID:  event.ThankYouNotificationsID.Int64,
		AdminMailID:     event.AdminNotificationsID.Int64,
		BankAccounts:    bankAccs,
		BankAccountID:   event.BankAccountsID.Int64,
		PayMessage:      event.PayMessage,
		Forms:           eventForms,
		FormChoices:     formChoices,
		Sharable:        event.Sharable.Bool,
		HideCosts:       event.HideCosts.Bool,
		Draft:           event.Status.String == "draft",
		PreviousEventID: event.PreviousEventID.Int64,
		Theme:           ThemeKey(event.Theme.String),
		Themes:          guestThemes,
	}

	// do wyboru poprzedniej edycji: inne imprezy organizacji, od najnowszej
	allEvents, err := db.EventGetAllByUserID(user.ID)
	if err != nil {
		log.Printf("buildEventVars: events: %v", err)
	}
	sort.Slice(allEvents, func(i, j int) bool { return allEvents[i].Date > allEvents[j].Date })
	eventNames := map[int64]string{}
	for _, e := range allEvents {
		eventNames[e.ID] = e.Name
		if e.ID != event.ID {
			v.OtherEvents = append(v.OtherEvents, e)
		}
	}
	for i := range v.FormChoices {
		v.FormChoices[i].OtherEvent = eventNames[v.FormChoices[i].OtherEventID]
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
			Title:     T(lang, "field.room_n", i+1) + name,
			DescLabel: T(lang, "field.desc"),
			DescField: fmt.Sprintf("room%ddesc", i+1),
			Desc:      template.HTML(descs[i]),
			Banner:    bannerPick(T(lang, "field.main_banner"), fmt.Sprintf("room%dbanner", i+1), banners[i], user),
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
	v.SumGuest = T(lang, "theme.label") + ": " + ThemeName(lang, event.Theme.String) + ", " + guest

	v.SumMailGuest, v.MissMailGuest = notificationName(db, user, event.ThankYouNotificationsID.Int64, lang)
	v.SumMailAdmin, v.MissMailAdmin = notificationName(db, user, event.AdminNotificationsID.Int64, lang)

	if event.BankAccountsID.Valid && event.BankAccountsID.Int64 != 0 {
		if ba, err := db.BankAccountGetByID(event.BankAccountsID.Int64, user.ID); err == nil {
			v.SumBank = ba.Name + ", " + ba.IBAN
			if msg := event.PayMessage; msg != "" {
				v.SumBank += " · " + T(lang, "pay.message") + ": " + msg
			}
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
	if event.Status.String == "draft" {
		settings = append(settings, T(lang, "event.draft"))
	}
	if event.Sharable.Bool {
		settings = append(settings, T(lang, "field.sharable"))
	}
	if event.HideCosts.Bool {
		settings = append(settings, T(lang, "field.hidecosts"))
	}
	if event.PreviousEventID.Valid && event.PreviousEventID.Int64 != 0 {
		if prev, err := db.EventGetByID(event.PreviousEventID.Int64); err == nil && prev.UserID == user.ID {
			settings = append(settings, T(lang, "field.previous_edition")+": "+prev.Name)
		}
	}
	if len(settings) == 0 {
		settings = append(settings, T(lang, "common.none"))
	}
	v.SumSettings = strings.Join(settings, ", ")

	fillReservationSummary(db, user, event, lang, v)
	fillEventReport(db, user, event, lang, v)

	// dodatki do zamówienia (rozdz. 9 HARMONIJKA.md)
	v.AddonsTemplateID = event.AddonsFormTemplateID.Int64
	v.AddonFields = addonFields(db, event)
	switch {
	case v.AddonsTemplateID == 0:
		v.SumAddons = T(lang, "common.none")
	case len(v.AddonFields) == 0:
		v.SumAddons = T(lang, "addons.no_fields")
	default:
		names := make([]string, 0, len(v.AddonFields))
		for _, f := range v.AddonFields {
			names = append(names, f.Label)
		}
		v.SumAddons = T(lang, "form.fields_count", len(v.AddonFields)) + ": " + strings.Join(names, ", ")
	}
}

// fillEventReport liczy rozliczenie imprezy po cenach miejsc.
// Bez porównania z poprzednią edycją - to czeka na kolumnę
// events.previous_event_id z tools/migration-redesign.sql.
func fillEventReport(db *DB, user User, event Event, lang string, v *nEventVars) {
	rr, err := db.ReservationFullGetAll(user.ID, event.ID)
	if err != nil {
		log.Printf("fillEventReport: %v", err)
	}

	type bucket struct {
		seats   int64
		sum     int64
		payed   int64
		waiting int64
		curr    string
	}
	buckets := map[int64]*bucket{}
	var order []int64
	var total bucket

	for _, r := range rr {
		if r.Status != "payed" && r.Status != "ordered" {
			continue
		}
		price := r.Price.Int64
		b, ok := buckets[price]
		if !ok {
			b = &bucket{curr: event.DefaultCurrency}
			buckets[price] = b
			order = append(order, price)
		}
		if r.Currency.Valid && r.Currency.String != "" {
			b.curr = r.Currency.String
		}

		b.seats++
		b.sum += price
		total.seats++
		total.sum += price
		if r.Status == "payed" {
			b.payed += price
			total.payed += price
		} else {
			b.waiting += price
			total.waiting += price
		}
	}

	sort.Slice(order, func(i, j int) bool { return order[i] > order[j] })

	for _, price := range order {
		b := buckets[price]
		v.Report = append(v.Report, nReportRow{
			Price:    FormatMoney(price, b.curr),
			Seats:    b.seats,
			Sum:      FormatMoney(b.sum, b.curr),
			Payed:    FormatMoney(b.payed, b.curr),
			Waiting:  FormatMoney(b.waiting, b.curr),
			rawPrice: price,
		})
	}

	v.ReportTotal = nReportRow{
		Seats:   total.seats,
		Sum:     FormatMoney(total.sum, event.DefaultCurrency),
		Payed:   FormatMoney(total.payed, event.DefaultCurrency),
		Waiting: FormatMoney(total.waiting, event.DefaultCurrency),
	}

	seats := eventStats(db, user.ID, event.ID).Seats
	if seats > 0 {
		v.Occupancy = (v.SeatsPayed + v.SeatsOrdered) * 100 / seats
	}
	v.SumReport = T(lang, "report.summary",
		fmt.Sprintf("%d%%", v.Occupancy), FormatMoney(total.payed, event.DefaultCurrency))

	fillEditionComparison(db, user, event, lang, v)
	fillAddonSummary(db, event, lang, v)
}

// fillAddonSummary zbiera dodatki ze wszystkich zamówień imprezy: co wybierano
// (lista dla kuchni) i ile dopłat zamówiono.
func fillAddonSummary(db *DB, event Event, lang string, v *nEventVars) {
	counts, err := db.AddonAnswerSummary(event.ID)
	if err != nil {
		log.Printf("fillAddonSummary: answers: %v", err)
	}
	priced := pricedAddonLabels(db, event)
	for _, c := range counts {
		if strings.TrimSpace(c.Value) == "" {
			continue
		}
		if priced[plainLabel(c.Field)] {
			continue // pola z ceną są niżej, w dopłatach
		}
		v.AddonSummary = append(v.AddonSummary, nAddonSummaryRow{
			Field: plainLabel(c.Field),
			Value: c.Value,
			Count: c.Count,
		})
	}

	items, err := db.OrderItemsGetByEvent(event.ID)
	if err != nil {
		log.Printf("fillAddonSummary: items: %v", err)
		return
	}

	type agg struct {
		amount int64
		sum    int64
		curr   string
	}
	byDesc := map[string]*agg{}
	var order []string
	var total int64

	for _, it := range items {
		a, ok := byDesc[it.Description]
		if !ok {
			a = &agg{curr: event.DefaultCurrency}
			byDesc[it.Description] = a
			order = append(order, it.Description)
		}
		if it.Currency.Valid && it.Currency.String != "" {
			a.curr = it.Currency.String
		}
		a.amount += it.Amount
		a.sum += it.Amount * it.Price
		total += it.Amount * it.Price
	}

	for _, d := range order {
		a := byDesc[d]
		v.SurchargeRows = append(v.SurchargeRows, nSurchargeRow{
			Description: d,
			Amount:      a.amount,
			Sum:         FormatMoney(a.sum, a.curr),
		})
	}
	if total > 0 {
		v.SurchargeTotal = FormatMoney(total, event.DefaultCurrency)
	}
}

// fillEditionComparison porównuje sprzedaż z poprzednią edycją w tym samym
// momencie: "21 dni przed imprezą teraz 58 miejsc, rok temu 49". Porównywanie
// po dacie kalendarzowej nie ma sensu, bo imprezy wypadają w różne dni.
func fillEditionComparison(db *DB, user User, event Event, lang string, v *nEventVars) {
	if !event.PreviousEventID.Valid || event.PreviousEventID.Int64 == 0 || event.Date <= 0 {
		return
	}
	prev, err := db.EventGetByID(event.PreviousEventID.Int64)
	if err != nil || prev.UserID != user.ID || prev.Date <= 0 {
		return
	}

	now := time.Now().Unix()
	const day = int64(24 * 3600)
	daysLeft := (event.Date - now) / day

	// ten sam punkt sprzedaży poprzedniej edycji
	cutoff := prev.Date - daysLeft*day
	if daysLeft <= 0 {
		cutoff = prev.Date // impreza już była: porównujemy całość
	}

	seats, sum, err := db.ReservationsSoldBefore(prev.ID, cutoff)
	if err != nil {
		log.Printf("fillEditionComparison: %v", err)
		return
	}

	v.CompareName = prev.Name
	v.CompareNow = v.SeatsPayed + v.SeatsOrdered
	v.CompareThen = seats
	v.CompareMoney = FormatMoney(sum, prev.DefaultCurrency)

	if daysLeft > 0 {
		v.CompareLine = T(lang, "report.compare_before",
			TN(lang, "count.day", daysLeft), v.CompareNow, prev.Name, seats)
	} else {
		v.CompareLine = T(lang, "report.compare_total", v.CompareNow, prev.Name, seats)
	}
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
		rooms  []string           // sale w kolejności pojawienia się
		seats  map[string][]int64 // numery miejsc per sala
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
				seats:  map[string][]int64{},
				payed:  true,
			}
			if g.person == "" {
				g.person = r.CustEmail.String
			}
			groups[key] = g
			order = append(order, key)
		}
		g.any = true
		if _, ok := g.seats[r.RoomName]; !ok {
			g.rooms = append(g.rooms, r.RoomName)
		}
		g.seats[r.RoomName] = append(g.seats[r.RoomName], r.ChairNumber)
		g.amount += r.Price.Int64
		if r.Currency.Valid && r.Currency.String != "" {
			g.curr = r.Currency.String
		}
		if r.Status != "payed" {
			g.payed = false
			waiting += r.Price.Int64
		}
	}

	seats := eventStats(db, user.ID, event.ID).Seats
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
			Seats:       seatsText(g.rooms, g.seats),
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
			event.MainBanner = ToNS(cleanBanner(val("mainbanner")))
			event.Room1Desc = ToNS(html("room1desc"))
			event.Room1Banner = ToNS(cleanBanner(val("room1banner")))
			event.Room2Desc = ToNS(html("room2desc"))
			event.Room2Banner = ToNS(cleanBanner(val("room2banner")))
			event.Room3Desc = ToNS(html("room3desc"))
			event.Room3Banner = ToNS(cleanBanner(val("room3banner")))
			event.Room4Desc = ToNS(html("room4desc"))
			event.Room4Banner = ToNS(cleanBanner(val("room4banner")))
			event.OrderHowto = html("order_howto")
			event.OrderNotesDescription = val("order_notes_desc")
			event.NoSitsSelectedTitle = val("no_sits_selected_title")
			event.NoSitsSelectedText = html("no_sits_selected_text")
			event.OrderedNoteTitle = val("ordered_note_title")
			event.OrderedNoteText = html("ordered_note_text")
			// tekst jest szablonem z danymi zamówienia (ParseOrderTmpl) - zepsuty
			// pokazałby gościowi surowe {{...}}
			if _, err := template.New("ordered").Parse(event.OrderedNoteText); err != nil {
				jsonMsg(w, http.StatusBadRequest, nSectionResp{Msg: T(lang, "field.ordered_text") + ": " + err.Error()})
				return
			}
			event.HowTo = html("how_to")
			event.Theme = ToNS(ThemeKey(val("theme")))
			if err := db.EventSetTheme(event.ID, user.ID, event.Theme.String); err != nil {
				jsonMsg(w, http.StatusTeapot, nSectionResp{Msg: err.Error()})
				return
			}
		case "mail_guest":
			event.ThankYouNotificationsID = ToNI(id64("thankyou_notifications_id_fk"))
		case "mail_admin":
			event.AdminNotificationsID = ToNI(id64("admin_notifications_id_fk"))
		case "bank":
			event.BankAccountsID = ToNI(id64("bank_account_id"))
			event.PayMessage = strings.TrimSpace(val("pay_message"))
			if err := db.EventSetPayMessage(event.ID, user.ID, event.PayMessage); err != nil {
				jsonMsg(w, http.StatusTeapot, nSectionResp{Msg: err.Error()})
				return
			}
		case "forms":
			if err := setEventForms(db, user.ID, event.ID, flag); err != nil {
				jsonMsg(w, http.StatusTeapot, nSectionResp{Msg: T(lang, "common.save_error") + ": " + err.Error()})
				return
			}
		case "settings":
			event.Sharable = ToNB(flag("sharable"))
			event.HideCosts = ToNB(flag("hidecosts"))
			if err := db.EventSetHideCosts(event.ID, user.ID, flag("hidecosts")); err != nil {
				log.Printf("EventSectionSave: hidecosts: %v", err)
			}

			// szkic: impreza nie pokazuje się gościom
			status := "published"
			if flag("draft") {
				status = "draft"
			}
			event.Status = ToNS(status)
			if err := db.EventSetStatus(event.ID, user.ID, status); err != nil {
				log.Printf("EventSectionSave: status: %v", err)
			}

			prev := ToNI(id64("previous_event_id"))
			event.PreviousEventID = prev
			if err := db.EventSetPreviousEdition(event.ID, user.ID, prev); err != nil {
				log.Printf("EventSectionSave: previous edition: %v", err)
				jsonMsg(w, http.StatusTeapot, nSectionResp{Msg: err.Error()})
				return
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
		case "forms":
			resp.Summary = v.SumForms
		case "settings":
			resp.Summary = v.SumSettings
		}
		jsonMsg(w, http.StatusOK, resp)
	}
}

// setEventForms podpina do imprezy zaznaczone formularze (pola "form_<id>")
// i odpina te, które były jej, a zostały odznaczone. Formularz zaznaczony
// tutaj, a podpięty do innej imprezy, zostaje przepięty.
func setEventForms(db *DB, userID, eventID int64, flag func(string) bool) error {
	templs, err := db.FormTemplateGetAll(userID)
	if err != nil {
		return err
	}
	for _, ft := range templs {
		if isAddonsTemplate(ft) {
			continue
		}
		mine := ft.EventID.Valid && ft.EventID.Int64 == eventID
		on := flag(fmt.Sprintf("form_%d", ft.ID))
		switch {
		case on && !mine:
			err = db.FormTemplateSetEvent(ft.ID, userID, ToNI(eventID))
		case !on && mine:
			err = db.FormTemplateSetEvent(ft.ID, userID, ToNI(0))
		}
		if err != nil {
			return err
		}
	}
	return nil
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

// seatsText: "Sala A: 1–4, 7; Sala B: 2" - nazwa sali raz, kolejne numery
// jako zakres (jak w tabeli rezerwacji, js/n_reservations.js).
func seatsText(rooms []string, seats map[string][]int64) string {
	parts := make([]string, 0, len(rooms))
	for _, room := range rooms {
		nums := append([]int64(nil), seats[room]...)
		sort.Slice(nums, func(i, j int) bool { return nums[i] < nums[j] })
		var runs []string
		for i := 0; i < len(nums); {
			j := i
			for j+1 < len(nums) && nums[j+1] == nums[j]+1 {
				j++
			}
			if j > i {
				runs = append(runs, fmt.Sprintf("%d–%d", nums[i], nums[j]))
			} else {
				runs = append(runs, strconv.FormatInt(nums[i], 10))
			}
			i = j + 1
		}
		parts = append(parts, room+": "+strings.Join(runs, ", "))
	}
	return strings.Join(parts, "; ")
}

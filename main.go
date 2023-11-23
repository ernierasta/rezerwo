package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

const (
	AUTHCOOKIE = "auth"
	MEDIAROOT  = "media"
)

func main() {

	conf := SettingsNew()
	conf.Read("config.toml")

	loc, err := time.LoadLocation(conf.Location)
	if err != nil {
		log.Println(err)
	}
	dateFormat := "2006-01-02"

	cookieStore := sessions.NewCookieStore(conf.AuthenticationKey, conf.EncryptionKey)
	cookieStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   60 * 60 * 2, //2 hours
		HttpOnly: true,
	}

	lang := "pl-PL"

	db := initDB()
	defer db.Close()

	stop := make(chan bool)
	Ticker5min(db, stop)
	defer TickerStop(stop)

	rtr := mux.NewRouter()
	rtr.HandleFunc("/res/{user}", ReservationHTML(db, lang))
	rtr.HandleFunc("/", AboutHTML(lang))

	handleStatic("js")
	handleStatic("css")
	handleStatic("img")
	handleStatic("media") //user data
	http.Handle("/", rtr)
	http.HandleFunc("/order", ReservationOrderHTML(db, lang))
	http.HandleFunc("/order/status", ReservationOrderStatusHTML(db, lang, &MailConfig{Server: conf.MailServer, Port: int(conf.MailPort), From: conf.MailFrom, User: conf.MailUser, Pass: conf.MailPass, IgnoreCert: conf.MailIgnoreCert}))
	http.HandleFunc("/admin/login", AdminLoginHTML(db, lang, cookieStore))
	http.HandleFunc("/admin", AdminMainPage(db, loc, lang, dateFormat, cookieStore))
	http.HandleFunc("/admin/designer", DesignerHTML(db, lang))
	http.HandleFunc("/admin/event", EventEditor(db, lang, cookieStore))
	http.HandleFunc("/admin/reservations", AdminReservations(db, lang, cookieStore))
	http.HandleFunc("/passreset", PasswdReset(db))
	http.HandleFunc("/api/login", LoginAPI(db, cookieStore))
	http.HandleFunc("/api/room", DesignerSetRoomSize(db))
	http.HandleFunc("/api/furnit", DesignerMoveObject(db))
	http.HandleFunc("/api/furdel", DesignerDeleteObject(db))
	http.HandleFunc("/api/ordercancel", OrderCancel(db))
	http.HandleFunc("/api/renumber", DesignerRenumberType(db))
	http.HandleFunc("/api/resstatus", ReservationChangeStatusAPI(db))
	http.HandleFunc("/api/resdelete", ReservationDeleteAPI(db))

	log.Fatal(http.ListenAndServe(":3002", nil))
}

func initDB() *DB {
	howto := `<h1>Legenda:</h1>
	<ul>
		<li><span class="free-text">Zielony</span> = możliwa rezerwacja.</li>
		<li><span class="marked-text">Żółty</span> = ktoś wybrał miejsce, ale jeszcze nie dokonał rezerwacji.
		<li><span class="ordered-text">Pomarańczowy</span> = zarezerwowane.</li>
		<li><span class="payed-text">Czerwony</span> = zapłacono.</li>
		<li><span class="disabled-text">Czarny</span> = aktualnie miejsca niedostępne.</li>
	</ul>
	<p>Cena biletu: <b>400 Kč</b>. W cenie biletu:</p>
	<ul>
		<li>Wstęp na bal.</li>
		<li>Miejscówka.</li>
		<li>Welcome drink.</li>
		<li>Smaczna kolacja.</li>
		<li>Woda i mały poczęstunek na stole.</li>
		<li>Super muzyka.</li>
		<li>Ciekawy program.</li>
	</ul>`

	howtoP := `<h1>Legenda:</h1>
	<ul>
		<li><span class="free-text">Zielony</span> = możliwa rezerwacja.</li>
		<li><span class="marked-text">Żółty</span> = ktoś wybrał miejsce, ale jeszcze nie dokonał rezerwacji.
		<li><span class="ordered-text">Pomarańczowy</span> = zarezerwowane.</li>
		<li><span class="payed-text">Czerwony</span> = zapłacono.</li>
		<li><span class="disabled-text">Czarny</span> = aktualnie miejsca niedostępne.</li>
	</ul>
	<p>Cena biletu: <b>400 Kč</b>. W cenie biletu:</p>
	<ul>
		<li>Wstęp na bal.</li>
		<li>Miejscówka.</li>
		<li>Smaczna kolacja.</li>
		<li>Woda i mały poczęstunek na stole.</li>
		<li>Super muzyka.</li>
		<li>Ciekawy program.</li>
	</ul>`

	mailText := `Szanowni Państwo,
dziękujemy za dokonanie rezerwacji biletów na Bal Macierzy Szkolnej przy PSP w Karwinie-Frysztacie.
Niniejszym mailem potwierdzamy zamówienie miejsc: {{.Sits}}.
Łączny koszt biletów wynosi: {{.TotalPrice}}.
Bal odbędzie się w piątek 7 lutego 2020 od godziny 19:00 w Domu Przyjaźni w Karwinie.

Uwaga! Dokonali Państwo tylko rezerwacji biletów.
Sprzedaż biletów odbędzie się w czwartek 21. 11. 2019 w budynku szkolnym od godziny 16:00 (przed zebraniami klasowymi) do godziny 17:30 (lub dłużej, o ile zebrania się przeciągną).
Dodatkowy termin zakupu biletów to wtorek 26. 11. 2019 (16:00 – 16:45) przy wejściu do szkoły.
W obu wymienionych terminach można również przekazać deklarację.

Zarezerwowane miejsca, które po 26. 11. 2019 nie zostaną opłacone, zostaną zwolnione.
W przypadku pytań lub wątpliwości prosimy o kontakt mailowy - pspmacierzkarwina@seznam.cz

Dziękujemy serdecznie!

Zarząd MSz przy PSP w Karwinie-Frysztacie
`
	mailTextP := `Szanowni Państwo,
dziękujemy za dokonanie rezerwacji biletów na BAL POLSKI organizowany 
miejskimi kołami PZKO w Karwinie.
Niniejszym mailem potwierdzamy zamówienie miejsc: {{.Sits}}.
Łączny koszt biletów wynosi: {{.TotalPrice}}.
Bal odbędzie się w piątek 24 stycznia 2020 od godziny 19:00 w Domu 
Przyjaźni w Karwinie.

Uwaga! Dokonali Państwo tylko rezerwacji biletów.
Sprzedaż biletów odbędzie się w środę 4.12.2019 w Domu Polskim MK PZKO 
Karwina-Frysztat od 16:30 do 18:00.
Dodatkowy termin zakupu biletów to środa 11.12.2019 od 16:30 do 18:00 w 
tym samym miejscu.

Zarezerwowane miejsca, które po 11.12.2019 nie zostaną opłacone, 
zostaną zwolnione.

W przypadku pytań lub wątpliwości prosimy o kontakt mailowy -
pzkokarwina@pzkokarwina.cz

Dziękujemy serdecznie!

Organizatorzy BALU POLSKIEGO w Karwinie
`

	roomDescription1 := `<div class="alert alert-warning" role="alert">Zapraszamy również na Balkon na 1. piętrze, tam pozostało jeszcze sporo wolnych miejsc.</div>
Koło Macierzy Szkolnej zaprasza wszystkich na bal pt. <b>„ROZTAŃCZMY PRZYJAŹŃ …”</b>,<br />
który odbędzie się w piątek <b>7 lutego 2020</b> od godziny 19:00 w Domu Przyjaźni w Karwinie.<br />
W celu zakupu biletów potrzebna jest wcześniejsza rezerwacja.<br />
W górnej części ekranu wybrać można zakładkę <b>"Sala główna - parter"</b> lub <b>"Balkon - 1. piętro"</b>.<br />
Proszę wybrać wolne miejsca (krzesła) i kliknąć na przycisk "Zamów", które przekieruje Państwa do formularza rezerwacji.`
	roomDescription2 := `Koło Macierzy Szkolnej zaprasza wszystkich na bal pt. <b>„ROZTAŃCZMY PRZYJAŹŃ …”</b>,<br />
który odbędzie się w piątek <b>7 lutego 2020</b> od godziny 19:00 w Domu Przyjaźni w Karwinie.<br />
W celu zakupu biletów potrzebna jest wcześniejsza rezerwacja.<br />
W górnej części ekranu wybrać można zakładkę <b>"Sala główna - parter"</b> lub <b>"Balkon - 1. piętro"</b>.<br />
Proszę wybrać wolne miejsca (krzesła) i kliknąć na przycisk "Zamów", które przekieruje Państwa do formularza rezerwacji.`
	roomDescription1P := `Miejskie Koła PZKO w Karwinie zapraszają na <b>„BAL POLSKI”</b>, który odbędzie 
się w piątek <b>24 stycznia 2020</b> od godziny 19:00 w Domu Przyjaźni w Karwinie.<br>
W celu zakupu biletów potrzebna jest wcześniejsza rezerwacja.
W górnej części ekranu wybrać można zakładkę <b>"Sala główna - parter"</b> lub 
<b>"Balkon - 1. piętro</b>".<br>
Proszę wybrać wolne miejsca (krzesła) i kliknąć na przycisk <b>"Zamów"</b>, 
które przekieruje Państwa do formularza rezerwacji.
`

	orderHowto := `W celu dokonania rezerwacji prosimy o wypełnienie poniższych danych. W przypadku kiedy Państwo dokonują rezerwacji większej ilości biletów, prosimy o podanie nazwisk osób, dla których są miejsca przeznaczone (wystarczy 1 nazwisko na 2 bilety).
Na podany przez Państwa mail zostanie wysłany mail z potwierdzeniem rezerwacji oraz z informacją na temat zakupu biletów.`
	orderedNoteTitle := "Pomyślnie zarezerwowano bilety!"
	orderedNoteText := `Na podany przez Państwa mail zostanie wysłany mail z potwierdzeniem rezerwacji oraz informacja na temat zakupu biletów.`

	noSitsSelTitle := "Nie wybrano siedzeń!"
	noSitsSelText := `Nie wybrano siedzeń. Prosimy kliknąć na wolne krzesła, czyli w kolorze <b class="free-text">zielonym</b> i zamówić ponownie.<br />Jeżeli nie ma wolnych krzeseł na parterze, prosimy sprawdzić na balkonie.`

	adminMailSubject := "Rezerwacja: {{.Name}} {{.Surname}}, {{.TotalPrice}}"
	adminMailText := "{{.Name}} {{.Surname}}\nkrzesła: {{.Sits}} sale: {{.Rooms}}\nŁączna cena: {{.TotalPrice}}\nEmail: {{.Email}}\nTel: {{.Phone}}\nNotatki:{{.Notes}}"

	db := DBInit("db.sql")
	db.MustConnect()

	db.StructureCreate()

	// macierz Karwina
	uID, err := db.UserAdd(&User{ID: 1, Email: "pspmacierzkarwina@seznam.cz", URL: "mskarwina", Passwd: "MagikINFO2019"})
	if err != nil {
		log.Println(err)
	}

	r1ID, err := db.RoomAdd(&Room{ID: 1, Name: "Sala główna - parter", Description: ToNS(roomDescription1), Width: 1000, Height: 1000})
	if err != nil {
		log.Println(err)
	}
	r2ID, err := db.RoomAdd(&Room{ID: 2, Name: "Balkon - 1. piętro", Description: ToNS(roomDescription2), Width: 500, Height: 500})
	if err != nil {
		log.Println(err)
	}
	err = db.RoomAssignToUser(uID, r1ID)
	if err != nil {
		log.Println(err)
	}

	err = db.RoomAssignToUser(uID, r2ID)
	if err != nil {
		log.Println(err)
	}
	eID, err := db.EventAdd(&Event{ID: 1, Name: "Bal MS Karwina", Date: 1581033600, FromDate: 1572998400, ToDate: 1580860800, DefaultPrice: 400, DefaultCurrency: "Kč", NoSitsSelectedTitle: noSitsSelTitle, NoSitsSelectedText: noSitsSelText, OrderHowto: orderHowto, OrderNotesDescription: "Prosimy o podanie nazwisk wszystkich rodzin, dla których przeznaczone są bilety.", OrderedNoteTitle: orderedNoteTitle, OrderedNoteText: orderedNoteText, MailSubject: "Rezerwacja biletów na Bal Macierzy", MailText: mailText, AdminMailSubject: adminMailSubject, AdminMailText: adminMailText, HowTo: howto, UserID: 1})
	if err != nil {
		log.Println(err)
	}
	err = db.EventAddRoom(eID, r1ID)
	if err != nil {
		log.Println(err)
	}
	err = db.EventAddRoom(eID, r2ID)
	if err != nil {
		log.Println(err)
	}

	// init PZKO
	uIDP, err := db.UserAdd(&User{ID: 2, Email: "rezerwacja@pzkokarwina.cz", URL: "pzkokarwina", Passwd: "PZKO+007"})
	if err != nil {
		log.Println(err)
		uIDP = 2
	}

	r1IDP, err := db.RoomAdd(&Room{ID: 3, Name: "SALA GŁÓWNA - parter", Description: ToNS(roomDescription1P), Width: 800, Height: 970})
	if err != nil {
		log.Println(err)
	}
	r2IDP, err := db.RoomAdd(&Room{ID: 4, Name: "BALKON - 1. piętro", Description: ToNS(roomDescription1P), Width: 800, Height: 950})
	if err != nil {
		log.Println(err)
	}
	err = db.RoomAssignToUser(uIDP, r1IDP)
	if err != nil {
		log.Println(err)
	}

	err = db.RoomAssignToUser(uIDP, r2IDP)
	if err != nil {
		log.Println(err)
	}
	eIDP, err := db.EventAdd(&Event{ID: 2, Name: "Bal polski", Date: 1581033600, FromDate: 1572998400, ToDate: 1580860800, DefaultPrice: 400, DefaultCurrency: "Kč", NoSitsSelectedTitle: noSitsSelTitle, NoSitsSelectedText: noSitsSelText, OrderHowto: orderHowto, OrderNotesDescription: "Prosimy o podanie nazwisk wszystkich rodzin, dla których przeznaczone są bilety.", OrderedNoteTitle: orderedNoteTitle, OrderedNoteText: orderedNoteText, MailSubject: "Rezerwacja biletów na Bal polski", MailText: mailTextP, AdminMailSubject: adminMailSubject, AdminMailText: adminMailText, HowTo: howtoP, UserID: uIDP})
	if err != nil {
		log.Println(err)
	}
	err = db.EventAddRoom(eIDP, r1IDP)
	if err != nil {
		log.Println(err)
	}
	err = db.EventAddRoom(eIDP, r2IDP)
	if err != nil {
		log.Println(err)
	}
	//db.FurnitureCopyRoom(1, 3)
	//db.FurnitureCopyRoom(2, 4)

	return db
}

func handleStatic(dir string) {
	fs := http.FileServer(http.Dir(dir))
	http.Handle("/"+dir+"/", http.StripPrefix("/"+dir+"/", fs))
}

type DesignerPage struct {
	TableNr, ChairNr, ObjectNr, LabelNr  int64
	HTMLBannerImg                        template.HTML
	HTMLRoomDescription                  template.HTML
	LBLWidth, LBLHeight                  string
	LBLLabelPlaceholder                  string
	BTNSetSize                           string
	BTNAddTable, BTNAddChair             string
	BTNChairDisToggle                    string
	BTNAddLabel, BTNAddObject            string
	LBLDropHere                          string
	BTNSpawnChairs, BTNSave              string
	BTNDelete, BTNRotate                 string
	BTNRenumberChairs, BTNRenumberTables string
}

type ReservationPage struct {
	HTMLHowTo template.HTML
	BTNOrder  string
}

type PageMeta struct {
	LBLLang  string
	LBLTitle string
	DesignerPage
	ReservationPage
	AdminMainPageVars
}

type Page struct {
	PageMeta
	Room
	Event
	Tables  []Furniture
	Chairs  []FurnitureFull
	Objects []Furniture
	Labels  []Furniture
}

type ReservationPageVars struct {
	LBLLang        string
	LBLTitle       string
	LBLNoSitsTitle string
	LBLNoSitsText  template.HTML
	BTNNoSitsOK    string
	Event
	Rooms []RoomVars
}

type RoomVars struct {
	Room
	HTMLBannerImg       template.HTML
	HTMLRoomDescription template.HTML
	HTMLHowTo           template.HTML
	LBLSelected         string
	LBLTotalPrice       string
	BTNOrder            string
	Tables              []Furniture
	Chairs              []FurnitureFull
	Objects             []Furniture
	Labels              []Furniture
}

func DesignerHTML(db *DB, lang string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID := int64(-1)
		eventID := int64(-1)
		if r.Method == "POST" {
			err := r.ParseForm()
			if err != nil {
				log.Printf("DesignerHTML: error parsing form, err: %v", err)
			}
			roomID, err = strconv.ParseInt(r.FormValue("room-id"), 10, 64)
			if err != nil {
				log.Printf("error converting roomID to int, err: %v", err)
			}
			eventIDs := r.FormValue("event-id")
			eventID, err = strconv.ParseInt(eventIDs, 10, 64)
			if err != nil {
				log.Printf("error: DesignerHTML: can not convert %q to int64, err: %v", eventIDs, err)
			}

		} else {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
			//return
		}

		if eventID < 1 {
			log.Printf("error: DesignerHTML: eventID < 1 redirecting to /admin")
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
		}
		if roomID < 1 {
			log.Printf("error: DesignerHTML: roomID < 1 redirecting to /admin")
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
		}

		// user would be needed here, but we will create banner image Path
		// from user name
		userURL := mux.Vars(r)["user"]

		p := GetPageVarsFromDB(db, roomID, eventID)
		//log.Printf("%+v", p)

		// parse banner from pic.png;600;300
		imgName, imgW, imgH := parseBanner(p.Room.Banner.String)

		enPM := PageMeta{
			LBLLang:  lang,
			LBLTitle: "Designer",
			DesignerPage: DesignerPage{
				TableNr:             TableNr(p.Tables) + 1,
				ChairNr:             ChairNrFull(p.Chairs) + 1,
				ObjectNr:            ObjectNr(p.Objects) + 1,
				LabelNr:             LabelNr(p.Labels) + 1,
				HTMLBannerImg:       template.HTML(getImgHTML(imgName, userURL, MEDIAROOT, imgW, imgH)),
				HTMLRoomDescription: template.HTML(p.Room.Description.String),
				LBLWidth:            "Width",
				LBLHeight:           "Height",
				BTNSetSize:          "Set size",
				BTNAddTable:         "Add table",
				BTNAddChair:         "Add chair",
				BTNChairDisToggle:   "Chair disable/enable",
				BTNAddLabel:         "Add label",
				BTNAddObject:        "Add object",
				LBLLabelPlaceholder: "Object label ...",
				LBLDropHere:         "Drop here",
				BTNSpawnChairs:      "Spawn chairs",
				BTNSave:             "Save",
				BTNDelete:           "Delete",
				BTNRotate:           "Rotate",
				BTNRenumberChairs:   "Renumber chairs",
				BTNRenumberTables:   "Renumber tables",
			},
		}
		p.PageMeta = enPM
		t := template.Must(template.ParseFiles("tmpl/a_designer.html", "tmpl/base.html"))
		err := t.ExecuteTemplate(w, "base", p)
		if err != nil {
			log.Print("Designer template executing error: ", err) //log it
		}
	}
}

func ReservationHTML(db *DB, lang string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		v := mux.Vars(r)
		user, err := db.UserGetByURL(v["user"])

		plErr := map[string]string{
			"title": "Nie znaleziono organizacji!",
			"text":  fmt.Sprintf("W bazie nie istnieje organizacja: %q\nProszę sprawdzić poprawność linka.\nJeżeli organizator twierdzi, że jest ok, to proszę o kontakt pod: admin (at) zori.cz.", v["user"]),
		}

		if err != nil {
			log.Printf("error getting user %q, err: %v", v["user"], err)

			ErrorHTML(plErr["title"], plErr["text"], lang, w, r)
			//http.Error(w, fmt.Sprintf("User %q not found! Check You have correct URL!", v["user"]), 500)
			return
		}
		event, err := EventGetCurrent(db, user.ID)
		if err != nil {
			log.Printf("error getting current event for userID: %d, err: %v", user.ID, err)
			ErrorHTML("Nie ma obecnie aktywnych imprez!", "Administrator nie ma obecnie żadnych otwartych imprez.\nProsimy o skontaktowanie się z organizatorem by stwierdzić, kiedy rezerwacje zostaną otwarte.", lang, w, r)
			//http.Error(w, "User have no active events! Come back later, when reservations will be opened!", 500) //TODO: inform about closest user event and when it is
			return
		}
		rr, err := db.EventGetRooms(event.ID)
		if err != nil {
			log.Printf("error getting rooms for eventID: %d, err: %v", event.ID, err)
			plErr := map[string]string{
				"title": "Brak aktywnych sal dla tej imprezy!",
				"text":  "Administrator nie powiązał żadnej sali z wydarzeniem.\nJeżeli po stronie administracji wszystko wygląda ok, to prosimy o informację o tym zdarzeniu na mail: admin (at) zori.cz.\nProsimy o wysłanie nazwy organizacji, której dotyczy problem.",
			}
			ErrorHTML(plErr["title"], plErr["text"], lang, w, r)
			//http.Error(w, fmt.Sprintf("Rooms for user: %q, event: %q not found!", v["user"], e.Name), 500)
			return
		}
		p := ReservationPageVars{
			//EN: LBLTitle: "Reservation",
			LBLTitle: "Rezerwacja",
			//EN: LBLNoSitsTitle: "No sits selected",
			//EN: LBLNoSitsText: "No sits selected, choose some free chairs and try it again",
			//EN: BTNNoSitsOK: "OK",
			LBLLang:        lang,
			LBLNoSitsTitle: event.NoSitsSelectedTitle,
			LBLNoSitsText:  template.HTML(event.NoSitsSelectedText),
			BTNNoSitsOK:    "OK",
			Event:          event,
			Rooms:          []RoomVars{},
		}
		for i := range rr {
			rv := GetFurnituresFromDB(db, rr[i].Name, event.ID)
			rv.Room = rr[i]
			imgName, imgW, imgH := parseBanner(rr[i].Banner.String)
			rv.HTMLBannerImg = template.HTML(getImgHTML(imgName, user.URL, MEDIAROOT, imgW, imgH))
			rv.HTMLRoomDescription = template.HTML(rr[i].Description.String)
			rv.HTMLHowTo = template.HTML(event.HowTo)
			// EN: rv.BTNOrder = "Order"
			// EN: rv.LBLSelected = "Selected"
			// EN: rv.LBLTotalPrice = "Total price"
			rv.LBLSelected = "Wybrano"
			rv.LBLTotalPrice = "Łączna suma"
			rv.BTNOrder = "Zamów"

			p.Rooms = append(p.Rooms, rv)
		}
		t := template.Must(template.ParseFiles("tmpl/reservation.html", "tmpl/base.html"))
		err = t.ExecuteTemplate(w, "base", p)
		if err != nil {
			log.Print("Reservation template executing error: ", err)
		}
	}
}

// parseBanner separates info from db into:
// filename, width, height
// in db it is like:
// myfile.png;400;300
func parseBanner(dbstring string) (string, int, int) {
	if dbstring == "" {
		return "", -1, -1
	}
	ss := strings.Split(dbstring, ";")
	if len(ss) != 3 {
		log.Printf("parseBanner: wrong nr of banner data, len:%d, from db:%v", len(ss), dbstring)
		return "", -1, -1
	}
	w, err := strconv.Atoi(ss[1])
	if err != nil {
		log.Print("parseBanner: wrong img width info in db(can't convert to int):", err)
	}
	h, err := strconv.Atoi(ss[2])
	if err != nil {
		log.Print("parseBanner: wrong img height info in db(can't convert to int):", err)
	}
	return ss[0], w, h
}

// path is unix specific here
func getImgHTML(imgName, userName, mediaRootPath string, w, h int) string {
	if imgName == "" {
		return ""
	}
	return fmt.Sprintf(`<img id="banner_img" src="/%s/%s/%s" width="%d" height="%d">`,
		mediaRootPath,
		userName,
		imgName,
		w, h)
}

type ReservationOrderStatusVars struct {
	LBLLang                  string
	LBLTitle                 string
	LBLStatus, LBLStatusText string
	BTNOk                    string
}

// TODO: split it, too long!
func ReservationOrderStatusHTML(db *DB, lang string, mailConf *MailConfig) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		o := Order{}
		event := Event{}
		user := User{}
		if r.Method == "POST" {
			err := r.ParseForm()
			if err != nil {
				log.Printf("error parsing form data:, err: %v", err)
			}
			eventIDs := r.FormValue("event-id")
			o.EventID, err = strconv.ParseInt(eventIDs, 10, 64)
			if err != nil {
				log.Printf("error: ReservationOrderStatusHTML: can not convert %q to int64, err: %v", eventIDs, err)
			}
			o.Sits = r.FormValue("sits")
			o.Prices = r.FormValue("prices")
			o.Rooms = r.FormValue("rooms")
			o.TotalPrice = r.FormValue("total-price")
			o.Email = r.FormValue("email")
			o.Password = r.FormValue("password") //hidden & unused
			o.Name = r.FormValue("name")
			o.Surname = r.FormValue("surname")
			o.Phone = r.FormValue("phone")
			o.Notes = r.FormValue("notes")

			//fmt.Printf("debug: %+v", o)
			c := Customer{
				Email:   o.Email,
				Passwd:  ToNS(o.Password),
				Name:    ToNS(o.Name),
				Surname: ToNS(o.Surname),
				Phone:   ToNS(o.Phone),
			}
			cID, err := db.CustomerAdd(&c)
			if err != nil {
				log.Printf("error adding customer: %+v, err: %v", c, err)
				plErr := map[string]string{
					"title": "Nie można zapisać zamówienia!",
					"text":  "Wystąpił błąd, nie można zapisać danych zamawiającego, tym samym również zamówienia.",
				}
				ErrorHTML(plErr["title"], plErr["text"], lang, w, r)
				//http.Error(w, fmt.Sprintf("<html><body><b>Can not add customer: %+v, err: %v</b></body></html>", c, err), 500)
				return

				//log.Printf("stare: %v, nowe: %v", c.Passwd.String, o.Password)
				// ER2022: disabled password check, we are not using it anyway
				/*
					if c.Passwd.String == "" || c.Passwd.String != o.Password {
						plErr := map[string]string{
							"title": "Nie można zapisać osoby!",
							"text":  "W bazie już istnieje zamówienie powiązane z tym mailem, lecz nie zgadza się hasło.\nProsimy o podanie poprawnego hasła.",
						}
						ErrorHTML(plErr["title"], plErr["text"], lang, w, r)
						//http.Error(w, fmt.Sprintf("<html><body><b>Can not add customer: %+v, err: %v</b></body></html>", c, err), 500)
						return
					}
				*/
			}

			// password is correct, so continue

			event, err = db.EventGetByID(o.EventID)
			if err != nil {
				log.Printf("error: ReservationOrderStatusHTML: problem getting event by ID: %q, err: %v", o.EventID, err)
			}
			user, err = db.UserGetByID(event.UserID) // TODO: the same trick here, should be ok
			if err != nil {
				log.Printf("error: ReservationOrderStatusHTML: problem getting user by ID(from events table): %q, err: %v", event.UserID, err)
			}
			//TODO: this will fail for returning user, maybe check if exist and do not insert?
			err = db.CustomerAppendToUser(user.ID, cID)
			if err != nil {
				log.Printf("info: ReservationOrderStatusHTML: can not append customer to user, err: %v", err)
			}
			ss, rr, err := SplitSitsRooms(o.Sits, o.Rooms)
			if err != nil {
				log.Printf("error: ReservationOrderStatusHTML: %v", err)
			}

			noteID := int64(0)
			if o.Notes != "" {
				noteID, err = db.NoteAdd(o.Notes)
				if err != nil {
					log.Println(err)
				}
			}

			for i := range ss {
				chair, err := db.FurnitureGetByTypeNumberRoom("chair", ss[i], rr[i])
				if err != nil {
					log.Println(err)
				}

				reservation, err := db.ReservationGet(chair.ID, o.EventID)
				if err != nil {
					log.Printf("error: ReservationOrderStatusHTML: can not get reservation for chair: %d from DB, eventID: %d, err: %v", chair.ID, o.EventID, err)
					plErr := map[string]string{
						"title": "Zamówienie wygasło!",
						"text":  "Zamówienie wygasło (obecnie wygasa po 5 minutach od kliknięcia na \"Zamów\"). Należy na nowo wybrać krzesła i ponowić zamówienie.",
					}
					ErrorHTML(plErr["title"], plErr["text"], lang, w, r)
					return
				}
				reservation.Status = "ordered"
				reservation.CustomerID = cID
				if noteID != 0 {
					reservation.NoteID = ToNI(noteID)
				}
				log.Printf("debug: noteID: %v", noteID)
				err = db.ReservationMod(&reservation)
				if err != nil {
					log.Printf("error modyfing reservation for chair: %d, eventID: %d, err: %v", chair.ID, o.EventID, err)
					plErr := map[string]string{
						"title": "Nie można zmienić stutusu zamówienia!",
						"text":  "Wystąpił problem ze zmianą stutusu zamówienia, przepraszamy za kłopot i prosimy o informację na\nmail: admin (at) zori.cz.\nProsimy o przesłanie info: email zamawiającego, numery zamawianych siedzień, nazwa sali/imprezy.",
					}
					ErrorHTML(plErr["title"], plErr["text"], lang, w, r)
					return
				}
			}

			adminMails, err := db.AdminGetEmails(user.ID)
			if err != nil {
				log.Println(err)
			}

			custMail := MailConfig{
				Server:     mailConf.Server,
				Port:       mailConf.Port,
				User:       mailConf.User,
				Pass:       mailConf.Pass,
				From:       user.Email,
				ReplyTo:    user.Email,
				Sender:     mailConf.Sender,
				To:         []string{o.Email},
				Subject:    event.MailSubject,
				Text:       ParseTmpl(event.MailText, o),
				IgnoreCert: mailConf.IgnoreCert,
			}

			err = MailSend(custMail)
			if err != nil {
				log.Println(err)
			}

			userMail := MailConfig{
				Server:     mailConf.Server,
				Port:       mailConf.Port,
				User:       mailConf.User,
				Pass:       mailConf.Pass,
				From:       mailConf.From,
				ReplyTo:    mailConf.From,
				Sender:     mailConf.From,
				To:         append(adminMails, user.Email),
				Subject:    ParseTmpl(event.AdminMailSubject, o),
				Text:       ParseTmpl(event.AdminMailText, o),
				IgnoreCert: mailConf.IgnoreCert,
			}
			err = MailSend(userMail)
			if err != nil {
				log.Println(err)
			}
		} else {
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}

		pEN := ReservationOrderStatusVars{
			LBLLang:       lang,
			LBLTitle:      "Order status",
			LBLStatus:     event.OrderedNoteTitle,
			LBLStatusText: event.OrderedNoteText,
			BTNOk:         "OK",
		}
		_ = pEN

		p := ReservationOrderStatusVars{
			LBLLang:       lang,
			LBLTitle:      "Zamówiono bilety!",
			LBLStatus:     event.OrderedNoteTitle,
			LBLStatusText: event.OrderedNoteText,
			BTNOk:         "OK",
		}

		t := template.Must(template.ParseFiles("tmpl/order-status.html", "tmpl/base.html"))
		err := t.ExecuteTemplate(w, "base", p)
		if err != nil {
			log.Print("Reservation template executing error: ", err)
		}
	}
}

func GetFurnituresFromDB(db *DB, roomName string, eventID int64) RoomVars {
	chairs, err := db.FurnitureFullGetChairs(eventID, roomName)
	if err != nil {
		log.Printf("error getting chairs(FurnitureFull) for room %q, err: %v", roomName, err)
	}
	tables, err := db.FurnitureGetAllByRoomNameOfType(roomName, "table")
	if err != nil {
		log.Printf("error getting 'tables' for room %q, err: %v", roomName, err)
	}
	objects, err := db.FurnitureGetAllByRoomNameOfType(roomName, "object")
	if err != nil {
		log.Printf("error getting 'objects' for room %q, err: %v", roomName, err)
	}
	labels, err := db.FurnitureGetAllByRoomNameOfType(roomName, "label")
	if err != nil {
		log.Printf("error getting 'labels' for room %q, err: %v", roomName, err)
	}
	return RoomVars{
		Tables:  tables,
		Chairs:  chairs,
		Objects: objects,
		Labels:  labels,
	}
}

// TODO: switch to room.ID in all db funcs
func GetPageVarsFromDB(db *DB, roomID, eventID int64) Page {
	room, err := db.RoomGetByID(roomID)
	if err != nil {
		log.Printf("error getting room by ID %d, err: %v", roomID, err)
	}
	event, err := db.EventGetByID(eventID)
	if err != nil {
		log.Printf("error getting event by ID: %d, err: %v", eventID, err)
	}
	chairs, err := db.FurnitureFullGetChairs(event.ID, room.Name)
	if err != nil {
		log.Printf("error getting chairs(FurnitureFull) for room %q, err: %v", room.Name, err)
	}
	tables, err := db.FurnitureGetAllByRoomNameOfType(room.Name, "table")
	if err != nil {
		log.Printf("error getting 'tables' for room %q, err: %v", room.Name, err)
	}
	objects, err := db.FurnitureGetAllByRoomNameOfType(room.Name, "object")
	if err != nil {
		log.Printf("error getting 'objects' for room %q, err: %v", room.Name, err)
	}
	labels, err := db.FurnitureGetAllByRoomNameOfType(room.Name, "label")
	if err != nil {
		log.Printf("error getting 'labels' for room %q, err: %v", room.Name, err)
	}
	return Page{
		Room:    room,
		Event:   event,
		Tables:  tables,
		Chairs:  chairs,
		Objects: objects,
		Labels:  labels,
	}
}

type ReservationOrderVars struct {
	Event                               Event
	LBLOrderHowto                       string
	LBLLang                             string
	LBLTitle                            string
	LBLCountdownDescription             string
	LBLEmail, LBLEmailPlaceholder       string
	LBLEmailHelp                        string
	LBLPassword, LBLPasswordPlaceholder string
	LBLPasswordHelp                     string
	LBLName, LBLNamePlaceholder         string
	LBLSurname, LBLSurnamePlaceholder   string
	LBLPhone, LBLPhonePlaceholder       string
	LBLPhoneHelp                        string
	LBLNotes, LBLNotesPlaceholder       string
	LBLNotesHelp                        string
	LBLPricesValue, LBLRoomsValue       string
	LBLSits, LBLSitsValue               string
	LBLUserURL                          string
	LBLTotalPrice, LBLTotalPriceValue   string
	BTNSubmit, BTNCancel                string
}

// TODO: this function is too long! split it!
func ReservationOrderHTML(db *DB, lang string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		eventID := int64(-1)
		sits := ""
		prices := ""
		rooms := ""
		totalPrice := ""
		defaultCurrency := ""

		if r.Method == "POST" {
			err := r.ParseForm()
			if err != nil {
				log.Printf("ReservationOrderStatusHTML: error parsing form data:, err: %v", err)
			}
			eventIDs := r.FormValue("event-id")
			eventID, err = strconv.ParseInt(eventIDs, 10, 64)
			if err != nil {
				log.Printf("error: ReservationOrderHTML: can not convert %q to int64, err: %v", eventIDs, err)
			}
			sits = r.FormValue("sits")
			prices = r.FormValue("prices")
			rooms = r.FormValue("rooms")
			totalPrice = r.FormValue("total-price")
			defaultCurrency = r.FormValue("default-currency")

			ss, rr, pp, err := SplitSitsRoomsPrices(sits, rooms, prices)
			if err != nil {
				log.Printf("ReservationOrderHTML: %v", err)
			}

			for i := range ss {
				chair, err := db.FurnitureGetByTypeNumberRoom("chair", ss[i], rr[i])
				if err != nil {
					log.Println(err)
				}
				_, err = db.ReservationAdd(&Reservation{
					OrderedDate: ToNI(time.Now().Unix()),
					Price:       ToNI(pp[i]),
					Currency:    ToNS(defaultCurrency),
					Status:      "marked", // this is very important
					FurnitureID: chair.ID,
					EventID:     eventID,
					CustomerID:  -1, // this will be updated when customer is created
				})
				if err != nil {
					log.Printf("error adding reservation for chair number: %d, roomID: %d, eventID: %d, err: %v", chair.Number, chair.RoomID, eventID, err)
					errPL := map[string]string{
						"title": "Nie udało się zarezerwować miejsca!",
						"text":  "Nie można zarezerwować wybranych miejsc, zostały one już zablokowane przez innego zamawiającego.\nProsimy o wybranie innych miejsc, lub poczekanie 5 minut. Po 5 minutach niezrealizowane zamówiania są automatycznie anulowane.",
					}
					ErrorHTML(errPL["title"], errPL["text"], lang, w, r)
					//http.Error(w, fmt.Sprintf("<html><body><b>Can not add reservation for chair: %d, eventID: %d, err: %v</b></body></html>", chair.ID, event.ID, err), 500)
					return
				}
			}
		} else {
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}

		event, err := db.EventGetByID(eventID)
		if err != nil {
			log.Printf("ReservationOrderHTML: error getting event by ID: %d, err: %v", eventID, err)
		}
		user, err := db.UserGetByID(event.UserID) //TODO: it is probably ok to do it like that?
		if err != nil {
			log.Println(err)
		}

		//p := GetPageVarsFromDB(db, roomName, eventName)
		pEN := ReservationOrderVars{
			Event:                 event,
			LBLOrderHowto:         event.OrderHowto,
			LBLLang:               lang,
			LBLTitle:              "Order",
			LBLEmail:              "Email",
			LBLEmailHelp:          "Email is also login",
			LBLEmailPlaceholder:   "email",
			LBLPassword:           "Password",
			LBLPasswordHelp:       "Optional password",
			LBLName:               "Name",
			LBLNamePlaceholder:    "name",
			LBLSurname:            "Surname",
			LBLSurnamePlaceholder: "surname",
			LBLPhone:              "Phone",
			LBLPhonePlaceholder:   "00420 ",
			LBLNotes:              "Notes",
			LBLNotesPlaceholder:   "Notes",
			LBLNotesHelp:          event.OrderNotesDescription,
			LBLPricesValue:        prices,
			LBLRoomsValue:         rooms,
			LBLSits:               "Sits",
			LBLSitsValue:          sits,
			LBLTotalPrice:         "Total price",
			LBLTotalPriceValue:    totalPrice + " " + defaultCurrency,
			BTNSubmit:             "Confirm order",
			BTNCancel:             "Cancel",
		}
		_ = pEN

		p := ReservationOrderVars{
			Event:                   event,
			LBLOrderHowto:           event.OrderHowto,
			LBLLang:                 lang,
			LBLTitle:                "Zamówienie",
			LBLCountdownDescription: "Sesja wygaśnie, kiedy skończy się odliczanie:",
			LBLEmail:                "Email",
			LBLEmailHelp:            "Na podany email zostanie wysłane potwierdzenie. Proszę sprawdzić, że podano poprawny!",
			LBLEmailPlaceholder:     "email",
			LBLPassword:             "Hasło",
			LBLPasswordHelp:         "Należy wymyślić dowolne hasło. Podanie hasła umożliwia wykonanie wielu rezerwacji używając tego samego konta mailowego a docelowo również zarządzianie zamówieniami.",
			LBLName:                 "Imię",
			LBLNamePlaceholder:      "imię",
			LBLSurname:              "Nazwisko",
			LBLSurnamePlaceholder:   "nazwisko",
			LBLPhone:                "Nr telefonu",
			LBLPhonePlaceholder:     "00420 ",
			LBLNotes:                "Notatki",
			LBLNotesPlaceholder:     "notatki",
			LBLNotesHelp:            event.OrderNotesDescription,
			LBLPricesValue:          prices,
			LBLRoomsValue:           rooms,
			LBLSits:                 "Numery krzeseł",
			LBLSitsValue:            sits,
			LBLTotalPrice:           "Łączna suma",
			LBLTotalPriceValue:      totalPrice + " " + defaultCurrency,
			LBLUserURL:              user.URL,
			BTNSubmit:               "Zamawiam",
			BTNCancel:               "Anuluj zamówienie",
		}

		t := template.Must(template.ParseFiles("tmpl/order.html", "tmpl/base.html"))
		err = t.ExecuteTemplate(w, "base", p)
		if err != nil {
			log.Print("Reservation template executing error: ", err)
		}
	}
}

type AdminLoginVars struct {
	LBLLang                string
	LBLTitle               string
	LBLEmail               string
	LBLEmailPlaceholder    string
	LBLEmailHelp           string
	LBLPassword            string
	LBLPasswordPlaceholder string
	LBLPasswordHelp        string
	LBLRememberMe          string
	LBLResetPassword       string
	BTNSubmit              string
}

func AdminLoginHTML(db *DB, lang string, cookieStore *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		pEN := AdminLoginVars{
			LBLLang:                lang,
			LBLTitle:               "Rezerwo admin login",
			LBLEmail:               "Email",
			LBLEmailHelp:           "Email is also login",
			LBLEmailPlaceholder:    "email",
			LBLPassword:            "Password",
			LBLPasswordHelp:        "Admin login password required",
			LBLPasswordPlaceholder: "password",
			LBLRememberMe:          "Remember me",
			LBLResetPassword:       "Reset forgotten password",
			BTNSubmit:              "Login",
		}

		t := template.Must(template.ParseFiles("tmpl/a_login.html", "tmpl/base.html"))
		err := t.ExecuteTemplate(w, "base", pEN)
		if err != nil {
			log.Print("AdminLogin template executing error: ", err)
		}

	}
}

type AdminPage struct {
	PageMeta
	Events     []Event
	Rooms      []Room
	Furnitures []Furniture
}

type AdminMainPageVars struct {
	LBLRoomEventTitle      string
	LBLRoomEventText       template.HTML
	BTNSelect              string
	BTNClose               string
	BTNAddRoom             string
	BTNAddEvent            string
	LBLSelectRoom          string
	BTNRoomEdit            string
	BTNRoomDelete          string
	LBLNewEventPlaceholder string
	LBLSelectEvent         string
	BTNEventEdit           string
	BTNEventDelete         string
	LBLMsgTitle            string
	LBLRaports             string
	BTNShowRaports         string
}

func AdminMainPage(db *DB, loc *time.Location, lang string, dateFormat string, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, email, err := InitSession(w, r, cs, "/admin/login", true)
		if err != nil {
			log.Printf("info: EventEditor: %v", err)
			return
		}

		user, err := db.UserGetByEmail(email)
		if err != nil {
			log.Printf("error: very strange, can not find admin in users table, but is authenticated, err: %v", err)
		}

		dtype := ""
		// if event/room detail form sent some data,
		// save them to db and show result
		if r.Method == "POST" {
			err := r.ParseForm()
			if err != nil {
				log.Printf("error parsing form data:, err: %v", err)
			}
			dtype = r.FormValue("type")
			if dtype == "event" {
				id, err := strconv.Atoi(r.FormValue("id"))
				if err != nil {
					log.Printf("problem converting %q to number, err: %v", r.FormValue("id"), err)
				}
				d, err := time.ParseInLocation(dateFormat, r.FormValue("date"), loc)
				if err != nil {
					log.Println(err)
				}

				fd, err := time.ParseInLocation(dateFormat, r.FormValue("from-date"), loc)
				if err != nil {
					log.Println(err)
				}
				td, err := time.ParseInLocation(dateFormat, r.FormValue("to-date"), loc)
				if err != nil {
					log.Println(err)
				}
				dp, err := strconv.Atoi(r.FormValue("default-price"))
				if err != nil {
					log.Println(err)
				}

				e := Event{
					ID: int64(id),
				}
				_ = e //TODO
				e.Name = r.FormValue("name")
				e.Date = d.Unix()
				e.FromDate = fd.Unix()
				e.ToDate = td.Unix()
				e.DefaultPrice = int64(dp)
				e.DefaultCurrency = r.FormValue("default-currency")
				e.MailSubject = r.FormValue("mail-subject")
				e.AdminMailSubject = r.FormValue("admin-mail-subject")
				e.AdminMailText = r.FormValue("admin-mail-text")
				e.HowTo = r.FormValue("html-howto")
				e.OrderedNoteTitle = r.FormValue("ordered-note-title")
				e.OrderedNoteText = r.FormValue("html-ordered-note-text")

				log.Printf("%+v", e)
				org, _ := db.EventGetByID(e.ID)
				log.Printf("org: %+v", org)
				log.Println("test equal, is:", reflect.DeepEqual(e, org))
			}
		} // POST END

		// Prepare form with rooms and events
		rooms, err := db.RoomGetAllByUserID(user.ID)
		if err != nil {
			log.Printf("error getting all rooms, err: %q", err)
		}
		events, err := db.EventGetAllByUserID(user.ID)
		if err != nil {
			log.Printf("error getting event by name: %q, err: %v", "TODO", err)
		}
		enPM := PageMeta{
			LBLLang:  lang,
			LBLTitle: "Admin main page",
			AdminMainPageVars: AdminMainPageVars{
				LBLRoomEventTitle:      "Select event",
				LBLRoomEventText:       template.HTML("<b>Why?</b><br />You need to select event for room, because chair <i>'disabled'</i> status and chair <i>'price'</i> are related to the <b>event</b>, not room itself. If You select different event next time, room will be the same, but 'disabled' and 'price' attributs may be different."),
				BTNSelect:              "Select",
				BTNClose:               "Close",
				BTNAddRoom:             "Add room",
				BTNAddEvent:            "Add event",
				BTNEventEdit:           "Edit",
				LBLNewEventPlaceholder: "New event name",
				LBLSelectRoom:          "Select room ...",
				BTNRoomEdit:            "Edit",
				BTNRoomDelete:          "Delete",
				LBLSelectEvent:         "Select event ...",
				BTNEventDelete:         "Delete",
				LBLMsgTitle:            dtype,
				LBLRaports:             "Raports",
				BTNShowRaports:         "Show raports",
			},
		}

		rp := AdminPage{
			PageMeta: enPM,
			Events:   events,
			Rooms:    rooms,
		}
		t := template.Must(template.ParseFiles("tmpl/a_main.html", "tmpl/base.html"))
		err = t.ExecuteTemplate(w, "base", rp)
		if err != nil {
			log.Print("AdminMainPage template executing error: ", err)
		}
	}
}

type EventEditorVars struct {
	LBLLang                                                  string
	LBLTitle                                                 string
	LBLID                                                    string
	LBLIDValue                                               int64
	LBLName, LBLNameValue                                    string
	NameHelpText                                             string
	LBLDate, LBLDateValue                                    string
	LBLFromDate, LBLFromDateValue                            string
	LBLToDate, LBLToDateValue                                string
	LBLDefaultPrice                                          string
	LBLDefaultPriceValue                                     int64
	LBLDefaultCurrency, LBLDefaultCurrencyValue              string
	LBLMailSubject, LBLMailSubjectValue                      string
	LBLMailText, LBLMailTextValue                            string
	LBLAdminMailSubject, LBLAdminMailSubjectValue            string
	LBLAdminMailText, LBLAdminMailTextValue                  string
	LBLOrderedNoteTitleValue                                 string
	LBLOrderedNoteTitle, LBLHowto, LBLOrderedNoteText        string
	HTMLOrderedNoteText, HTMLHowTo, HTMLOrderedNoteTextValue template.HTML
	BTNSave                                                  string
	BTNCancel                                                string
}

func EventEditor(db *DB, lang string, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			_, _, _, err := InitSession(w, r, cs, "/admin/login", true)

			if err != nil {
				log.Printf("info: EventEditor: %v", err)
				return
			}
			err = r.ParseForm()
			if err != nil {
				log.Printf("EventEditor: problem parsing form data, err: %v", err)
			}
			eventID, err := strconv.ParseInt(r.FormValue("event-id"), 10, 64)
			if err != nil {
				log.Printf("error converting eventID to int, err: %v", err)
			}
			event, err := db.EventGetByID(eventID)
			if err != nil {
				log.Printf("error retrieving event with ID: %q from DB, err: %v", eventID, err)
			}

			rp := EventEditorVars{
				LBLLang:                  lang,
				LBLTitle:                 "Event details",
				LBLID:                    "ID",
				LBLIDValue:               event.ID,
				LBLName:                  "Name",
				LBLNameValue:             event.Name,
				NameHelpText:             "Name help text",
				LBLDate:                  "Date",
				LBLDateValue:             ToDate(event.Date),
				LBLFromDate:              "Reservation starts",
				LBLFromDateValue:         ToDate(event.FromDate),
				LBLToDate:                "Reservation ends",
				LBLToDateValue:           ToDate(event.ToDate),
				LBLDefaultPrice:          "Default chair price",
				LBLDefaultPriceValue:     event.DefaultPrice,
				LBLDefaultCurrency:       "Default currency",
				LBLDefaultCurrencyValue:  event.DefaultCurrency,
				LBLMailSubject:           "Customer mail subject",
				LBLMailSubjectValue:      event.MailSubject,
				LBLMailText:              "Customer mail text",
				LBLMailTextValue:         event.MailText,
				LBLAdminMailSubject:      "Admin mail subject",
				LBLAdminMailSubjectValue: event.AdminMailSubject,
				LBLAdminMailText:         "Admin mail text",
				LBLAdminMailTextValue:    event.AdminMailText,
				BTNSave:                  "Save",
				BTNCancel:                "Cancel",
				LBLHowto:                 "Howto room legend",
				HTMLHowTo:                template.HTML(event.HowTo),
				LBLOrderedNoteTitle:      "After ordered note title",
				LBLOrderedNoteTitleValue: event.OrderedNoteTitle,
				LBLOrderedNoteText:       "After ordered note text",
				HTMLOrderedNoteTextValue: template.HTML(event.OrderedNoteText),
			}
			t := template.Must(template.ParseFiles("tmpl/a_event.html", "tmpl/base.html"))
			err = t.ExecuteTemplate(w, "base", rp)
			if err != nil {
				log.Print("AdminEventEditor template executing error: ", err)
			}
		}
	}
}

type AdminReservationsVars struct {
	LBLLang          string
	EventID          int64
	LBLTitle         string
	LBLTotalPrice    string
	LBLTotalSits     string
	THChairNumber    string
	THRoomName       string
	THCustName       string
	THCustSurname    string
	THCustEmail      string
	THCustPhone      string
	THPrice          string
	THCurrency       string
	THStatus         string
	THNotes          string
	THOrderedDate    string
	THPayedDate      string
	ReservationsFull []ReservationFull
}

func AdminReservations(db *DB, lang string, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		eventID := int64(-1)
		if r.Method == "POST" {
			err := r.ParseForm()
			if err != nil {
				log.Printf("error: AdminReservations: problem parsing form, err: %v", err)
			}
			eventIDs := r.FormValue("event-id")
			eventID, err = strconv.ParseInt(eventIDs, 10, 64)
			if err != nil {
				log.Printf("error: AdminReservations: can not convert %q to int64, err: %v", eventIDs, err)
			}

		} else {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
		}
		_, _, email, err := InitSession(w, r, cs, "/admin/login", true)
		if err != nil {
			log.Printf("info: AdminReservations: %v", err)
			return
		}
		_ = email
		user, err := db.UserGetByEmail(email)
		if err != nil {
			log.Printf("error: AdminReservations: can't get user %q by mail, err: %v", email, err)
		}

		rf, err := db.ReservationFullGetAll(user.ID, eventID)
		if err != nil {
			log.Printf("error: AdminReservations: can not get reservations for event ID: %d and user ID: %d, err: %v", eventID, user.ID, err)
		}

		enP := AdminReservationsVars{
			LBLLang:          lang,
			EventID:          eventID,
			LBLTitle:         "Reservations",
			LBLTotalPrice:    "Total",
			LBLTotalSits:     "Sits",
			THChairNumber:    "Chair nr",
			THRoomName:       "Room",
			THCustName:       "Name",
			THCustSurname:    "Surname",
			THCustEmail:      "Email",
			THCustPhone:      "Phone",
			THPrice:          "Price",
			THCurrency:       "Currency",
			THStatus:         "Order status",
			THNotes:          "Notes",
			THOrderedDate:    "Ordered",
			THPayedDate:      "Payed",
			ReservationsFull: rf,
		}

		t := template.Must(template.ParseFiles("tmpl/a_reservations.html", "tmpl/base.html"))
		err = t.ExecuteTemplate(w, "base", enP)
		if err != nil {
			log.Print("AdminReservations template executing error: ", err)
		}

	}
}

type AboutVars struct {
	LBLLang                     string
	LBLTitle                    string
	LBLAboutTitle, LBLAboutText template.HTML
}

func AboutHTML(lang string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		pPL := AboutVars{
			LBLLang:       lang,
			LBLTitle:      "REZERWO",
			LBLAboutTitle: template.HTML("REZERWO - Zaolziański system do rezerwacji"),
			// 	LBLAboutText: template.HTML(`
			// <b>Sezon 2022/2023</b><br />
			// Wszyscy wymawiają się na covid, więc i ja skorzystam. System jak widzicie żyje i ma się dobrze. Covid spowodował,
			// że mieliśmy trzyletnią pauzę i bali po prostu nie było. <br />
			// W tym roku dodałem kilka zmian w tle a z widoczych to wyświetlanie czasu do zakończenia sesji podczas zamawiania biletów.<br />
			// Na razie nie promuję systemu wśród innych kół Macierzy i PZKO, od strony administracji jest jeszcze kilka rzeczy do zrobienia by doświadczenia zarządzających były na prawdę bezproblemowe.<br />
			// <br />
			// <b>Uaktualniony opis projektu:</b><br />
			// Rezerwo to projekt utworzony z myślą o polskich organizacjach w Czechach. Ponieważ projekt to "one man show"
			// pisany w wolnym czasie, mamy dobrze działający proces zamawiania, tworzenia pomieszczeń i całkiem dobre raporty, tylko administracja imprez wymaga jeszcze trochę pracy. Poza tym pomysłów na ulepszenia jest dużo ...<br />
			// <b>Głównym celem projektu jest, by wszystkie bale (i podobne), których organizatorem są polskie organizacje w Czechach,
			// były zarządzane za pomocą REZERWO.</b><br />
			// Leszek Cimała, admin (at) zori.cz`),
			LBLAboutText: template.HTML(`<b>Skorzystali Państwo z zaolziańskiego systemu rezerwacji miejsc.<br />
			Dziękujemy i zapraszamy ponownie!</b><br />
			Leszek Cimała, admin (at) zori.cz`),
		}

		t := template.Must(template.ParseFiles("tmpl/about.html", "tmpl/base.html"))
		err := t.ExecuteTemplate(w, "base", pPL)
		if err != nil {
			log.Print("ErrorHTML: template executing error: ", err) //log it
		}
	}

}

type ErrorVars struct {
	LBLLang       string
	LBLTitle      string
	LBLAlertTitle string
	LBLAlertText  string
	BTNBack       string
}

func ErrorHTML(errorTitle, errorText, lang string, w http.ResponseWriter, r *http.Request) {
	errEN := ErrorVars{
		LBLLang:       lang,
		LBLTitle:      "Error",
		LBLAlertTitle: errorTitle,
		LBLAlertText:  errorText,
		BTNBack:       "OK",
	}
	_ = errEN
	errPL := ErrorVars{
		LBLLang:       lang,
		LBLTitle:      "Błąd",
		LBLAlertTitle: errorTitle,
		LBLAlertText:  errorText,
		BTNBack:       "OK",
	}

	t := template.Must(template.ParseFiles("tmpl/error.html", "tmpl/base.html"))
	err := t.ExecuteTemplate(w, "base", errPL)
	if err != nil {
		log.Print("ErrorHTML: template executing error: ", err) //log it
	}

}

type RoomMsg struct {
	RoomID int64 `json:"room_id"`
	Width  int64 `json:"width"`
	Height int64 `json:"height"`
}

func DesignerSetRoomSize(db *DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var m RoomMsg
		if r.Method == "POST" {
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&m)
			if err != nil {
				log.Println(err)
			}
			room := Room{
				ID:     m.RoomID,
				Height: m.Height,
				Width:  m.Width,
			}
			//log.Printf("write to db: %+v\n", room)
			err = db.RoomModSizeByID(&room)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

type MoveMsg struct {
	EventID     int64  `json:"event_id"`
	RoomID      int64  `json:"room_id"`
	Number      int64  `json:"name"`
	Type        string `json:"type"`
	Orientation string `json:"orientation"`
	Disabled    bool   `json:"disabled"`
	X           int64  `json:"x"`
	Y           int64  `json:"y"`
	Width       int64  `json:"width"`
	Height      int64  `json:"height"`
	Color       string `json:"color"`
	Price       int64  `json:"price"`
	Currency    string `json:"currency"`
	Capacity    int64  `json:"capacity"`
	Label       string `json:"label"`
}

func DesignerMoveObject(db *DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var m MoveMsg
		if r.Method == "POST" {
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&m)
			if err != nil {
				log.Println(err)
			}
			fmt.Printf("%+v", m)
			f := Furniture{
				Number:      m.Number,
				X:           m.X,
				Y:           m.Y,
				Type:        m.Type,
				Orientation: ToNS(m.Orientation),
				Width:       ToNI(m.Width),
				Height:      ToNI(m.Height),
				Color:       ToNS(m.Color),
				Label:       ToNS(m.Label),
				Capacity:    ToNI(m.Capacity),
				RoomID:      m.RoomID,
			}

			_, err = db.FurnitureAdd(&f)
			if err != nil {
				log.Printf("info: inserting furniture failed, trying to update, err: %v", err)
				err := db.FurnitureModByNumberTypeRoom(&f)
				if err != nil {
					log.Printf("error: furniture insert failed, now also update failed, f: %+v, err: %v", f, err)
				}
			}

			furn, err := db.FurnitureGetByTypeNumberRoom(m.Type, m.Number, m.RoomID)
			if err != nil {
				log.Printf("error: furniture get failed %+v, err:%v", f, err)
			}
			//log.Printf("write to db: %+v\n", f)

			dis := int64(0)
			if m.Disabled {
				dis = 1
			}

			p := Price{
				Price:       m.Price,
				Currency:    m.Currency,
				Disabled:    dis,
				EventID:     m.EventID,
				FurnitureID: furn.ID,
			}
			_, err = db.PriceAdd(&p)
			if err != nil {
				log.Printf("info: price inserting failed, trying update, err: %v", err)
				err = db.PriceModByEventIDFurnID(&p)
				if err != nil {
					log.Printf("error: price insert failed, now also update failed, p: %+v, err: %v", p, err)
				}

			}
		}
	}
}

type DeleteMsg struct {
	EventID int64  `json:"event_id"`
	RoomID  int64  `json:"room_id"`
	Number  int64  `json:"name"`
	Type    string `json:"type"`
}

func DesignerDeleteObject(db *DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var m DeleteMsg
		if r.Method == "POST" {
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&m)
			if err != nil {
				log.Println(err)
			}
			log.Printf("deleting from db: %+v\n", m)
			err = db.FurnitureDelByNumberTypeRoom(m.Number, m.Type, m.RoomID)
			if err != nil {
				log.Println(err)
			}
			db.PriceDelByEventIDFurn(m.EventID, m.Number, m.Type)
		}
	}
}

type RenumberMsg struct {
	RoomID int64  `json:"room_id"`
	Type   string `json:"type"`
}

func DesignerRenumberType(db *DB) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		var m RenumberMsg
		if r.Method == "POST" {
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&m)
			if err != nil {
				log.Println(err)
			}
			log.Printf("renumbering: %+v\n", m)
			ff, err := db.FurnitureGetAllByRoomIDOfType(m.RoomID, m.Type)
			if err != nil {
				log.Printf("error retrieving %q for room %v, err: %s", m.Type, m.RoomID, err)
			}
			fmt.Printf("%+v", ff)
			ff, err = FurnitureRenumber(ff)
			if err != nil {
				log.Println(err)
			}
			log.Println("after")
			fmt.Printf("%+v", ff)
			for i := range ff {
				err := db.FurnitureMod(&ff[i])
				if err != nil {
					log.Println(err)
				}
			}
		}
	}
}

type OrderCancelMsg struct {
	Sits    string `json:"sits"`
	Rooms   string `json:"rooms"`
	EventID int64  `json:"event-id"`
}

func OrderCancel(db *DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var m OrderCancelMsg
		if r.Method == "POST" {
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&m)
			log.Printf("ordercancel: %+v", m)
			if err != nil {
				log.Println(err)
			}
			ss, rr, err := SplitSitsRooms(m.Sits, m.Rooms)
			if err != nil {
				log.Printf("OrderCancel: %v", err)
			}

			for i := range ss {
				chair, err := db.FurnitureGetByTypeNumberRoom("chair", ss[i], rr[i])
				if err != nil {
					log.Println(err)
				}

				reservation, err := db.ReservationGet(chair.ID, m.EventID)
				if err != nil {
					log.Printf("OrderCancel: error retrieving reservation for chair: %d, eventID: %d, err: %v", chair.ID, m.EventID, err)
				}
				err = db.ReservationDel(reservation.ID)
				if err != nil {
					log.Printf("OrderCancel: error deleting reservation to %q status, err: %v", "free", err)
				}
			}
		}
	}
}

type LoginAPIMsg struct {
	Role     string `json:"role"`
	Email    string `json:"email"`
	Pass     string `json:"password"`
	Remember string `json:"remember-me"`
}

func LoginAPI(db *DB, cookieStore *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		var m LoginAPIMsg
		if r.Method == "POST" {
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&m)
			if err != nil {
				log.Println(err)
			}

			// now authentication
			// generate hash on register: hashedPassword, err := bcrypt.GenerateFromPassword([]byte(m.Pass), 6)
			if m.Role == "admin" {
				storedPasswd, err := db.UserGetPass(m.Email)
				if err != nil {
					log.Println(err)
				}
				hashedPassword, err := bcrypt.GenerateFromPassword([]byte(storedPasswd), 6)

				if err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(m.Pass)); err != nil {
					log.Println("passwd do not match")
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
			}

			w.Header().Set("Content-Type", "application/json")
			// respJSON, err := json.Marshal(m) //if we want to send message back
			if err != nil {
				w.Write([]byte(err.Error()))
			}
			if m.Remember == "on" {
				// modify cookie max age
				cookieStore.Options = &sessions.Options{
					Path:     "/",
					MaxAge:   86400 * 14, //2 weeks
					HttpOnly: true,
				}
			}
			session, err := cookieStore.Get(r, AUTHCOOKIE)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			session.Values["email"] = m.Email
			session.Values["role"] = m.Role
			log.Println(m.Email, ", role: ", m.Role)
			session.Save(r, w)
			w.Write([]byte(`{"resp":"OK"}`)) //TODO: do we really need to send something to trigger js: success func?
		}
	}
}

// TODO: add support for multiple active events
func EventGetCurrent(db *DB, userID int64) (Event, error) {
	events, err := db.EventGetAllByUserID(userID)
	if err != nil {
		return Event{}, err
	}
	now := time.Now().Unix()
	for i := range events {
		// return first active user event
		if events[i].FromDate < now && events[i].ToDate > now {
			return events[i], nil
		}
	}
	return Event{}, fmt.Errorf("no active event found for userID: %d", userID)
}

func PasswdReset(db *DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {}
}

type ReservationChangeStatusMsg struct {
	EventID    int64  `json:"event_id"`
	FurnNumber int64  `json:"furn_number"`
	RoomName   string `json:"room_name"`
	Status     string `json:"status"`
}

func ReservationChangeStatusAPI(db *DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var m ReservationChangeStatusMsg
		if r.Method == "POST" {
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&m)
			if err != nil {
				log.Printf("error: ReservationChangeStatusAPI: problem decoding json, err: %v", err)
			}
			fmt.Printf("%+v", m)
			room, err := db.RoomGetByName(m.RoomName)
			if err != nil {
				log.Printf("error: ReservationChangeStatusAPI: problem retrieving room, name: %s, err: %v", m.RoomName, err)
			}

			chair, err := db.FurnitureGetByTypeNumberRoom("chair", m.FurnNumber, room.ID)
			if err != nil {
				log.Printf("error: ReservationChangeStatusAPI: problem retrieving furniture, number: %d, roomID: %d, err: %v", m.FurnNumber, room.ID, err)
			}
			timestamp := int64(0)
			if m.Status == "payed" {
				timestamp = time.Now().Unix()
			}

			if err := db.ReservationModStatus(m.Status, timestamp, m.EventID, chair.ID); err != nil {
				log.Printf("error: ReservationChangeStatusAPI: problem updating status to %q for FurnID: %d, EventID: %d, err: %v", m.Status, chair.ID, m.EventID, err)
			}
		}
	}
}

type ReservationDeleteMsg struct {
	EventID    int64  `json:"event_id"`
	FurnNumber int64  `json:"furn_number"`
	RoomName   string `json:"room_name"`
}

func ReservationDeleteAPI(db *DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var m ReservationDeleteMsg
		if r.Method == "DELETE" {
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&m)
			if err != nil {
				log.Printf("error: ReservationDeleteAPI: problem decoding json, err: %v", err)
			}
			fmt.Printf("%+v", m)
			room, err := db.RoomGetByName(m.RoomName)
			if err != nil {
				log.Printf("error: ReservationDeleteAPI: problem retrieving room, name: %s, err: %v", m.RoomName, err)
			}

			chair, err := db.FurnitureGetByTypeNumberRoom("chair", m.FurnNumber, room.ID)
			if err != nil {
				log.Printf("error: ReservationDeleteAPI: problem retrieving furniture, number: %d, roomID: %d, err: %v", m.FurnNumber, room.ID, err)
			}

			reservation, err := db.ReservationGet(chair.ID, m.EventID)
			if err != nil {
				log.Printf("error: ReservationDeleteAPI: error retrieving reservation for chair: %d, eventID: %d, err: %v", chair.ID, m.EventID, err)
			}
			log.Println("debug: reservationID:", reservation.ID)
			err = db.ReservationDel(reservation.ID)
			if err != nil {
				log.Printf("error: ReservationDeleteAPI: error deleting reservation, err: %v", err)
			}
			if reservation.NoteID.Valid {
				log.Printf("debug: is valid!")
				rr, err := db.ReservationGetAllByNoteID(reservation.NoteID.Int64)
				if err != nil {
					log.Printf("error: ReservationDeleteAPI: error retrieving reservations by NoteID: %d, err: %v", reservation.NoteID.Int64, err)
				}
				log.Println(rr)
				if len(rr) == 0 {
					err = db.NoteDel(reservation.NoteID.Int64)
					if err != nil {
						log.Printf("error: ReservationDeleteAPI: problem deleting note with ID: %d, err: %v", reservation.NoteID.Int64, err)
					}

				} else {
					log.Printf("debug: ReservationDeleteAPI: not deleting note, %d related reservation remains", len(rr))
				}
			}

		}
	}
}

type Order struct {
	EventID             int64
	TotalPrice          string
	Sits, Prices, Rooms string
	Email               string
	Password            string
	Name, Surname       string
	Phone, Notes        string
}

func ParseTmpl(t string, o Order) string {
	var buf bytes.Buffer
	tmpl, err := template.New("test").Parse(t)
	if err != nil {
		log.Printf("error parsing template %q, order %+v, err: %v", t, o, err)
		return t
	}
	err = tmpl.Execute(&buf, o)
	if err != nil {
		log.Printf("error executing template %q, order %+v, err: %v", t, o, err)
		return t
	}
	return buf.String()
}

func ToNS(s string) sql.NullString {
	if s == "" {
		return sql.NullString{
			Valid: false,
		}
	}
	return sql.NullString{
		String: s,
		Valid:  true,
	}
}

func ToNI(i int64) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{
			Valid: false,
		}
	}
	return sql.NullInt64{
		Int64: i,
		Valid: true,
	}
}

func ToDate(unix int64) string {
	t := time.Unix(unix, 0)
	return t.Format("2006-01-02")
}

func InitSession(w http.ResponseWriter, r *http.Request, cookieStore *sessions.CookieStore, onErrRedir string, requiredAdmin bool) (*sessions.Session, string, string, error) {
	var role, email string

	session, err := cookieStore.Get(r, AUTHCOOKIE)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return &sessions.Session{}, "", "", err
	}

	emailIntf, eok := session.Values["email"]
	roleIntf, rok := session.Values["role"]
	switch emailIntf.(type) {
	case nil:
		// cookie has disapeared, so re-auth
		http.Redirect(w, r, onErrRedir, http.StatusSeeOther)
		return session, "", "", fmt.Errorf("nil \"email\" cookie disapeared?, redirecting to %s", onErrRedir)
	case string:
		email = emailIntf.(string)

	}
	switch roleIntf.(type) {
	case nil:
		// cookie has disapeared, so re-auth
		http.Redirect(w, r, onErrRedir, http.StatusSeeOther)
		return session, "", "", fmt.Errorf("nil \"role\" cookie disapeared?, redirecting to %s", onErrRedir)
	case string:
		role = roleIntf.(string)
	}

	if !eok && !rok {
		http.Redirect(w, r, onErrRedir, http.StatusSeeOther)
		return session, "", "", fmt.Errorf("\"role\" or \"email\" are empty string, redirecting to %s", onErrRedir)
	}

	if requiredAdmin {
		if err := mustBeAdmin(w, r, role, email, onErrRedir); err != nil {
			return session, "", "", err
		}
	}

	return session, role, email, nil
}

func mustBeAdmin(w http.ResponseWriter, r *http.Request, role, email, onErrRedir string) error {
	if role != "admin" {
		http.Redirect(w, r, onErrRedir, http.StatusSeeOther)
		return fmt.Errorf("user %q is not admin!, redirecting to %s", email, onErrRedir)
	}
	return nil
}

func SplitSitsRoomsPrices(sits, rooms, prices string) ([]int64, []int64, []int64, error) {
	var ssi, rri, ppi []int64
	ss := strings.Split(sits, ",")
	rr := strings.Split(rooms, ",")
	pp := strings.Split(prices, ",")

	if len(ss) != len(pp) || len(ss) != len(rr) {
		return ssi, rri, ppi, fmt.Errorf("error sits/rooms/prices POST - wrong lenght, ss: %q, pp: %q, rr: %q",
			sits, prices, rooms)
	}
	for i := range ss {
		sit, err := strconv.ParseInt(strings.TrimSpace(ss[i]), 10, 64)
		if err != nil {
			return ssi, rri, ppi, fmt.Errorf("SplitSitsRoomsPrices: error converting sit ID %q to int64, err: %v", ss[i], err)
		}
		ssi = append(ssi, sit)

		room, err := strconv.ParseInt(strings.TrimSpace(rr[i]), 10, 64)
		if err != nil {
			return ssi, rri, ppi, fmt.Errorf("SplitSitsRoomsPrices: error converting room ID %q to int64, err: %v", rr[i], err)
		}
		rri = append(rri, room)

		price, err := strconv.ParseInt(strings.TrimSpace(pp[i]), 10, 64)
		if err != nil {
			return ssi, rri, ppi, fmt.Errorf("SplitSitsRoomsPrices: error converting price %q to int64, err: %v", pp[i], err)
		}
		ppi = append(ppi, price)

	}
	return ssi, rri, ppi, nil
}

func SplitSitsRooms(sits, rooms string) ([]int64, []int64, error) {
	var ssi, rri []int64
	ss := strings.Split(sits, ",")
	rr := strings.Split(rooms, ",")

	if len(ss) != len(rr) {
		return ssi, rri, fmt.Errorf("error sits/rooms POST - wrong lenght, ss: %q, rr: %q",
			sits, rooms)
	}
	for i := range ss {
		sit, err := strconv.ParseInt(strings.TrimSpace(ss[i]), 10, 64)
		if err != nil {
			return ssi, rri, fmt.Errorf("SplitSitsRoomsPrices: error converting sit ID %q to int64, err: %v", ss[i], err)
		}
		ssi = append(ssi, sit)

		room, err := strconv.ParseInt(strings.TrimSpace(rr[i]), 10, 64)
		if err != nil {
			return ssi, rri, fmt.Errorf("SplitSitsRoomsPrices: error converting room ID %q to int64, err: %v", rr[i], err)
		}
		rri = append(rri, room)
	}

	return ssi, rri, nil
}

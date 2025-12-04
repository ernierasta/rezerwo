package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/crypto/bcrypt"
)

const (
	AUTHCOOKIE        = "auth"
	MEDIAROOT         = "media"
	MEDIAEMAILSUBDIR  = "email"
	MEDIAQRCODESUBDIR = "qrcode"
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

	lang := "pl" // cs

	mailConf := &MailConfig{Server: conf.MailServer, Port: int(conf.MailPort), From: conf.MailFrom, User: conf.MailUser, Pass: conf.MailPass, IgnoreCert: conf.MailIgnoreCert, Hostname: conf.MailHostname}

	db := initDB()
	defer db.Close()

	stop := make(chan bool)
	Ticker5min(db, stop)
	defer TickerStop(stop)

	rtr := mux.NewRouter()
	rtr.HandleFunc("/res/{user}", ReservationHTML(db, lang))
	rtr.HandleFunc("/form/{userurl}/{formurl}", FormRenderer(db, lang))
	rtr.HandleFunc("/form/{userurl}/{formurl}/done", FormThankYou(db, lang))
	rtr.HandleFunc("/", AboutHTML(lang))

	handleStatic("js")
	handleStatic("css")
	handleStatic("img")
	handleStatic("media") //user data
	http.Handle("/", rtr)
	http.HandleFunc("/order", ReservationOrderHTML(db, lang))
	http.HandleFunc("/order/status", ReservationOrderStatusHTML(db, lang, mailConf))
	http.HandleFunc("/admin/login", AdminLoginHTML(db, lang, cookieStore))
	http.HandleFunc("/admin", AdminMainPage(db, loc, lang, dateFormat, cookieStore))
	http.HandleFunc("/admin/designer", DesignerHTML(db, lang))
	http.HandleFunc("/admin/event", EventEditor(db, lang, cookieStore))
	http.HandleFunc("/admin/reservations", AdminReservations(db, lang, cookieStore))
	http.HandleFunc("/admin/formeditor", FormEditor(db, lang, cookieStore))
	http.HandleFunc("/admin/formraport", FormRaport(db, lang, cookieStore))
	http.HandleFunc("/admin/bankacceditor", BankAccountEditor(db, lang, cookieStore))
	http.HandleFunc("/admin/maileditor", MailEditor(db, lang, cookieStore))
	http.HandleFunc("/passreset", PasswdReset(db))
	http.HandleFunc("/api/login", LoginAPI(db, cookieStore))
	http.HandleFunc("/api/room", DesignerSetRoomSize(db))
	http.HandleFunc("/api/furnit", DesignerMoveObject(db))
	http.HandleFunc("/api/furdel", DesignerDeleteObject(db))
	http.HandleFunc("/api/ordercancel", OrderCancel(db))
	http.HandleFunc("/api/renumber", DesignerRenumberType(db))
	http.HandleFunc("/api/resstatus", ReservationChangeStatusAPI(db))
	http.HandleFunc("/api/resdelete", ReservationDeleteAPI(db))
	http.HandleFunc("/api/formstatus", FormChangeStatusAPI(db))
	http.HandleFunc("/api/eved", EventAddMod(db, loc, dateFormat, cookieStore))
	http.HandleFunc("/api/formed", FormTemplateAddMod(db, cookieStore))
	http.HandleFunc("/api/baed", BankAccountAddMod(db, cookieStore))
	http.HandleFunc("/api/formans", FormAddMod(db, mailConf))
	http.HandleFunc("/api/formansdelete", FormAnsDelete(db, cookieStore))
	http.HandleFunc("/api/formanssendmail", FormAnsSendMail(db, mailConf, cookieStore))
	http.HandleFunc("/api/maed", MailAddMod(db, cookieStore))
	http.HandleFunc("/api/formstmpls", FormTemplsGetAPI(db, cookieStore))
	http.HandleFunc("/api/formdefs", GenerateFormDefsAPI(db, cookieStore))

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
	_ = mailText
	mailTextP := `<html>
Szanowni Państwo,<br />
dziękujemy za dokonanie rezerwacji biletów na Bal Macierzy Szkolnej przy PSP w Karwinie-Frysztacie.<br />
Niniejszym mailem potwierdzamy zamówienie miejsc: {{.Sits}}.<br />
Łączna kwota biletów wynosi: <b>{{.TotalPrice}}</b>.<br />
Bal rozpocznie się w piątek <b>26 stycznia 2023</b> o godzinie 19:00 w Domu Przyjaźni w Karwinie.<br />
<br />
Uwaga! Dokonali Państwo tylko rezerwacji biletów.<br />
<br />
Kupić bilety można we <b>wtorek 12.12.2023 od 15:30 - 16:30</b>. Główne wejście do szkoły. <br />
 <br />
Cena biletu wynosi 500 Kč. Płatność tylko i wyłącznie gotówką!<br />
 <br />
Dziękujemy serdecznie!<br />
<br />
Zarząd MSz przy PSP w Karwinie-Frysztacie<br />
<br />
W przypadku pytań lub wątpliwości zapraszam do kontaktu pod adresem karwina@macierz.cz.<br />
</html>`
	_ = mailTextP

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

	//adminMailSubject := "Rezerwacja: {{.Name}} {{.Surname}}, {{.TotalPrice}}"
	//adminMailText := "{{.Name}} {{.Surname}}\nkrzesła: {{.Sits}} sale: {{.Rooms}}\nŁączna cena: {{.TotalPrice}}\nEmail: {{.Email}}\nTel: {{.Phone}}\nNotatki:{{.Notes}}"

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
	eID, err := db.EventAdd(&Event{ID: 1, Name: "Bal MS Karwina", Date: 1581033600, FromDate: 1572998400, ToDate: 1580860800, DefaultPrice: 400, DefaultCurrency: "Kč", NoSitsSelectedTitle: noSitsSelTitle, NoSitsSelectedText: noSitsSelText, OrderHowto: orderHowto, OrderNotesDescription: "Prosimy o podanie nazwisk wszystkich rodzin, dla których przeznaczone są bilety.", OrderedNoteTitle: orderedNoteTitle, OrderedNoteText: orderedNoteText, HowTo: howto, UserID: 1})
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
	eIDP, err := db.EventAdd(&Event{ID: 2, Name: "Bal polski", Date: 1581033600, FromDate: 1572998400, ToDate: 1580860800, DefaultPrice: 400, DefaultCurrency: "Kč", NoSitsSelectedTitle: noSitsSelTitle, NoSitsSelectedText: noSitsSelText, OrderHowto: orderHowto, OrderNotesDescription: "Prosimy o podanie nazwisk wszystkich rodzin, dla których przeznaczone są bilety.", OrderedNoteTitle: orderedNoteTitle, OrderedNoteText: orderedNoteText, HowTo: howtoP, UserID: uIDP})
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
		lng := lang
		v := mux.Vars(r)
		user, err := db.UserGetByURL(v["user"])

		plErr := map[string]string{
			"title": "Nie znaleziono organizacji!",
			"text":  fmt.Sprintf("W bazie nie istnieje organizacja: %q\nProszę sprawdzić poprawność linka.\nJeżeli organizator twierdzi, że jest ok, to proszę o kontakt pod: admin (at) zori.cz.", v["user"]),
		}

		if err != nil {
			log.Printf("error getting user %q, err: %v", v["user"], err)

			ErrorHTML(plErr["title"], plErr["text"], lng, w, r)
			//http.Error(w, fmt.Sprintf("User %q not found! Check You have correct URL!", v["user"]), 500)
			return
		}
		event, err := EventGetCurrent(db, user.ID)
		if err != nil {
			log.Printf("error getting current event for userID: %d, err: %v", user.ID, err)
			ErrorHTML("Nie ma obecnie aktywnych imprez!", "Administrator nie ma obecnie żadnych otwartych imprez.\nProsimy o skontaktowanie się z organizatorem by stwierdzić, kiedy rezerwacje zostaną otwarte.", lng, w, r)
			//http.Error(w, "User have no active events! Come back later, when reservations will be opened!", 500) //TODO: inform about closest user event and when it is
			return
		}

		if event.Language.Valid {
			lng = event.Language.String
		}

		rr, err := db.EventGetRooms(event.ID)
		if err != nil {
			log.Printf("error getting rooms for eventID: %d, err: %v", event.ID, err)
			plErr := map[string]string{
				"title": "Brak aktywnych sal dla tej imprezy!",
				"text":  "Administrator nie powiązał żadnej sali z wydarzeniem.\nJeżeli po stronie administracji wszystko wygląda ok, to prosimy o informację o tym zdarzeniu na mail: admin (at) zori.cz.\nProsimy o wysłanie nazwy organizacji, której dotyczy problem.",
			}
			ErrorHTML(plErr["title"], plErr["text"], lng, w, r)
			//http.Error(w, fmt.Sprintf("Rooms for user: %q, event: %q not found!", v["user"], e.Name), 500)
			return
		}
		title := "Reservation"
		switch lng {
		case "pl":
			title = "Rezerwacja"
		case "cs":
			title = "Rezervace"
		}

		p := ReservationPageVars{
			//EN: LBLTitle: "Reservation",
			LBLTitle: title,
			//EN: LBLNoSitsTitle: "No sits selected",
			//EN: LBLNoSitsText: "No sits selected, choose some free chairs and try it again",
			//EN: BTNNoSitsOK: "OK",
			LBLLang:        lng,
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
			// rv.HTMLRoomDescription = template.HTML(rr[i].Description.String) // Changed to event.RoomXDescription
			switch i {
			case 0:
				rv.HTMLRoomDescription = template.HTML(event.Room1Desc.String)
			case 1:
				rv.HTMLRoomDescription = template.HTML(event.Room2Desc.String)
			case 2:
				rv.HTMLRoomDescription = template.HTML(event.Room3Desc.String)
			case 3:
				rv.HTMLRoomDescription = template.HTML(event.Room4Desc.String)
			}
			if rv.HTMLRoomDescription == "" { // failback to room description if empty in event table
				rv.HTMLRoomDescription = template.HTML(rr[i].Description.String)
			}

			rv.HTMLHowTo = template.HTML(event.HowTo)
			switch lng {
			case "pl":
				rv.LBLSelected = "Wybrano"
				rv.LBLTotalPrice = "Łączna suma"
				rv.BTNOrder = "Zamów"
			case "cs":
				rv.LBLSelected = "Vybrané"
				rv.LBLTotalPrice = "Cena celkem"
				rv.BTNOrder = "Objednat"
			case "en":
				rv.LBLSelected = "Selected"
				rv.LBLTotalPrice = "Total price"
				rv.BTNOrder = "Order"
			}

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
	LBLLang       string
	LBLTitle      string
	LBLStatus     string
	HiddenFormID  string
	HiddenSurname string
	HiddenName    string
	LBLStatusText template.HTML
	BTNOk         string
}

// TODO: split it, too long!
func ReservationOrderStatusHTML(db *DB, lang string, mailConf *MailConfig) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		lng := lang
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

			l, err := db.EventGetLang(o.EventID)
			if err != nil {
				log.Printf("error: ReservationOrderStatusHTML: can not get event language for event %q, err: %v", o.EventID, err)
			}
			if l.Valid {
				lng = l.String
			}

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
				Err := map[string]string{}
				switch lng {
				case "pl":
					Err = map[string]string{
						"title": "Nie można zapisać zamówienia!",
						"text":  "Wystąpił błąd, nie można zapisać danych zamawiającego, tym samym również zamówienia. Kontakt: admin (at) zori.cz.",
					}
				case "cs":
					Err = map[string]string{
						"title": "Nepovedlo se uložit objednávku!",
						"text":  "Chyba na serveru neumožnůje uložení Vaších údajů. Tím pádem nejde uložit objednávku. Kontaktujte: admin (at) zori.cz.",
					}
				case "en":
					Err = map[string]string{

						"title": "Error saving order",
						"text":  "Can not save customer data. Contact me: admin (at) zori.cz.",
					}

				}
				ErrorHTML(Err["title"], Err["text"], lng, w, r)
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
						ErrorHTML(plErr["title"], plErr["text"], lng, w, r)
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
			cstMail, err := db.NotificationGetByID(event.ThankYouNotificationsID.Int64, user.ID)
			if err != nil {
				log.Printf("ReservationOrderStatusHTML: problem getting cust notification by ID: %q, err: %v", event.ThankYouNotificationsID.Int64, err)
			}

			adminMail, err := db.NotificationGetByID(event.AdminNotificationsID.Int64, user.ID)
			if err != nil {
				log.Printf("ReservationOrderStatusHTML: problem getting admin notification by ID: %q, err: %v", event.AdminNotificationsID.Int64, err)
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
					Err := map[string]string{}
					switch lng {
					case "pl":
						Err = map[string]string{
							"title": "Zamówienie wygasło!",
							"text":  "Zamówienie wygasło (wygasa po 5 minutach od kliknięcia na \"Zamów\"). Należy na nowo wybrać krzesła i ponowić zamówienie.",
						}
					case "cs":
						Err = map[string]string{
							"title": "Čas na objednání vypršel!",
							"text":  "Čas na odeslání vypršel (po 5 minutách). Vyberte židle znova a objednejte ještě jednou.",
						}
					case "en":
						Err = map[string]string{
							"title": "Order time ended.",
							"text":  "Order timed out. Please select chairs again and make new order.",
						}

					}

					ErrorHTML(Err["title"], Err["text"], lng, w, r)
					return
				}
				reservation.Status = "ordered"
				reservation.CustomerID = cID
				if noteID != 0 {
					reservation.NoteID = ToNI(noteID)
				}
				err = db.ReservationMod(&reservation)
				if err != nil {
					log.Printf("error modyfing reservation for chair: %d, eventID: %d, err: %v", chair.ID, o.EventID, err)

					Err := map[string]string{}
					switch lng {
					case "pl":
						Err = map[string]string{
							"title": "Nie można zmienić stutusu zamówienia!",
							"text":  "Wystąpił problem ze zmianą stutusu zamówienia, przepraszamy za kłopot i prosimy o informację na\nmail: admin (at) zori.cz.\nProsimy o przesłanie info: email zamawiającego, numery zamawianych siedzień, nazwa sali/imprezy.",
						}
					case "cs":
						Err = map[string]string{
							"title": "Nepovedlo se změnit stav objednávky!",
							"text":  "Nepovedlo se změnit stav objednávky, prosíme o informaci na\nmail: admin (at) zori.cz.\n Pošlete Váš e-mail, čísla objednaných míst, název místnosti/akce.",
						}
					case "en":
						Err = map[string]string{
							"title": "Error changing order status.",
							"text":  "Order status can not be changed. Contact: admin (at) zori.cz",
						}

					}

					ErrorHTML(Err["title"], Err["text"], lng, w, r)
					return
				}
			}

			adminMails, err := db.AdminGetEmails(user.ID) // this is empty table, no mails currently there
			if err != nil {
				log.Println(err)
			}

			// TODO: we are adding <html> tag, so mails are recognized as HTML, we are also modying default <p> tag margin
			// Shouldn't we do it in some more logical place? Or even make it editable by user?

			parsed, embimgs := ParseOrderTmpl(cstMail.Text, o, db, user)

			custMail := MailConfig{
				Server:          mailConf.Server,
				Port:            mailConf.Port,
				User:            mailConf.User,
				Pass:            mailConf.Pass,
				From:            chooseEmail(user.Email, user.AltEmail.String), // choose user (organizators) mails. Use primary email if alt_email is NULL. Otherwise use alt_email.
				ReplyTo:         user.Email,                                    // we wont they reply to organizators mail
				Sender:          mailConf.Sender,
				To:              []string{o.Email},
				Subject:         cstMail.Title.String,
				Text:            "<html><head><style>p {margin:0;}</style></head>" + parsed + "</html>",
				IgnoreCert:      mailConf.IgnoreCert,
				Hostname:        mailConf.Hostname,
				Files:           getAttachments(cstMail.AttachedFilesDelimited.String, user.URL, MEDIAEMAILSUBDIR, MEDIAROOT),
				EmbededHTMLImgs: getEmbeddedImgs(cstMail.EmbeddedImgsDelimited.String, user.URL, MEDIAEMAILSUBDIR, MEDIAROOT),
			}

			// append qrcode to EmbeddedImgs
			for i := range embimgs {
				custMail.EmbededHTMLImgs = append(custMail.EmbededHTMLImgs, embimgs[i])
			}
			err = MailSend(custMail)
			if err != nil {
				log.Println(err)
			}

			subj, _ := ParseOrderTmpl(adminMail.Title.String, o, db, user) // ignoring EmbImg, no qr code here
			text, _ := ParseOrderTmpl(adminMail.Text, o, db, user)
			userMail := MailConfig{
				Server:     mailConf.Server,
				Port:       mailConf.Port,
				User:       mailConf.User,
				Pass:       mailConf.Pass,
				From:       mailConf.From,
				ReplyTo:    mailConf.From,
				Sender:     mailConf.From,
				To:         append(adminMails, user.Email),
				Subject:    subj,
				Text:       "<html><head><style>p {margin:0;}</style></head>" + text + "</html>",
				IgnoreCert: mailConf.IgnoreCert,
				Hostname:   mailConf.Hostname,
			}
			err = MailSend(userMail)
			if err != nil {
				log.Println(err)
			}
		} else {
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}

		// now parse "OrderedNoteText" template to show qrcode
		// after posting form
		// we ignore embimg - it will be empty
		stsText, _ := ParseOrderTmpl(event.OrderedNoteText, o, db, user)

		pEN := ReservationOrderStatusVars{
			LBLLang:       lng,
			LBLTitle:      "Order status",
			LBLStatus:     event.OrderedNoteTitle,
			LBLStatusText: template.HTML(stsText),
			BTNOk:         "OK",
		}
		_ = pEN

		title := ""
		switch lng {
		case "pl":
			title = "Zamówiono bilety!"
		case "cs":
			title = "Objednáno lístky!"
		case "en":
			title = "Order status"
		}

		p := ReservationOrderStatusVars{
			LBLLang:       lng,
			LBLTitle:      title,
			LBLStatus:     event.OrderedNoteTitle,
			LBLStatusText: template.HTML(stsText),
			BTNOk:         "OK",
		}

		t := template.Must(template.ParseFiles("tmpl/order-status.html", "tmpl/a_base.html"))
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
	LBLOrderHowtoHTML                   template.HTML
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
		lng := lang
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

			l, err := db.EventGetLang(eventID)
			if err != nil {
				log.Printf("ReservationOrderHTML: cant get lang, event %q, %v", eventID, err)
			}
			if l.Valid {
				lng = l.String
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
					Err := map[string]string{}
					switch lng {
					case "pl":
						Err = map[string]string{
							"title": "Nie udało się zarezerwować miejsca!",
							"text":  "Nie można zarezerwować wybranych miejsc, zostały one już zablokowane przez innego zamawiającego.\nProsimy o wybranie innych miejsc, lub poczekanie 5 minut. Po 5 minutach niezrealizowane zamówiania są automatycznie anulowane.",
						}
					case "cs":
						Err = map[string]string{
							"title": "Nepovedlo se rezervovat židli/-e!",
							"text":  "Nepovedlo se rezervovat židle, jsou už bloknute jiným uživatelem.\nVyberte jiné místa, nebo počkejte 5 minut. Pokud uživatel objednávku nedokončí budou spět dostupné.",
						}
					case "en":
						Err = map[string]string{
							"title": "Sits can't be ordered.",
							"text":  "Sits can't be ordered, they are blocked by another user. Wait 5 minutes or choose other sits.",
						}

					}

					ErrorHTML(Err["title"], Err["text"], lng, w, r)
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

		p := ReservationOrderVars{
			Event:                 event,
			LBLOrderHowtoHTML:     template.HTML(event.OrderHowto),
			LBLLang:               lng,
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

		switch lng {
		case "pl":
			p = ReservationOrderVars{
				Event:                   event,
				LBLOrderHowtoHTML:       template.HTML(event.OrderHowto),
				LBLLang:                 lng,
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
		case "cs":
			p = ReservationOrderVars{
				Event:                   event,
				LBLOrderHowtoHTML:       template.HTML(event.OrderHowto),
				LBLLang:                 lng,
				LBLTitle:                "Objednávka",
				LBLCountdownDescription: "Objednávka bude zrušená za:",
				LBLEmail:                "Email",
				LBLEmailHelp:            "Na uvedený e-mail odešleme potvrzení. Ujistěte se, že správný.",
				LBLEmailPlaceholder:     "e-mail",
				LBLPassword:             "Heslo",
				LBLPasswordHelp:         "",
				LBLName:                 "Jméno",
				LBLNamePlaceholder:      "jméno",
				LBLSurname:              "Příjmení",
				LBLSurnamePlaceholder:   "příjmení",
				LBLPhone:                "Číslo telefonu",
				LBLPhonePlaceholder:     "00420 ",
				LBLNotes:                "Poznámky",
				LBLNotesPlaceholder:     "poznámky",
				LBLNotesHelp:            event.OrderNotesDescription,
				LBLPricesValue:          prices,
				LBLRoomsValue:           rooms,
				LBLSits:                 "Čísla míst",
				LBLSitsValue:            sits,
				LBLTotalPrice:           "Cena celkem",
				LBLTotalPriceValue:      totalPrice + " " + defaultCurrency,
				LBLUserURL:              user.URL,
				BTNSubmit:               "Objednat",
				BTNCancel:               "Zrušit objednávku",
			}
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
	Events        []Event
	Rooms         []Room
	Furnitures    []Furniture
	FormTempls    []FormTemplate
	BankAccounts  []BankAccount
	Notifications []Notification
}

type AdminMainPageVars struct {
	TabEvents                     string
	TabForms                      string
	TabSettings                   string
	LBLEvents                     string
	LBLRoomEventTitle             string
	LBLRoomEventText              template.HTML
	BTNSelect                     string
	BTNClose                      string
	BTNAddRoom                    string
	BTNAddEvent                   string
	LBLSelectRoom                 string
	BTNRoomEdit                   string
	BTNRoomDelete                 string
	LBLNewEventPlaceholder        string
	LBLSelectEvent                string
	BTNEventEdit                  string
	BTNEventDelete                string
	LBLMsgTitle                   string
	LBLRaports                    string
	BTNShowRaports                string
	LBLForms                      string
	BTNAddForm                    string
	LBLNewForm                    string
	LBLNewFormPlaceholder         string
	LBLNewFormURL                 string
	LBLSelectForm                 string
	LBLNewFormURLPlaceholder      string
	BTNEditForm                   string
	LBLSelectFormRaport           string
	BTNShowFormRaports            string
	LBLBankAccountsTitle          string
	LBLNewBAPlaceholder           string
	LBLSelectBankAccount          string
	BTNAddNewBA                   string
	BTNBAEdit                     string
	BTNBADelete                   string
	LBLNotificationsTitle         string
	LBLNewNotificationPlaceholder string
	LBLSelectNotification         string
	BTNAddNewNotification         string
	BTNNotificationEdit           string
	BTNNotificationDelete         string
	LBLNotificationIsShared       string
	UserID                        int64
	MyNotif                       string
	SharedNotif                   string
	NotifRelatedToEvent           string
	NotifRelatedToForm            string
	NotifRelatedToEventCode       string
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
		//if r.Method == "POST" {
		//	// TODO: remove this section!
		//	// I had remade it, so post goes to /eved api url
		//	err := r.ParseForm()
		//	if err != nil {
		//		log.Printf("error parsing form data:, err: %v", err)
		//	}
		//	dtype = r.FormValue("type")
		//	if dtype == "event" {
		//		id, err := strconv.Atoi(r.FormValue("id"))
		//		if err != nil {
		//			log.Printf("problem converting %q to number, err: %v", r.FormValue("id"), err)
		//		}
		//		d, err := time.ParseInLocation(dateFormat, r.FormValue("date"), loc)
		//		if err != nil {
		//			log.Println(err)
		//		}

		//		fd, err := time.ParseInLocation(dateFormat, r.FormValue("from-date"), loc)
		//		if err != nil {
		//			log.Println(err)
		//		}
		//		td, err := time.ParseInLocation(dateFormat, r.FormValue("to-date"), loc)
		//		if err != nil {
		//			log.Println(err)
		//		}
		//		dp, err := strconv.Atoi(r.FormValue("default-price"))
		//		if err != nil {
		//			log.Println(err)
		//		}

		//		e := Event{
		//			ID: int64(id),
		//		}
		//		_ = e //TODO
		//		e.Name = r.FormValue("name")
		//		e.Date = d.Unix()
		//		e.FromDate = fd.Unix()
		//		e.ToDate = td.Unix()
		//		e.DefaultPrice = int64(dp)
		//		e.DefaultCurrency = r.FormValue("default-currency")
		//		e.MailSubject = r.FormValue("mail-subject")
		//		e.AdminMailSubject = r.FormValue("admin-mail-subject")
		//		e.AdminMailText = r.FormValue("admin-mail-text")
		//		e.HowTo = r.FormValue("html-howto")
		//		e.OrderedNoteTitle = r.FormValue("ordered-note-title")
		//		e.OrderedNoteText = r.FormValue("html-ordered-note-text")

		//		log.Printf("%+v", e)
		//		org, _ := db.EventGetByID(e.ID)
		//		log.Printf("org: %+v", org)
		//		log.Println("test equal, is:", reflect.DeepEqual(e, org))
		//	}
		//} // POST END

		// Prepare form with rooms and events
		rooms, err := db.RoomGetAllByUserID(user.ID)
		if err != nil {
			log.Printf("AdminMainPage: error getting all rooms, err: %q", err)
		}
		events, err := db.EventGetAllByUserID(user.ID)
		if err != nil {
			log.Printf("AdminMainPage: error getting all events by userID: %d, err: %v", user.ID, err)
		}
		formTempls, err := db.FormTemplateGetAll(user.ID)
		if err != nil {
			log.Printf("AdminMainPage: error getting all forms for user %d, %v", user.ID, err)
		}
		bankAccs, err := db.BankAccountGetAll(user.ID)
		if err != nil {
			log.Printf("AdminMainPage: error getting all bankaccounts for user %d, %v", user.ID, err)
		}

		notifs, err := db.NotificationGetAllUsersAndSharable(user.ID)
		if err != nil {
			log.Printf("AdminMainPage: error getting all notifications for user %d, %v", user.ID, err)
		}

		enPM := PageMeta{
			LBLLang:  lang,
			LBLTitle: "Admin main page",
			AdminMainPageVars: AdminMainPageVars{
				TabEvents:                     "Events",
				TabForms:                      "Forms",
				TabSettings:                   "Settings",
				LBLEvents:                     "Events",
				LBLRoomEventTitle:             "Select event",
				LBLRoomEventText:              template.HTML("<b>Why?</b><br />You need to select event for room, because chair <i>'disabled'</i> status and chair <i>'price'</i> are related to the <b>event</b>, not room itself. If You select different event next time, room will be the same, but 'disabled' and 'price' attributs may be different."),
				BTNSelect:                     "Select",
				BTNClose:                      "Close",
				BTNAddRoom:                    "Add room",
				BTNAddEvent:                   "Add event",
				BTNEventEdit:                  "Edit",
				LBLNewEventPlaceholder:        "New event name",
				LBLSelectRoom:                 "Select room ...",
				BTNRoomEdit:                   "Edit",
				BTNRoomDelete:                 "Delete",
				LBLSelectEvent:                "Select event ...",
				BTNEventDelete:                "Delete",
				LBLMsgTitle:                   dtype,
				LBLRaports:                    "Raports",
				BTNShowRaports:                "Show raports",
				LBLForms:                      "Forms",
				LBLNewForm:                    "New form",
				BTNAddForm:                    "New form",
				LBLNewFormPlaceholder:         "Enter unique form name",
				LBLSelectForm:                 "Select form ...",
				LBLNewFormURL:                 "Form URL",
				LBLNewFormURLPlaceholder:      "Enter unique name - used as part of form link",
				BTNEditForm:                   "Edit form",
				LBLSelectFormRaport:           "Select form raport ...",
				BTNShowFormRaports:            "Show",
				LBLBankAccountsTitle:          "Bank Accounts (QRPay)",
				LBLNewBAPlaceholder:           "New account name",
				BTNAddNewBA:                   "Add new",
				LBLSelectBankAccount:          "Choose account ...",
				BTNBAEdit:                     "Edit",
				BTNBADelete:                   "Delete",
				LBLNotificationsTitle:         "Notifications",
				LBLNewNotificationPlaceholder: "New notification name",
				LBLSelectNotification:         "Choose notification ...",
				BTNAddNewNotification:         "Add new",
				BTNNotificationEdit:           "Edit",
				BTNNotificationDelete:         "Delete",
				LBLNotificationIsShared:       "shared",
				UserID:                        user.ID,
				MyNotif:                       "my",
				SharedNotif:                   "*",
				NotifRelatedToEvent:           "rezerw",
				NotifRelatedToForm:            "form",
				NotifRelatedToEventCode:       "events",
			},
		}

		_ = enPM

		plPM := PageMeta{
			LBLLang:  lang,
			LBLTitle: "Administracja",
			AdminMainPageVars: AdminMainPageVars{
				TabEvents:                     "Rezerwacje",
				TabForms:                      "Formularze",
				TabSettings:                   "Ustawienia/Katalogi",
				LBLEvents:                     "Imprezy",
				LBLRoomEventTitle:             "Wybierz imprezę",
				LBLRoomEventText:              template.HTML("<b>Dlaczego?</b><br />Musisz wybrać imprezę, ponieważ status krzeseł <i>'wyłączony'</i> oraz <i>'cena'</i> miejsca są związanie z <b>imprezą</b>, a nie z pomieszczeniem jako takim."),
				BTNSelect:                     "Wybierz",
				BTNClose:                      "Zamknij",
				BTNAddRoom:                    "Dodaj pomieszczenie",
				BTNAddEvent:                   "Dodaj imprezę",
				BTNEventEdit:                  "Zmień",
				LBLNewEventPlaceholder:        "Nazwa nowej imprezy",
				LBLSelectRoom:                 "Wybierz pomieszczenie ...",
				BTNRoomEdit:                   "Zmień",
				BTNRoomDelete:                 "Usuń",
				LBLSelectEvent:                "Wybierz imprezę ...",
				BTNEventDelete:                "Usuń",
				LBLMsgTitle:                   dtype,
				LBLRaports:                    "Raporty",
				BTNShowRaports:                "Wyświetl raporty",
				LBLForms:                      "Formularze",
				LBLNewForm:                    "Nowy formularz",
				BTNAddForm:                    "Nowy formularz",
				LBLNewFormPlaceholder:         "Unikatowa nazwa",
				LBLSelectForm:                 "Wybierz formularz ...",
				LBLNewFormURL:                 "URL formularza",
				LBLNewFormURLPlaceholder:      "Odnośnik (bez PL znaków/spacji)",
				BTNEditForm:                   "Edytuj formularz",
				LBLSelectFormRaport:           "Wybierz raport do formularza ...",
				BTNShowFormRaports:            "Wyświetl",
				LBLBankAccountsTitle:          "Konta bankowe (QRPay)",
				LBLNewBAPlaceholder:           "Nazwa nowego konta",
				BTNAddNewBA:                   "Dodaj nowe",
				LBLSelectBankAccount:          "Wybierz konto ...",
				BTNBAEdit:                     "Edytuj",
				BTNBADelete:                   "Usuń",
				LBLNotificationsTitle:         "Maile (notyfikacje)",
				LBLNewNotificationPlaceholder: "Nazwa nowej notyfikacji",
				LBLSelectNotification:         "Wybierz notyfikację ...",
				BTNAddNewNotification:         "Dodaj",
				BTNNotificationEdit:           "Edytuj",
				BTNNotificationDelete:         "Usuń",
				LBLNotificationIsShared:       "Notyfikacje oznaczone (*) są notyfikacjami współdzielonymi.",
				UserID:                        user.ID,
				MyNotif:                       "moja",
				SharedNotif:                   "*",
				NotifRelatedToEvent:           "rezerw",
				NotifRelatedToForm:            "form",
				NotifRelatedToEventCode:       "events",
			},
		}

		rp := AdminPage{
			PageMeta:      plPM,
			Events:        events,
			Rooms:         rooms,
			FormTempls:    formTempls,
			BankAccounts:  bankAccs,
			Notifications: notifs,
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
	IDVal, UserIDVal                                         int64
	LBLName, LBLNameValue                                    string
	NameHelpText                                             string
	LBLLanguage                                              string
	LanguageValue                                            string
	LanguageHelpText                                         string
	LBLDate, LBLDateValue                                    string
	LBLFromDate, LBLFromDateValue                            string
	LBLToDate, LBLToDateValue                                string
	LBLDefaultPrice                                          string
	LBLDefaultPriceValue                                     int64
	LBLDefaultCurrency, LBLDefaultCurrencyValue              string
	LBLSelectThankYouMail, LBLSelectMailHint                 string
	LBLSelectAdminMail, LBLSelectAdminMailHint               string
	CurrentThankYouMail, CurrentAdminMail                    int64
	LBLOrderedNoteTitleValue                                 string
	LBLOrderedNoteTitle, LBLHowto, LBLOrderedNoteText        string
	HTMLOrderedNoteText, HTMLHowTo, HTMLOrderedNoteTextValue template.HTML
	BTNSave                                                  string
	BTNCancel                                                string
	Notifications                                            []Notification
	BTNNotificationEdit                                      string
	LBLNoSitsSelectedTitle, LBLNoSitsSelectedText            string
	HTMLNoSitsSelectedTextValue                              template.HTML
	NoSitsSelectedTitleValue                                 string
	LBLOrderHowTo                                            string
	HTMLOrderHowToValue                                      template.HTML
	LBLOrderDescription                                      string
	OrderDescriptionValue                                    string
	LBLSharable                                              string
	IsSharableVal                                            bool
	LBLRoomsSelect                                           string
	LBLSelectRoomHint                                        string
	Rooms                                                    []Room
	BTNRoomAdd                                               string
	LBLRooms                                                 string
	RoomsVal                                                 string
	RoomsHelpText                                            string
	BTNClearRooms                                            string
	LBLCurrentRooms                                          string
	CurrentBankAccount                                       int64
	BTNBAEdit                                                string
	BTNBADelete                                              string
	LBLSelectBankAccount                                     string
	BankAccounts                                             []BankAccount
	LBLTitleDates                                            string
	LBLTitlePrice                                            string
	LBLTitleMails                                            string
	LBLTitleNoSitsSelected                                   string
	LBLTitleHowToOrder                                       string
	LBLTitleOrdered                                          string
	LBLTitleRoomLegend                                       string
	LBLTitleSharable                                         string
	LBLRoomDescSection                                       string
	LBLRoom1Desc                                             string
	Room1DescValue                                           template.HTML
	LBLRoom1Banner                                           string
	Room1BannerValue                                         string
	LBLRoom2Desc                                             string
	Room2DescValue                                           template.HTML
	LBLRoom2Banner                                           string
	Room2BannerValue                                         string
	LBLRoom3Desc                                             string
	Room3DescValue                                           template.HTML
	LBLRoom3Banner                                           string
	Room3BannerValue                                         string
	LBLRoom4Desc                                             string
	Room4DescValue                                           template.HTML
	LBLRoom4Banner                                           string
	Room4BannerValue                                         string
}

func EventEditor(db *DB, lang string, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			_, _, email, err := InitSession(w, r, cs, "/admin/login", true)

			if err != nil {
				log.Printf("info: EventEditor: %v", err)
				return
			}

			user, err := db.UserGetByEmail(email)
			if err != nil {
				log.Printf("error: AdminReservations: can't get user %q by mail, err: %v", email, err)
			}

			err = r.ParseForm()
			if err != nil {
				log.Printf("EventEditor: problem parsing form data, err: %v", err)
			}

			var event Event
			var srooms = ""

			eventID, err := strconv.ParseInt(r.FormValue("event-id"), 10, 64)
			if err != nil {
				name := r.FormValue("name")
				event = Event{
					Name:   name,
					UserID: user.ID,
				}
				if name == "" {
					log.Printf("EventEditor: error retrieving event, no valid ID %q, nor name %q", eventID, name)
				}
			} else {
				event, err = db.EventGetByID(eventID)
				if err != nil {
					log.Printf("EventEditor: error retrieving event with ID: %q from DB, err: %v", eventID, err)
				}
				// set polish as default if empty in database
				if !event.Language.Valid {
					event.Language = sql.NullString{String: "pl", Valid: true}
				}
				evrooms, err := db.EventGetRooms(event.ID)
				if err != nil {
					log.Printf("EventEditor: error getting rooms for event %d, %v", event.ID, err)
				}
				// get string with ID's separated by comma
				srooms = getRoomsString(evrooms)
			}

			notifs, err := db.NotificationGetAllRelatedToEventsForUser(user.ID)
			if err != nil {
				log.Printf("EventEditor: error getting notifications for user %d, %v", user.ID, err)
			}

			userrooms, err := db.RoomGetAllByUserID(user.ID)
			if err != nil {
				log.Printf("EventEditor: error getting rooms for user %d, %v", user.ID, err)
			}

			bankAccs, err := db.BankAccountGetAll(user.ID)
			if err != nil {
				log.Printf("EventEditor: error getting all bankaccounts for user %d, %v", user.ID, err)
			}

			rpEN := EventEditorVars{
				LBLLang:                 lang,
				LBLTitle:                "Event details",
				IDVal:                   event.ID,
				UserIDVal:               event.UserID,
				LBLName:                 "Name",
				LBLNameValue:            event.Name,
				NameHelpText:            "Name help text",
				LBLLanguage:             "Language",
				LanguageValue:           event.Language.String,
				LanguageHelpText:        "Decides about some displayed texts language (pl, cs, en)",
				LBLDate:                 "Date",
				LBLDateValue:            ToDate(event.Date),
				LBLFromDate:             "Reservation starts",
				LBLFromDateValue:        ToDate(event.FromDate),
				LBLToDate:               "Reservation ends",
				LBLToDateValue:          ToDate(event.ToDate),
				LBLDefaultPrice:         "Default chair price",
				LBLDefaultPriceValue:    event.DefaultPrice,
				LBLDefaultCurrency:      "Default currency",
				LBLDefaultCurrencyValue: event.DefaultCurrency,
				//LBLMailSubject:              "Customer mail subject",
				//LBLMailSubjectValue:         event.MailSubject,
				//LBLMailText:                 "Customer mail text",
				//LBLMailTextValue:            event.MailText,
				//LBLAdminMailSubject:         "Admin mail subject",
				//LBLAdminMailSubjectValue:    event.AdminMailSubject,
				//LBLAdminMailText:            "Admin mail text",
				//LBLAdminMailTextValue:       event.AdminMailText,
				BTNNotificationEdit:         "Edit",
				BTNSave:                     "Save",
				BTNCancel:                   "Cancel",
				LBLHowto:                    "Howto room legend",
				HTMLHowTo:                   template.HTML(event.HowTo),
				LBLOrderedNoteTitle:         "After ordered note title",
				LBLOrderedNoteTitleValue:    event.OrderedNoteTitle,
				LBLOrderedNoteText:          "After ordered note text",
				HTMLOrderedNoteTextValue:    template.HTML(event.OrderedNoteText),
				Notifications:               notifs,
				LBLSelectBankAccount:        "Select bank account (for QR code) ...",
				BankAccounts:                bankAccs,
				CurrentBankAccount:          event.BankAccountsID.Int64,
				BTNBAEdit:                   "Edit",
				BTNBADelete:                 "Delete",
				CurrentThankYouMail:         event.ThankYouNotificationsID.Int64,
				CurrentAdminMail:            event.AdminNotificationsID.Int64,
				LBLSelectThankYouMail:       "After order (cust.)",
				LBLSelectAdminMail:          "After order (admin)",
				LBLSelectMailHint:           "Select customer mail",
				LBLSelectAdminMailHint:      "Select admin mail",
				LBLNoSitsSelectedTitle:      "No sits selected title",
				NoSitsSelectedTitleValue:    event.NoSitsSelectedTitle,
				LBLNoSitsSelectedText:       "No sits select text",
				HTMLNoSitsSelectedTextValue: template.HTML(event.NoSitsSelectedText),
				LBLOrderHowTo:               "How to order",
				HTMLOrderHowToValue:         template.HTML(event.OrderHowto),
				LBLOrderDescription:         "Order description text",
				OrderDescriptionValue:       event.OrderNotesDescription,
				IsSharableVal:               event.Sharable.Bool,
				LBLSharable:                 "Shared",
				LBLRoomsSelect:              "Add room to this event",
				LBLSelectRoomHint:           "Here are all rooms ID's related to this event",
				Rooms:                       userrooms,
				BTNRoomAdd:                  "Assign",
				LBLRooms:                    "Rooms",
				RoomsVal:                    srooms,
				RoomsHelpText:               "Assign one or more rooms to event",
				BTNClearRooms:               "Unassign all rooms",
				LBLCurrentRooms:             "Currently assigned rooms",
				LBLTitleDates:               "When?",
				LBLTitlePrice:               "How much?",
				LBLTitleMails:               "E-mails",
				LBLTitleOrdered:             "After order - thank you",
				LBLTitleHowToOrder:          "How to order",
				LBLTitleNoSitsSelected:      "When no sits selected",
				LBLTitleRoomLegend:          "Room legend, price, ...",
				LBLTitleSharable:            "Shared",
				LBLRoomDescSection:          "Event/Room descreption",
				Room1DescValue:              template.HTML(event.Room1Desc.String),
				LBLRoom1Desc:                "Room1 Description",
				Room2DescValue:              template.HTML(event.Room2Desc.String),
				LBLRoom2Desc:                "Room2 Description",
				Room3DescValue:              template.HTML(event.Room3Desc.String),
				LBLRoom3Desc:                "Room3 Description",
				Room4DescValue:              template.HTML(event.Room4Desc.String),
				LBLRoom4Desc:                "Room4 Description",
				Room1BannerValue:            event.Room1Banner.String,
				LBLRoom1Banner:              "Room1 Banner",
				Room2BannerValue:            event.Room2Banner.String,
				LBLRoom2Banner:              "Room2 Banner",
				Room3BannerValue:            event.Room3Banner.String,
				LBLRoom3Banner:              "Room3 Banner",
				Room4BannerValue:            event.Room4Banner.String,
				LBLRoom4Banner:              "Room4 Banner",
			}

			rpPL := EventEditorVars{
				LBLLang:                 lang,
				LBLTitle:                "Szczegóły imprezy",
				IDVal:                   event.ID,
				UserIDVal:               event.UserID,
				LBLName:                 "Nazwa",
				LBLNameValue:            event.Name,
				NameHelpText:            "Nazwa powinna być unikatowa, np. Bal 2033",
				LBLLanguage:             "Język",
				LanguageValue:           event.Language.String,
				LanguageHelpText:        "Decyduje o języku niektórych tekstów (wartości: pl, cs, en, ...)",
				LBLDate:                 "Data",
				LBLDateValue:            ToDate(event.Date),
				LBLFromDate:             "Początek rezerwacji",
				LBLFromDateValue:        ToDate(event.FromDate),
				LBLToDate:               "Koniec rezerwacji",
				LBLToDateValue:          ToDate(event.ToDate),
				LBLDefaultPrice:         "Cena za miejsce",
				LBLDefaultPriceValue:    event.DefaultPrice,
				LBLDefaultCurrency:      "Waluta (Kč, Zł, ...)",
				LBLDefaultCurrencyValue: event.DefaultCurrency,
				//LBLMailSubject:              "Tytuł maila do zamawiającego",
				//LBLMailSubjectValue:         event.MailSubject,
				//LBLMailText:                 "Treść maila do zamawiającego",
				//LBLMailTextValue:            event.MailText,
				//LBLAdminMailSubject:         "Temat maila do administratora",
				//LBLAdminMailSubjectValue:    event.AdminMailSubject,
				//LBLAdminMailText:            "Treść maila do administratora",
				//LBLAdminMailTextValue:       event.AdminMailText,
				BTNNotificationEdit:         "Edytuj",
				BTNSave:                     "Zapisz",
				BTNCancel:                   "Anuluj",
				LBLHowto:                    "Legenda pomieszczenia - podaj cenę biletu, program",
				HTMLHowTo:                   template.HTML(event.HowTo),
				LBLOrderedNoteTitle:         "Tytuł wyświetlany po zamówieniu",
				LBLOrderedNoteTitleValue:    event.OrderedNoteTitle,
				LBLOrderedNoteText:          "Tekst wyświetlany po zamówieniu",
				HTMLOrderedNoteTextValue:    template.HTML(event.OrderedNoteText),
				LBLSelectBankAccount:        "Wybierz konto bankowe (dla QR kodu) ...",
				BankAccounts:                bankAccs,
				CurrentBankAccount:          event.BankAccountsID.Int64,
				BTNBAEdit:                   "Edytuj",
				BTNBADelete:                 "Kasuj",
				Notifications:               notifs,
				CurrentThankYouMail:         event.ThankYouNotificationsID.Int64,
				CurrentAdminMail:            event.AdminNotificationsID.Int64,
				LBLSelectThankYouMail:       "Mail po zamówieniu",
				LBLSelectAdminMail:          "Mail po zamówieniu (admin)",
				LBLSelectMailHint:           "Wybierz mail do zamawiającego",
				LBLSelectAdminMailHint:      "Wybierz mail do administratora",
				LBLNoSitsSelectedTitle:      "Nie wybrano siedzień - tytuł",
				NoSitsSelectedTitleValue:    event.NoSitsSelectedTitle,
				LBLNoSitsSelectedText:       "Nie wybrano siedzeń - tekst",
				HTMLNoSitsSelectedTextValue: template.HTML(event.NoSitsSelectedText),
				LBLOrderHowTo:               "Podpowiedź jak zamówić",
				HTMLOrderHowToValue:         template.HTML(event.OrderHowto),
				LBLOrderDescription:         "Podpowiedź w formularzu zamówienia",
				OrderDescriptionValue:       event.OrderNotesDescription,
				LBLSharable:                 "Współdzielona",
				IsSharableVal:               event.Sharable.Bool,
				LBLRoomsSelect:              "Dodaj pomieszczenie do tego wydarzenia ...",
				LBLSelectRoomHint:           "ID przypisanych pomieszczeń (musi być przynajmniej jedno!)",
				Rooms:                       userrooms,
				BTNRoomAdd:                  "Przypisz",
				LBLRooms:                    "Pomieszczenia (ID w nawiasie)",
				RoomsVal:                    srooms,
				RoomsHelpText:               "Przypisz jeden lub więcej pomieszczeń do imprezy",
				BTNClearRooms:               "Usuń przypisanie",
				LBLCurrentRooms:             "Przypisane pomieszczenia",
				LBLTitleDates:               "Kiedy?",
				LBLTitlePrice:               "Za ile?",
				LBLTitleMails:               "Maile",
				LBLTitleOrdered:             "Wyświetl po zamówieniu",
				LBLTitleHowToOrder:          "Jak zamówić",
				LBLTitleNoSitsSelected:      "Kiedy nie wybrano siedzeń",
				LBLTitleRoomLegend:          "Legenda pomieszczenia(-eń), ceny, ...",
				LBLTitleSharable:            "Współdzielone",
				LBLRoomDescSection:          "Opis imprezy/pomieszczeń",
				LBLRoom1Desc:                "Opis imprezy/pomieszczenie 1",
				LBLRoom2Desc:                "Opis imprezy/pomieszczenie 2",
				LBLRoom3Desc:                "Opis imprezy/pomieszczenie 3",
				LBLRoom4Desc:                "Opis imprezy/pomieszczenie 4",
				LBLRoom1Banner:              "Banner pomieszczenie 1",
				LBLRoom2Banner:              "Banner pomieszczenie 2",
				LBLRoom3Banner:              "Banner pomieszczenie 3",
				LBLRoom4Banner:              "Banner pomieszczenie 4",
				Room1DescValue:              template.HTML(event.Room1Desc.String),
				Room2DescValue:              template.HTML(event.Room2Desc.String),
				Room3DescValue:              template.HTML(event.Room3Desc.String),
				Room4DescValue:              template.HTML(event.Room4Desc.String),
				Room1BannerValue:            event.Room1Banner.String,
				Room2BannerValue:            event.Room2Banner.String,
				Room3BannerValue:            event.Room3Banner.String,
				Room4BannerValue:            event.Room4Banner.String,
			}

			_ = rpEN

			t := template.Must(template.ParseFiles("tmpl/a_event.html", "tmpl/a_base.html"))
			err = t.ExecuteTemplate(w, "base", rpPL)
			if err != nil {
				log.Print("AdminEventEditor template executing error: ", err)
			}
		}
	}
}

func getRoomsString(rr []Room) string {
	if len(rr) == 0 {
		return ""
	}
	var s string
	for i := range rr {
		s += fmt.Sprintf("%d,", rr[i].ID)
	}
	return s[:len(s)-1]
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
			log.Printf("DEBUG: AR: eventIDs: %v", eventIDs)
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
		user, err := db.UserGetByEmail(email)
		if err != nil {
			log.Printf("error: AdminReservations: can't get user %q by mail, err: %v", email, err)
		}

		rf, err := db.ReservationFullGetAll(user.ID, eventID)
		if err != nil {
			log.Printf("error: AdminReservations: can not get reservations for event ID: %d and user ID: %d, err: %v", eventID, user.ID, err)
		}

		for i := range rf {
			rf[i].PayedDateS = ToDateTime(rf[i].PayedDate.Int64)
			rf[i].OrderedDateS = ToDateTime(rf[i].OrderedDate.Int64)
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
		_ = enP

		plP := AdminReservationsVars{
			LBLLang:          lang,
			EventID:          eventID,
			LBLTitle:         "Rezerwacje",
			LBLTotalPrice:    "Łącznie",
			LBLTotalSits:     "Bilety łącznie",
			THChairNumber:    "Nr krzesła",
			THRoomName:       "Pomieszczenie",
			THCustName:       "Imię",
			THCustSurname:    "Nazwisko",
			THCustEmail:      "Email",
			THCustPhone:      "Telefon",
			THPrice:          "Cena",
			THCurrency:       "Waluta",
			THStatus:         "Status",
			THNotes:          "Notatki",
			THOrderedDate:    "Zamówiono",
			THPayedDate:      "Zapłacono",
			ReservationsFull: rf,
		}

		t := template.Must(template.ParseFiles("tmpl/a_reservations.html", "tmpl/base.html"))
		err = t.ExecuteTemplate(w, "base", plP)
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
			LBLAboutTitle: template.HTML("REZERWO - Zaolzie reservation system"),
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
			LBLAboutText: template.HTML(`<b>You used Rezerwo. Thank you and see you again!</b><br />
			Leszek Cimała, admin (at) zori.cz`),
		}

		t := template.Must(template.ParseFiles("tmpl/about.html", "tmpl/base.html"))
		err := t.ExecuteTemplate(w, "base", pPL)
		if err != nil {
			log.Print("ErrorHTML: template executing error: ", err) //log it
		}
	}

}

type BankAccountEditorVars struct {
	LBLLang              string
	LBLTitle             string
	LBLQRPayTitle        string
	LBLQRPayHowTo        string
	LBLQRAccount         string
	LBLQRRecipientName   string
	LBLQRName            string
	LBLQRBankID          string
	LBLQRMessage         string
	LBLQAmountFieldName  string
	LBLQRCurrency        string
	LBLQRVarSymbol       string
	QRNameVal            string
	QRAccountVal         string
	QRRecipientNameVal   string
	QRBankIDVal          string
	QRMessageVal         string
	QRAmountFieldNameVal string
	QRCurrencyVal        string
	QRVarSymbolVal       string
	BTNSave              string
}

func BankAccountEditor(db *DB, lang string, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		_, _, email, err := InitSession(w, r, cs, "/admin/login", true)
		if err != nil {
			log.Printf("BankAccountEditor: problem getting session %v", err)
			return
		}
		user, err := db.UserGetByEmail(email)
		if err != nil {
			log.Printf("BankAccountEditor: problem getting user by mail %q, err: %v", email, err)
		}

		var curBA BankAccount

		err = r.ParseForm()
		if err != nil {
			log.Printf("BankAccountEditor: problem parsing form data, err: %v", err)
		}
		bankIDs := r.FormValue("ba-id")
		bankAccID, err := strconv.ParseInt(bankIDs, 10, 64)
		if err != nil {
			// it is probably new account, so name is sent
			name := r.FormValue("name")
			curBA = BankAccount{
				Name: name,
			}
			if name == "" {
				log.Printf("BankAccountEditor: error retrieving bank account, no valid ID %q and name %q", bankIDs, name)
			}
		} else {
			curBA, err = db.BankAccountGetByID(bankAccID, user.ID)
			if err != nil {
				log.Printf("BankAccountEditor: problem getting bank account by id %d, %v", bankAccID, err)
			}
		}

		pPL := BankAccountEditorVars{
			LBLLang:              lang,
			LBLTitle:             "Edycja kont bankowych (QRPay)",
			LBLQRPayTitle:        "QR Kod - płatność",
			LBLQRName:            "Nazwa konta(w Rezerwo):",
			LBLQRPayHowTo:        "Podaj namiary na konto, dla którego ma być wygenerowany kod QR. IBAN stwierdź z kodu wygenerowanego przez aplikację bankowości mobilnej.",
			LBLQRAccount:         "Konto bankowe w formacie IBAN (ważne!):",
			LBLQRRecipientName:   "Nazwa konta wyświetlana płacącemu (opcjonalne):",
			LBLQRMessage:         "Początek wiadomości do adrasata(np. bal2024), zostanie dodane imię i nazwisko:",
			LBLQRCurrency:        "Waluta (w formacie 3-znakowym: CZK, PLN, EUR, ...)",
			LBLQAmountFieldName:  "Nazwa pola na formularzu zawierającego sumę (opcjonalne):",
			LBLQRVarSymbol:       "Variabilní symbol (opcjonalne):",
			QRNameVal:            curBA.Name,
			QRAccountVal:         curBA.IBAN,
			QRRecipientNameVal:   curBA.RecipientName.String,
			QRBankIDVal:          curBA.BankID.String,
			QRMessageVal:         curBA.Message.String,
			QRAmountFieldNameVal: curBA.AmountField.String,
			QRCurrencyVal:        curBA.Currency,
			BTNSave:              "Zapisz",
		}

		t := template.Must(template.ParseFiles("tmpl/a_bankaccount.html", "tmpl/a_base.html"))
		err = t.ExecuteTemplate(w, "base", pPL)
		if err != nil {
			log.Print("ErrorHTML: template executing error: ", err) //log it
		}
	}

}

type EventOrForm struct {
	ID    int64
	Table string
	Name  string
}

type MailEditorVars struct {
	LBLLang            string
	LBLTitle           string
	NotificationIDVal  int64
	UserIDVal          int64
	LBLMailTitle       string
	LBLHowTo           string
	LBLSubject         string
	LBLTextHowTo       string
	LBLTextTitle       string
	LBLName            string
	LBLType            string
	LBLRelatedTo       string
	LBLSelectRelatedTo string
	LBLEmbeddedImgs    string
	EmbeddedImgsVal    string
	NameVal            string
	SubjectVal         string
	HTMLTextVal        template.HTML
	BTNSave            string
	BTNCopy            string
	BTNCancel          string
	IsSharableVal      bool
	LBLRelatedToEvents string
	LBLRelatedToForms  string
	LBLSharable        string
	LBLAttachedFiles   string
	AttachedFilesVal   string
	CurrentRelatedTo   string // events or forms
	IsOwner            bool
}

func MailEditor(db *DB, lang string, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		_, _, email, err := InitSession(w, r, cs, "/admin/login", true)
		if err != nil {
			log.Printf("MailEditor: problem getting session %v", err)
			return
		}
		user, err := db.UserGetByEmail(email)
		if err != nil {
			log.Printf("MailEditor: problem getting user by mail %q, err: %v", email, err)
		}

		var curMail Notification

		err = r.ParseForm()
		if err != nil {
			log.Printf("MailEditor: problem parsing form data, err: %v", err)
		}
		mailIDs := r.FormValue("mail-id") // it is empty for new notification
		log.Println(mailIDs)              //debug
		mailID, err := strconv.ParseInt(mailIDs, 10, 64)
		if err != nil {
			// it is probably new main/notification, so name is sent
			name := r.FormValue("name")
			curMail = Notification{
				Name:   name,
				UserID: user.ID,
			}
			if name == "" {
				log.Printf("MailEditor: error retrieving notification, no valid ID %q and name %q", mailIDs, name)
			}
		} else {
			curMail, err = db.NotificationGetByIDUnsafe(mailID)
			if err != nil {
				log.Printf("MailEditor: problem getting notification by id %d, %v", mailID, err)
			}
		}

		pPL := MailEditorVars{
			LBLLang:            lang,
			LBLType:            "Typ notyfikacji",
			LBLTitle:           "Edycja notyfikacji (mailowej)",
			NotificationIDVal:  curMail.ID,
			UserIDVal:          curMail.UserID,
			LBLMailTitle:       "Edycja notyfikacji",
			LBLName:            "Nazwa notyfikacji (jak będzie wyświelana w Rezerwo):",
			LBLHowTo:           "Podaj namiary na konto, dla którego ma być wygenerowany kod QR. IBAN stwierdź z kodu wygenerowanego przez aplikację bankowości mobilnej.",
			LBLSubject:         "Tytuł notyfikacji (e-mail)",
			LBLTextHowTo:       "Pomoc do notyfikacji, zmienne, obrazki w tekscie, QR kody, ... ",
			LBLTextTitle:       "Tekst notyfikacji (e-mail)",
			LBLRelatedTo:       "Powiązane z",
			LBLSelectRelatedTo: "Wybierz imprezę lub formularz",
			LBLRelatedToEvents: "Wydarzenie (rezerwacje)",
			LBLRelatedToForms:  "Formularz (deklaracja i inne bez mapy pomieszczenia)",
			LBLSharable:        "Udostępnij ten szablon dla innych",
			LBLEmbeddedImgs:    "Obrazki w treści maila (format: obrazek.jpeg;obrazek2.jpeg), w treści maila użyj: CID:obrazekjpeg@nazwaorg(z URL)",
			LBLAttachedFiles:   "Załączniki (format: plik.txt;plik2.jpeg)",
			NameVal:            curMail.Name,
			SubjectVal:         curMail.Title.String,
			HTMLTextVal:        template.HTML(curMail.Text),
			IsSharableVal:      curMail.Sharable,
			AttachedFilesVal:   curMail.AttachedFilesDelimited.String,
			CurrentRelatedTo:   curMail.RelatedTo,
			BTNSave:            "Zapisz",
			BTNCopy:            "Utwórz kopię",
			BTNCancel:          "Anuluj",
			IsOwner:            curMail.UserID == user.ID,
		}

		t := template.Must(template.ParseFiles("tmpl/a_mail_editor.html", "tmpl/a_base.html"))
		err = t.ExecuteTemplate(w, "base", pPL)
		if err != nil {
			log.Print("ErrorHTML: template executing error: ", err) //log it
		}
	}
}

type FormEditorVars struct {
	LBLLang                    string
	LBLTitle                   string
	LBLAdminHowTo              string
	LBLFormHowTo               string
	LBLFormName                string
	LBLFormURL                 string
	LBLFormBanner              string
	LBLFormThankYou            string
	LBLFormInfoPanel           string
	FormNameVal                string
	FormURLVal                 string
	FormBannerVal              string
	FormDataVal                string
	HTMLFormHowToVal           template.HTML
	HTMLFormThankYouVal        template.HTML
	HTMLFormInfoPanelVal       template.HTML
	LBLSelectBankAccount       string
	LBLBankAccountsSelectTitle string
	LBLSelectThankYouMail      string
	LBLSelectMailHint          string
	CurrentThankYouMail        int64
	LBLMoneyField              string
	MoneyFieldVal              string
	BTNSave                    string
	BTNSaveAndClose            string
	BTNClose                   string
	BTNSelect                  string
	BankAccounts               []BankAccount
	BTNNotificationEdit        string
	LBLThankYouTitle           string
	LBLThankYouMailTitle       string
	CurrentBankAccount         int64
	Notifications              []Notification
}

func FormEditor(db *DB, lang string, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var formTempl FormTemplate

		_, _, email, err := InitSession(w, r, cs, "/admin/login", true)
		if err != nil {
			log.Printf("FormEditor: problem getting session %v", err)
			return
		}

		user, err := db.UserGetByEmail(email)
		if err != nil {
			log.Printf("FormEditor: problem getting user by mail %q, err: %v", email, err)
		}

		ba, err := db.BankAccountGetAll(user.ID)
		if err != nil {
			log.Printf("FormEditor: problem getting bankaccounts for user %q, err: %v", user.Name.String, err)
		}

		err = r.ParseForm()
		if err != nil {
			log.Printf("FormEditor: problem parsing form data, err: %v", err)
		}
		formTemplID, err := strconv.ParseInt(r.FormValue("form-id"), 10, 64)
		if err != nil {
			log.Printf("info: probably no form-id given, so this is new form, err: %v", err)
			formTempl.Name = r.FormValue("name")
			formTempl.URL = r.FormValue("url")
			formTempl.UserID = user.ID
		} else {
			formTempl, err = db.FormTemplateGetByID(formTemplID, user.ID)
			if err != nil {
				log.Printf("error retrieving formTemplate with ID: %q from DB, err: %v", formTemplID, err)
			}
		}

		if formTempl.Content.String == "" || formTempl.Content.String == "[]" {
			formTempl.Content.String = `[{"type":"text","required":true,"label":"Imię","className":"form-control row-1 col-md-3","name":"name-REQUIRED","access":false,"subtype":"text","maxlength":30},{"type":"text","required":true,"label":"Nazwisko","className":"form-control row-1 col-md-4","name":"surname-REQUIRED","access":false,"subtype":"text","maxlength":50},{"type":"text","subtype":"email","required":true,"label":"E-mail","className":"form-control row-1 col-md-5","name":"email-REQUIRED","access":false,"maxlength":50}]`
		}

		var curBA BankAccount
		if formTempl.BankAccountID.Int64 != 0 {
			curBA, err = db.BankAccountGetByID(formTempl.BankAccountID.Int64, user.ID)
			if err != nil {
				log.Printf("FormEditor: can not get current bank account %d, %v", formTempl.BankAccountID.Int64, err)
			}
		}

		notifs, err := db.NotificationGetAllRelatedToFormsForUser(user.ID)
		if err != nil {
			log.Printf("FormEditor: error getting notifications for user %d, %v", user.ID, err)
		}

		log.Printf("notifictions: %v", notifs) // DEBUG

		pPL := FormEditorVars{
			LBLTitle:                   "Edytor deklaracji",
			LBLFormName:                "Nazwa formularza:",
			LBLAdminHowTo:              "Przeciągaj elementy na głowny ekran. Zmień ich tytuły i Zapisz. Pamiętaj by Link był unikatowy (np. dekl2024)!",
			LBLFormURL:                 "Link (URL):",
			LBLFormHowTo:               "Instrukcja dla wypełniającego (HTML):",
			LBLFormBanner:              "Banner formularza (szuka zdjęcia w katalogu domowym - /media/url/):",
			LBLFormThankYou:            "Komunikat/podziękowanie po wypełnienieniu formularza:",
			LBLFormInfoPanel:           "Panel boczny formularza (np. zliczanie aktualnych wartości):",
			BTNSave:                    "Zapisz",
			BTNSaveAndClose:            "Zapisz i zamknij",
			FormNameVal:                formTempl.Name,
			FormURLVal:                 formTempl.URL,
			FormBannerVal:              formTempl.Banner.String,
			FormDataVal:                formTempl.Content.String,
			HTMLFormHowToVal:           template.HTML(formTempl.HowTo.String),
			HTMLFormThankYouVal:        template.HTML(formTempl.ThankYou.String),
			HTMLFormInfoPanelVal:       template.HTML(reallyEmpty(formTempl.InfoPanel.String)),
			LBLBankAccountsSelectTitle: "Konta bankowe:",
			LBLSelectBankAccount:       "Wybierz konto ...",
			LBLMoneyField:              "Nazwa pola zawierającego ilość pieniedzy:",
			MoneyFieldVal:              chooseEmail(curBA.AmountField.String, formTempl.MoneyAmountFieldName.String),
			BankAccounts:               ba,
			CurrentBankAccount:         formTempl.BankAccountID.Int64,
			BTNClose:                   "Zamknij",
			BTNSelect:                  "Wybierz",
			LBLThankYouTitle:           "Wiadomość po wypełnieniu (podsumowanie + podziękowanie)",
			LBLThankYouMailTitle:       "E-mail z podsumowaniem, podziękowaniem",
			LBLSelectThankYouMail:      "Wybierz mail, który zostanie wysłany natychmiast po wypełnieniu formularza",
			LBLSelectMailHint:          "Wybierz mail ...",
			CurrentThankYouMail:        formTempl.NotificationID.Int64,
			BTNNotificationEdit:        "Edytuj",
			Notifications:              notifs,
		}

		t := template.Must(template.ParseFiles("tmpl/a_form_editor.html", "tmpl/a_base.html"))
		err = t.ExecuteTemplate(w, "base", pPL)
		if err != nil {
			log.Print("ErrorHTML: template executing error: ", err) //log it
		}
	}
}

func reallyEmpty(s string) string {
	log.Printf("after trimming: %s", strings.TrimSpace(s))
	t := bluemonday.StripTagsPolicy()
	if t.Sanitize(s) == "" {
		return ""
	}
	return s
}

type FormRapRow struct {
	FormID   int64
	AnswerID int64
	Value    string
}

type FormRaportVars struct {
	LBLLang               string
	FormTmplID            int64
	LBLTitle              string
	LBLTotalPrice         string
	FormFields            []FormField
	AnswersRows           [][]FormRapRow
	HTMLNotificationHowTo template.HTML
	LBLSelectNotification string
	Notifications         []Notification
}

func FormRaport(db *DB, lang string, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		formTmplID := int64(-1)
		if r.Method == "POST" {
			err := r.ParseForm()
			if err != nil {
				log.Printf("FormRaport: problem parsing form, err: %v", err)
			}
			formTemplIDs := r.FormValue("formtmpl-id")
			formTmplID, err = strconv.ParseInt(formTemplIDs, 10, 64)
			if err != nil {
				log.Printf("FormRaport: can not convert %q to int64, err: %v", formTemplIDs, err)
			}

		} else {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
		}
		_, _, email, err := InitSession(w, r, cs, "/admin/login", true)
		if err != nil {
			log.Printf("FormRaport info: %v", err)
			return
		}
		user, err := db.UserGetByEmail(email)
		if err != nil {
			log.Printf("FormRaport: can't get user %q by mail, err: %v", email, err)
		}

		// we only need moneyfield from FormTemplate
		tmpl, err := db.FormTemplateGetByID(formTmplID, user.ID)
		if err != nil {
			log.Printf("FormRaport: can not get FormTemplate with id: %d for user: %s, err: %v", formTmplID, user.Name.String, err)
		}
		_ = tmpl

		ff, err := db.FormFieldGetAllForTmpl(formTmplID)
		if err != nil {
			log.Printf("FormRaport: can not get fields for formTmplID %d, %v", formTmplID, err)
		}
		prependPL := []FormField{
			{Display: "ID"},
			{Display: "Status"},
			{Display: "Ostatnia notyf."},
			{Display: "Ilość wysłanych"},
			{Display: "Imię"},
			{Display: "Nazwisko"},
			{Display: "E-mail"},
			{Display: "Utworzono"},
		}

		cols := append(prependPL, ff...)

		// gen answers for this formTempl
		forms, err := db.FormGetAll(user.ID, formTmplID)
		if err != nil {
			log.Printf("FormRaport: can not get forms form formTmplID %d, %v", formTmplID, err)
		}

		rows := [][]FormRapRow{}
		for i := range forms {
			// get notification data first
			lastdate, err := db.FormNotificationLogGetLast(forms[i].ID)
			if err != nil {
				log.Printf("FormRaport: can not get notification date for %d, %v", forms[i].ID, err)
			}
			amount, err := db.FormNotificationLogGetAmount(forms[i].ID)
			if err != nil {
				log.Printf("FormRaport: can not get notification date for %d, %v", forms[i].ID, err)
			}

			row := make([]FormRapRow, len(cols))
			row[0] = FormRapRow{
				AnswerID: 0,
				FormID:   0,
				Value:    strconv.FormatInt(forms[i].ID, 10),
			}
			row[1] = FormRapRow{
				AnswerID: 0,
				FormID:   0,
				Value:    forms[i].Status.String,
			}
			row[2] = FormRapRow{
				AnswerID: 0,
				FormID:   0,
				Value:    ToDateTime(lastdate),
			}
			row[3] = FormRapRow{
				AnswerID: 0,
				FormID:   0,
				Value:    strconv.FormatInt(amount, 10),
			}
			row[4] = FormRapRow{
				AnswerID: 0,
				FormID:   0,
				Value:    forms[i].Name.String,
			}
			row[5] = FormRapRow{
				AnswerID: 0,
				FormID:   0,
				Value:    forms[i].Surname.String,
			}
			row[6] = FormRapRow{
				AnswerID: 0,
				FormID:   0,
				Value:    forms[i].Email.String,
			}
			row[7] = FormRapRow{
				AnswerID: 0,
				FormID:   0,
				Value:    ToDateTime(forms[i].CreatedDate),
			}

			ans, err := db.FormAnswerGetAll(forms[i].ID)
			if err != nil {
				log.Printf("FormRaport: can not get answers form formID %d, %v", forms[i].ID, err)
			}

			for n := range ans {
				index := getFormFieldIndex(ans[n].FormFieldID, ff) // search in column list column index
				if index == -1 {
					log.Println("answer", ans[n].Value.String, "with id", ans[n].FormFieldID, "not found in FormFields!")
					continue
				}
				row[index+len(prependPL)] = FormRapRow{ // we have to shift index, we are prependig some data to row
					AnswerID: ans[n].ID,
					FormID:   ans[n].FormID,
					Value:    ans[n].Value.String} // put answer data on given index
			}
			rows = append(rows, row)
		}

		// TODO: add support for shared notifications
		notifs, err := db.NotificationGetAllForUser(user.ID)
		if err != nil {
			log.Printf("FormRaport: error getting all notifications for user %d, %v", user.ID, err)
		}

		plP := FormRaportVars{
			LBLLang:               lang,
			FormTmplID:            formTmplID,
			LBLTitle:              "Reservations",
			LBLTotalPrice:         "Total",
			HTMLNotificationHowTo: template.HTML("Wybierz przygotowaną wcześniej wiadomość e-mail."),
			LBLSelectNotification: "Wybierz notyfikację ...",
			FormFields:            cols,
			AnswersRows:           rows,
			Notifications:         notifs,
		}

		t := template.Must(template.ParseFiles("tmpl/a_form_raport.html", "tmpl/base.html"))
		err = t.ExecuteTemplate(w, "base", plP)
		if err != nil {
			log.Print("FormRaport template executing error: ", err)
		}

	}
}

// getFormFieldIndex returns possition in given list if found
// or -1 if not in list
func getFormFieldIndex(formFieldID int64, ff []FormField) int {
	for i := range ff {
		if formFieldID == ff[i].ID {
			return i
		}
	}
	return -1
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
				//log.Printf("info: inserting furniture failed, trying to update, err: %v", err) // disable this, it is normal to happen and makes me panic ;-)
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
				// log.Printf("info: price inserting failed, trying update, err: %v", err) // also disabled - do not panic ;-)
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

type EventJson struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Language              string `json:"language"`
	Date                  string `json:"date"`
	FromDate              string `json:"from_date"`
	ToDate                string `json:"to_date"`
	DefaultPrice          string `json:"price"`
	DefaultCurrency       string `json:"currency"`
	NoSitsSelectedTitle   string `json:"no_sits_selected_title"`
	NoSitsSelectedText    string `json:"no_sits_selected_text"`
	OrderHowto            string `json:"order_howto"`
	OrderNotesDescription string `json:"order_notes_desc"`
	OrderedNoteTitle      string `json:"ordered_note_title"`
	OrderedNoteText       string `json:"ordered_note_text"`
	//MailSubject               string `json:"mail_subject"`
	//MailText                  string `json:"mail_text"`
	//MailAttachmentsDelimited  string `json:"mail_attachments"`
	//MailEmbeddedImgsDelimited string `json:"mail_embeded_imgs"`
	//AdminMailSubject          string `json:"admin_mail_subject"`
	//AdminMailText             string `json:"admin_mail_text"`
	HowTo          string `json:"how_to"`
	UserID         string `json:"user_id"`
	ThankYouMailID string `json:"thankyou_notifications_id_fk"`
	AdminMailID    string `json:"admin_notifications_id_fk"`
	Sharable       bool   `json:"sharable"`
	BankAccountID  string `json:"bank_account_id"`
	Rooms          string `json:"rooms"`
	Room1Desc      string `json:"room1desc"`
	Room1Banner    string `json:"room1banner"`
	Room2Desc      string `json:"room2desc"`
	Room2Banner    string `json:"room2banner"`
	Room3Desc      string `json:"room3desc"`
	Room3Banner    string `json:"room3banner"`
	Room4Desc      string `json:"room4desc"`
	Room4Banner    string `json:"room4banner"`
}

// EventAddMod is API func
func EventAddMod(db *DB, loc *time.Location, dF string, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var eventJson EventJson

		_, _, email, err := InitSession(w, r, cs, "/admin/login", true)
		if err != nil {
			log.Printf("EventNew: session error: %v", err)
			return
		}

		user, err := db.UserGetByEmail(email)
		if err != nil {
			log.Printf("EventNew: can not get user by mail %q, %v", email, err)
			return
		}

		if r.Method == "POST" {
			bodyJson, err := io.ReadAll(r.Body)
			if err != nil {
				log.Println("EventAddMod: error reading event json sent from event editor,", err)
			}
			err = json.Unmarshal(bodyJson, &eventJson)
			if err != nil {
				log.Println(err)
			}
			date, err := ToUnix(eventJson.Date, loc, dF)
			if err != nil {
				log.Printf("EventAddMod: error converting date string %q to int64, %v", eventJson.Date, err)
			}

			fromDate, err := ToUnix(eventJson.FromDate, loc, dF)
			if err != nil {
				log.Printf("EventAddMod: error converting fromDate string %q to int64, %v", eventJson.FromDate, err)
			}
			toDate, err := ToUnix(eventJson.ToDate, loc, dF)
			if err != nil {
				log.Printf("EventAddMod: error converting toDate string %q to int64, %v", eventJson.ToDate, err)
			}
			price, err := strconv.ParseInt(eventJson.DefaultPrice, 10, 64)
			if err != nil {
				log.Printf("EventAddMod: error converting price string %q to int64, %v", eventJson.DefaultPrice, err)
			}
			id, err := strconv.ParseInt(eventJson.ID, 10, 64)
			if err != nil {
				log.Printf("EventAddMod: error converting id string %q to int64, %v", eventJson.ID, err)
			}
			userid, err := strconv.ParseInt(eventJson.UserID, 10, 64)
			if err != nil {
				log.Printf("EventAddMod: error converting userid string %q to int64, %v", eventJson.UserID, err)
			}
			thankYouMailID, err := strconv.ParseInt(eventJson.ThankYouMailID, 10, 64)
			if err != nil {
				log.Printf("EventAddMod: error converting current ThankYou string %q to int64, %v", eventJson.ThankYouMailID, err)
			}
			adminMailID, err := strconv.ParseInt(eventJson.AdminMailID, 10, 64)
			if err != nil {
				log.Printf("EventAddMod: error converting current adminMail string %q to int64, %v", eventJson.AdminMailID, err)
			}
			// for now not used
			bankAccountID, err := strconv.ParseInt(eventJson.BankAccountID, 10, 64)
			if err != nil {
				log.Printf("EventAddMod: error converting current bankAccountID string %q to int64, %v", eventJson.BankAccountID, err)
			}
			rr := strings.Split(eventJson.Rooms, ",")
			for i := range rr {
				roomID, err := strconv.ParseInt(rr[i], 10, 64)
				if err != nil {
					log.Printf("EventAddMod: error converting room id %q to int64, %v", rr[i], err)
				}
				err = db.RoomEventAdd(id, roomID)
				if err != nil {
					log.Printf("EventAddMod: error (it may be normal if already exists) adding room %d, to event %d, %v", roomID, id, err)
				}
			}
			// empty descriptions if no text (just <p><br></p> tags)
			re := regexp.MustCompile(`(?i)</?(p|br)\s*/?>`)
			for i, v := range []string{eventJson.Room1Desc, eventJson.Room2Desc, eventJson.Room3Desc, eventJson.Room4Desc} {
				if strings.TrimSpace(re.ReplaceAllString(v, "")) == "" {
					switch i {
					case 0:
						eventJson.Room1Desc = ""
					case 1:
						eventJson.Room2Desc = ""
					case 2:
						eventJson.Room3Desc = ""
					case 3:
						eventJson.Room4Desc = ""
					}
				}
			}

			ev := &Event{
				ID:                      id,
				Name:                    eventJson.Name,
				Language:                ToNS(eventJson.Language),
				Date:                    date,
				FromDate:                fromDate,
				ToDate:                  toDate,
				DefaultPrice:            price,
				DefaultCurrency:         eventJson.DefaultCurrency,
				NoSitsSelectedTitle:     eventJson.NoSitsSelectedTitle,
				NoSitsSelectedText:      eventJson.NoSitsSelectedText,
				OrderHowto:              eventJson.OrderHowto,
				OrderNotesDescription:   eventJson.OrderNotesDescription,
				OrderedNoteTitle:        eventJson.OrderedNoteTitle,
				OrderedNoteText:         eventJson.OrderedNoteText,
				ThankYouNotificationsID: ToNI(thankYouMailID),
				AdminNotificationsID:    ToNI(adminMailID),
				HowTo:                   eventJson.HowTo,
				UserID:                  userid,
				Sharable:                ToNB(eventJson.Sharable),
				BankAccountsID:          ToNI(bankAccountID),
				Room1Desc:               ToNS(eventJson.Room1Desc),
				Room1Banner:             ToNS(eventJson.Room1Banner),
				Room2Desc:               ToNS(eventJson.Room2Desc),
				Room2Banner:             ToNS(eventJson.Room2Banner),
				Room3Desc:               ToNS(eventJson.Room3Desc),
				Room3Banner:             ToNS(eventJson.Room3Banner),
				Room4Desc:               ToNS(eventJson.Room4Desc),
				Room4Banner:             ToNS(eventJson.Room4Banner),
			}

			//spew.Dump(ev) // DEBUG

			var lastid int64

			if id != 0 { // when it is existing event
				//but from other user - create copy
				if userid != user.ID {
					ev.ID = 0
					ev.UserID = user.ID
					ev.Sharable = ToNB(false)
					lastid, err = db.EventAdd(ev)
				} else { // it is existing event, and belongs to user
					err = db.EventModByID(ev, user.ID)
				}

			} else { // it is new event
				lastid, err = db.EventAdd(ev)
			}

			if err != nil {
				log.Printf("EventAddMod: insert and update of %v failed, %v", ev, err)
				http.Error(w, fmt.Sprintf(`{"msg":"Nie udało się zapisać imprezy!\n%s"}`, err.Error()), http.StatusTeapot) // 418
			} else {
				w.Write([]byte(fmt.Sprintf(`{"msg":"inserted or updated %d (%s)"}`, lastid, ev.Name)))
			}
		}
	}
}

type FormTemplJson struct {
	Name           string         `json:"name"`
	URL            string         `json:"url"`
	Banner         string         `json:"banner"`
	HowTo          string         `json:"howto"`
	ThankYou       string         `json:"thankyou"`
	ThankYouMailID string         `json:"thankyoumail"`
	InfoPanel      string         `json:"infopanel"`
	BankAccount    string         `json:"bankaccount"`
	MoneyField     string         `json:"moneyfield"`
	Content        []FormFieldDef `json:"content"`
}

type FormTemplRawContent struct {
	Content json.RawMessage `json:"content"`
}

type FormFieldDef struct {
	Type    string `json:"type"`
	Display string `json:"label"`
	Name    string `json:"name"`
}

func (f FormFieldDef) GetType() string {
	return f.Type
}

func (f FormFieldDef) GetName() string {
	return f.Name
}

// FormTemplateAddMod is API func
func FormTemplateAddMod(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var formTemplJson FormTemplJson
		var rawContent FormTemplRawContent

		_, _, email, err := InitSession(w, r, cs, "/admin/login", true)
		if err != nil {
			log.Printf("FormTemplateNew: session error: %v", err)
			return
		}

		user, err := db.UserGetByEmail(email)
		if err != nil {
			log.Printf("FormTemplateNew: can not get user by mail %q, %v", email, err)
			return
		}

		if r.Method == "POST" {
			bodyJson, err := io.ReadAll(r.Body)
			if err != nil {
				log.Println("FormTemplateAddMod: error reading form template json sent from form editor,", err)
			}
			err = json.Unmarshal(bodyJson, &formTemplJson)
			if err != nil {
				log.Println(err)
			}

			//log.Println("fields:")
			//log.Println(formTemplJson.Content)
			log.Println("BA:")
			log.Println(formTemplJson.BankAccount)
			log.Println("raw json:")
			log.Println(string(bodyJson))

			err = json.Unmarshal(bodyJson, &rawContent)
			if err != nil {
				log.Printf("FormTemplateAddMod: error marshalling fields to the string, err: %v", err)
				return
			}

			baID, err := strconv.ParseInt(formTemplJson.BankAccount, 10, 64)
			if err != nil {
				log.Printf("FormTemplateAddMod: error converting bankaccount string %q to int64, %v", formTemplJson.BankAccount, err)
			}

			notifID, err := strconv.ParseInt(formTemplJson.ThankYouMailID, 10, 64)
			if err != nil {
				log.Printf("FormTemplateAddMod: error converting ThankYouMailID string %q to int64, %v", formTemplJson.ThankYouMailID, err)
			}

			//log.Println("raw json content:", string(rawContent.Content))

			ft := &FormTemplate{
				Name:                 formTemplJson.Name,
				URL:                  formTemplJson.URL,
				UserID:               user.ID,
				HowTo:                ToNS(formTemplJson.HowTo),
				Banner:               ToNS(formTemplJson.Banner),
				ThankYou:             ToNS(formTemplJson.ThankYou),
				NotificationID:       ToNI(notifID),
				InfoPanel:            ToNS(reallyEmpty(formTemplJson.InfoPanel)),
				BankAccountID:        ToNI(baID),
				MoneyAmountFieldName: ToNS(formTemplJson.MoneyField),
				CreatedDate:          time.Now().Unix(),
				Content:              ToNS(string(rawContent.Content)),
				// TODO: if we want to assign it to event, add here:
				// EventID: "get event somehow",
			}

			// try to add FormTemplate to db
			lastid, err := db.FormTemplateAdd(ft)
			// form template probably already exists,
			// we will try to modify it
			if err != nil {
				//log.Printf("FormTemplateAddMod: insert failed, probably already exists in db, %v", err)
				if len(string(rawContent.Content)) < 5 {
					log.Println("FormTemplateAddMod: while updating form template, definition is < 5, no changes saved")
					http.Error(w, `"msg":"Zawartość formularza < 5 znaków, nie zapisuję niczego!"`, http.StatusTeapot)
					return
				}
				err := db.FormTemplateModByURL(ft)
				if err != nil {
					log.Printf("FormTemplateAddMod: insert and update faied! %v", err)
					http.Error(w, fmt.Sprintf(`{"msg":"Nie udało się zapisać formularza!\n%s"}`, err.Error()), http.StatusTeapot) // amazing status 418
				} else {
					w.Write([]byte(`{"msg":"updated"}`))
				}
			} else {
				w.Write([]byte(fmt.Sprintf(`{"msg":"inserted %d"}`, lastid)))
			}

			tmpl, err := db.FormTemplateGetByURL(formTemplJson.URL, user.ID)
			if err != nil {
				log.Printf("FormTemplateAddMod: can not get FormTemplate by URL %q for User %d, %v", formTemplJson.URL, user.ID, err)
			}
			updateFormFieldsInDB(tmpl.ID, formTemplJson.Content, db)
		}
	}
}

func updateFormFieldsInDB(templateID int64, current []FormFieldDef, db *DB) {
	current = filterFormFields(current)
	previous, err := db.FormFieldGetAllForTmpl(templateID)
	if err != nil {
		log.Printf("updateFormFieldsInDB: error getting FormFields for template %d, %v", templateID, err)
	}
	previous = filterFormFields(previous)

	found := make([]bool, len(previous))

	for i := range current {
		f := &FormField{
			Name:           current[i].Name,
			Display:        current[i].Display,
			Type:           current[i].Type,
			FormTemplateID: templateID,
		}
		if n := isFieldIn(current[i].Name, previous); n >= 0 {
			found[n] = true
			err := db.FormFieldModByName(f)
			if err != nil {
				log.Printf("updateFormFieldsInDB: error updating field %q, %v", f.Name, err)
			}
		} else { // this is new field
			lastid, err := db.FormFieldAdd(f)
			if err != nil {
				log.Printf("updateFormFieldsInDB: error adding field %q, %v", f.Name, err)
			}
			_ = lastid
		}
	}

	// remove old fields, which are not in the template anymore
	for i := range found {
		if !found[i] {
			err := db.FormFieldDelByName(previous[i].Name, templateID)
			if err != nil {
				log.Printf("updateFormFieldsInDB: error deleting old formfield %q, templateID %d, %v", previous[i].Name, templateID, err)
			}
		}
	}

}

// isFieldIn returns possition in given list if found
// or -1 if not in list
func isFieldIn(name string, l []FormField) int {
	for i := range l {
		if name == l[i].Name {
			return i
		}
	}
	return -1
}

type BankAccountJson struct {
	Name      string `json:"name"`
	Account   string `json:"account"`
	Recipient string `json:"recipient"`
	Message   string `json:"message"`
	FieldName string `json:"fieldname"`
	VarSymbol string `json:"varsymbol"`
	Currency  string `json:"currency"`
}

func BankAccountAddMod(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, email, err := InitSession(w, r, cs, "/admin/login", true)
		if err != nil {
			log.Printf("FormTemplateNew: session error: %v", err)
			return
		}

		user, err := db.UserGetByEmail(email)
		if err != nil {
			log.Printf("FormTemplateNew: can not get user by mail %q, %v", email, err)
			return
		}

		if r.Method == "POST" {
			var b BankAccountJson
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&b)
			if err != nil {
				log.Printf("BankAccountAddMod: error reading json answer, %v", err)
			}

			vs, err := strconv.ParseInt(strings.TrimSpace(b.VarSymbol), 10, 64)
			if err != nil {
				log.Printf("BankAccountAddMod: error converting varsymbol %q to int, %v", b.VarSymbol, err)
			}

			log.Printf("%+v", b) // debug

			ba := &BankAccount{
				Name:          b.Name,
				IBAN:          b.Account,
				RecipientName: ToNS(b.Recipient),
				Message:       ToNS(b.Message),
				AmountField:   ToNS(b.FieldName),
				VarSymbol:     ToNI(vs),
				Currency:      b.Currency,
				UserID:        user.ID,
			}

			lastid, err := db.BankAccountAdd(ba)

			if err != nil {
				err = db.BankAccountModByName(ba)
				if err != nil {
					log.Printf("BankAccountAddMod: insert and update of %v failed, %v", ba, err)
					http.Error(w, fmt.Sprintf(`{"msg":"Nie udało się zapisać konta!\n%s"}`, err.Error()), http.StatusTeapot) // 418
				} else {
					w.Write([]byte(fmt.Sprintf(`{"msg":"updated %s"}`, b.Name)))
				}
			} else {
				w.Write([]byte(fmt.Sprintf(`{"msg":"inserted %d"}`, lastid)))
			}
		}

	}
}

type MailJson struct {
	ID            string `json:"id"`
	UserID        string `json:"userid"`
	Name          string `json:"name"`
	Subject       string `json:"subject"`
	Text          string `json:"text"`
	Sharable      bool   `json:"sharable"`
	RelatedTo     string `json:"relatedto"` // events or forms
	EmbeddedImgs  string `json:"embeddedimgs"`
	AttachedFiles string `json:"attachedfiles"`
}

func MailAddMod(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, email, err := InitSession(w, r, cs, "/admin/login", true)
		if err != nil {
			log.Printf("MailAddMod: session error: %v", err)
			return
		}

		user, err := db.UserGetByEmail(email)
		if err != nil {
			log.Printf("MailAddMod: can not get user by mail %q, %v", email, err)
			return
		}

		if r.Method == "POST" {
			var b MailJson
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&b)
			if err != nil {
				log.Printf("MailAddMod: error reading json answer, %v", err)
			}

			//log.Printf("%+v", b) // debug

			now := time.Now().Unix()

			id, err := strconv.ParseInt(b.ID, 10, 64)
			if err != nil {
				log.Printf("MailAddMod: error converting id %q to int64, %v", b.ID, err)
			}

			userid, err := strconv.ParseInt(b.UserID, 10, 64)
			if err != nil {
				log.Printf("MailAddMod: error converting userID %q to int64, %v", b.UserID, err)
			}

			not := &Notification{
				ID:                     id,
				Name:                   b.Name,
				Type:                   "mail", // for now only option
				RelatedTo:              b.RelatedTo,
				Title:                  ToNS(b.Subject),
				Text:                   makeSureIsHTML(b.Text),
				EmbeddedImgsDelimited:  ToNS(b.EmbeddedImgs),
				AttachedFilesDelimited: ToNS(b.AttachedFiles),
				Sharable:               b.Sharable,
				CreatedDate:            now,
				UpdatedDate:            ToNI(now),
				UserID:                 user.ID,
			}

			var lastid int64

			// when it is existing notifications
			if id != 0 {
				//but from other user - create copy
				if userid != user.ID {
					not.ID = 0
					not.UserID = user.ID
					not.Sharable = false // do not share copy of notification by default
					lastid, err = db.NotificationAdd(not)
				} else { // it is existing notifications, and belongs to user
					log.Println("we have existing")
					err = db.NotificationModByID(not, user.ID)
				}
			} else { // it is new notification
				lastid, err = db.NotificationAdd(not)
			}

			if err != nil {
				log.Printf("MailAddMod: insert and update of %v failed, %v", not, err)
				http.Error(w, fmt.Sprintf(`{"msg":"Nie udało się zapisać notyfikacji!\n%s"}`, err.Error()), http.StatusTeapot) // 418
			} else {
				w.Write([]byte(fmt.Sprintf(`{"msg":"inserted or updated %d (%s)"}`, lastid, b.Name)))
			}
		}
	}
}

type FormFieldType interface {
	FormField | FormFieldDef
	GetName() string
	GetType() string
}

// filterFormFields is generic function which works
// for both formfield types
func filterFormFields[L FormFieldType](l []L) []L {
	var ret []L
	for i := range l {
		if l[i].GetName() == "" {
			continue
		}
		if !isTypeIgnored(l[i].GetType()) {
			ret = append(ret, l[i])
		}
	}
	return ret
}

func isTypeIgnored(t string) bool {
	var ignored = []string{"header", "paragraph", "button", "hidden"}
	for i := range ignored {
		if ignored[i] == t {
			return true
		}
	}
	return false
}

type FormAnsData struct {
	URI    string            `json:"uri"`
	UniqID string            `json:"uniqid"`
	Data   []FormAnswersJson `json:"data"`
}

type FormAnswersJson struct {
	Name     string   `json:"name"`
	UserData []string `json:"userData"`
}

// FormAddMod is API func to write form answer to DB and send mail with summary
func FormAddMod(db *DB, mailConf *MailConfig) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var formAnsJson FormAnsData

		if r.Method == "POST" {

			bodyJson, err := io.ReadAll(r.Body)
			if err != nil {
				log.Println("FormAddMod: error reading form template json sent from form editor,", err)
			}
			err = json.Unmarshal(bodyJson, &formAnsJson)
			if err != nil {
				log.Println(err)
			}

			userURL, templURL := parseFormURI(formAnsJson.URI)

			user, err := db.UserGetByURL(userURL)
			if err != nil {
				log.Printf("FormAddMod: can not get user by url %q, %v", userURL, err)
				http.Error(w, fmt.Sprintf(`{"msg":"Nie udało się zapisać formularza!\nCan not get user by url %q,\n%s"}`, userURL, err.Error()), http.StatusTeapot)
				return
			}

			templ, err := db.FormTemplateGetByURL(templURL, user.ID)
			if err != nil {
				log.Printf("FormAddMod: can not get template by url %q, user %d, %v", templURL, user.ID, err)
				http.Error(w, fmt.Sprintf(`{"msg":"Nie udało się zapisać formularza!\n%s"}`, err.Error()), http.StatusTeapot)
				return
			}

			log.Println("answer parsed json:", formAnsJson)
			//log.Println("raw json:")
			//log.Println(string(bodyJson))

			// we can have name, surname, email or UniqID (for anonymous forms)
			name, surname, email := getBasicFields(formAnsJson.Data, formAnsJson.UniqID)

			f := &Form{
				Name:           ToNS(name),
				Surname:        ToNS(surname),
				Email:          ToNS(email),
				CreatedDate:    time.Now().Unix(),
				UserID:         user.ID,
				FormTemplateID: templ.ID,
			}
			// try to add Form to db
			FormID, err := db.FormAdd(f)
			if err != nil {
				// form probably already exists,
				// we will try to modify it

				log.Printf("FormAddMod: insert failed, probably already exists in db, %v", err)
				err := db.FormModByEmail(f)
				if err != nil {
					log.Printf("FormAddMod: insert and update failed! %v", err)
					http.Error(w, fmt.Sprintf(`{"msg":"Nie udało się zapisać formularza!\n%s"}`, err.Error()), http.StatusTeapot) // 418
					return
				} else {
					FormID, err = db.FormGetIDByEmail(f.Email.String, templ.ID, user.ID)
					if err != nil {
						log.Printf("FormAddMod: can not get FormID via email %q, templateID %d, userID %d, %v", f.Email.String, templ.ID, user.ID, err)
					}
					//w.Write([]byte(fmt.Sprintf(`{"formid":"%d"}`, FormID)))
					w.Write([]byte(fmt.Sprintf(`{"formid":"%d", "name":"%s", "surname":"%s", "templurl":"%s"}`, FormID, name, surname, templ.URL)))
				}
			} else {
				w.Write([]byte(fmt.Sprintf(`{"formid":"%d", "name":"%s", "surname":"%s", "templurl":"%s"}`, FormID, name, surname, templ.URL)))
			}

			// add/update answers

			// get all formfields first
			formfields, err := db.FormFieldGetAllForTmpl(templ.ID)
			if err != nil {
				log.Printf("FormAddMod: error getting all formfields for templateID %d, %v", templ.ID, err)
			}

			for i := range formAnsJson.Data {
				var v sql.NullString
				var fi FormField
				n := isFieldIn(formAnsJson.Data[i].Name, formfields)
				if n > -1 {
					fi = formfields[n]
				} else {
					continue // ignore fields with empty names
				}
				if !isEmpty(formAnsJson.Data[i].UserData) {
					// if checkbox or other form of "multiple answers", join them to one string
					if len(formAnsJson.Data[i].UserData) > 1 {
						v = ToNS(strings.Join(formAnsJson.Data[i].UserData, ", "))
					} else {
						v = ToNS(formAnsJson.Data[i].UserData[0])
					}
				} else {
					v = ToNS("")
				}
				formAnsField := &FormAnswer{
					Value:       v,
					FormFieldID: fi.ID,
					FormID:      FormID,
				}
				_, err = db.FormAnswerAdd(formAnsField)
				if err != nil {
					log.Printf("FormAddMod: info: adding answer failed, FormFieldID %q(%d), Forms %d, val: %s, %v", fi.Name,
						fi.ID, FormID, v.String, err,
					)
					err = db.FormAnswerMod(formAnsField)
					if err != nil {
						log.Printf("FormAddMod: insert and update of FormAnswer failed! FormFieldID %q(%d), Forms %d, val: %s, %v",
							fi.Name, fi.ID, FormID, v.String, err,
						)
					}

				}
			}
			// Send mail after form submission.
			mail, err := db.NotificationGetByID(templ.NotificationID.Int64, user.ID)
			if err != nil {
				log.Printf("FormAnsSendMail: error getting mail with ID: %d, err: %v", templ.NotificationID.Int64, err)
			}

			err = prepareAndSendMail(
				f.Email.String,
				f.Surname.String,
				f.Name.String,
				mail.Title.String,
				makeSureIsHTML(mail.Text),
				user,
				FormID,
				templ,
				db,
				mailConf,
			)

			if err != nil {
				log.Printf("FormAnsSendMail: error sending, %v", err)
			}

			// Check if admin mail template is defined
			if !templ.AdminNotificationID.Valid {
				log.Println("FormAnsSendMail: admin mail template is not defined")
				return
			}
			// It is defined, so send mail to admin
			adminMail, err := db.NotificationGetByID(templ.AdminNotificationID.Int64, user.ID)
			if err != nil {
				log.Printf("FormAnsSendMail: error getting admin mail with ID: %d, err: %v", templ.AdminNotificationID.Int64, err)
			}

			err = prepareAndSendMail(
				chooseEmail(user.Email, user.AltEmail.String), // send to alt mail if defined
				f.Surname.String,
				f.Name.String,
				adminMail.Title.String,
				makeSureIsHTML(adminMail.Text),
				user,
				FormID,
				templ,
				db,
				mailConf,
			)

			if err != nil {
				log.Printf("FormAnsSendMail: error sending admin mail, %v", err)
			}

		}
	}
}

type FormAnsManipulateJson struct {
	FormTemplateID int64 `json:"formtmpl_id"`
	FormID         int64 `json:"forms_id"`
	NotificationID int64 `json:"notification_id"`
}

func FormAnsDelete(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			var m FormAnsManipulateJson
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&m)
			if err != nil {
				log.Printf("FormAnsDelete: problem decoding json, err: %v", err)
			}

			err = db.FormAnswerDel(m.FormID, m.FormTemplateID)
			if err != nil {
				log.Printf("FormAnsDelete: problem deleting formanswers, formID: %d (templID: %d), err: %v", m.FormID, m.FormTemplateID, err)
			}

			err = db.FormDel(m.FormID, m.FormTemplateID)
			if err != nil {
				log.Printf("FormAnsDelete: problem deleting formID: %d (templID: %d), err: %v", m.FormID, m.FormTemplateID, err)
			}
			// TODO: is that all? Should we return error to frontend if occurs?

		}

	}
}

func FormAnsSendMail(db *DB, mailConf *MailConfig, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var m FormAnsManipulateJson

			_, _, userEmail, err := InitSession(w, r, cs, "/admin/login", true)
			if err != nil {
				log.Printf("FormAnsSendMail: session error: %v", err)
				return
			}

			user, err := db.UserGetByEmail(userEmail)
			if err != nil {
				log.Printf("FormAnsSendMail: very strange, can not find admin in users table, but is authenticated, err: %v", err)
			}

			bodyJson, err := io.ReadAll(r.Body)
			if err != nil {
				log.Println("FormAnsSendMail: error reading form template json sent from form editor,", err)
			}
			err = json.Unmarshal(bodyJson, &m)
			if err != nil {
				log.Println(err)
			}

			form, err := db.FormGet(m.FormID, m.FormTemplateID)
			if err != nil {
				log.Println("FormAnsSendMail: error reading form template json sent from form editor,", err)
			}

			//log.Printf("%+v, email: %s", m, form.Email.String) // debug

			formTempl, err := db.FormTemplateGetByID(m.FormTemplateID, user.ID)
			if err != nil {
				log.Printf("FormAnsSendMail: error retrieving formTemplate with ID: %d from DB, err: %v", m.FormTemplateID, err)
			}

			mail, err := db.NotificationGetByID(m.NotificationID, user.ID)
			if err != nil {
				log.Printf("FormAnsSendMail: error getting mail with ID: %d, err: %v", formTempl.NotificationID.Int64, err)
			}

			err = prepareAndSendMail(
				form.Email.String,
				form.Surname.String,
				form.Name.String,
				mail.Title.String,
				makeSureIsHTML(mail.Text),
				user,
				m.FormID,
				formTempl,
				db,
				mailConf,
			)

			if err != nil {
				log.Printf("FormAnsSendMail: error sending, %v", err)
			} else {
				// write info to db about sent mail (FormNotificationLog table)
				_, err := db.FormNotificationLogAdd(&FormNotificationLog{
					Date:           time.Now().Unix(),
					NotificationID: m.NotificationID,
					FormID:         m.FormID,
				})
				if err != nil {
					log.Printf("prepareAndSendMail: error writting to DB - logging sent mail info, %v", err)
				}
			}

		}
	}
}

// prepareAndSendMail will prepare mail text and send mail
func prepareAndSendMail(email, name, surname, msubject, mtext string, user User, formID int64, formTempl FormTemplate, db *DB, mailConf *MailConfig) error {
	var parsedMail bytes.Buffer

	// if email is empty we assume it anonymous form. We will not send any mail
	if email == "" {
		return fmt.Errorf("prepareAndSendMail: no mail sent for %v, empty mail(anonymous form?)", formID)
	}

	nsn := surname + "_" + name

	thp := &FormFuncs{DB: db, User: user, Template: formTempl, FormID: formID, NameSurname: nsn}

	tmpl, err := template.New("thankyou").Parse(mtext)
	if err != nil {
		log.Printf("prepareAndSendMail: error parsing ThankYou mail template, %v", err)
	}
	err = tmpl.ExecuteTemplate(&parsedMail, "thankyou", thp)
	if err != nil {
		log.Printf("prepareAndSendMail: error executing ThankYou template, %v", err)
	}

	//log.Println("MAIL TEXT:", parsedMail.String()) // DEBUG

	mail := MailConfig{
		Server:          mailConf.Server,
		Port:            mailConf.Port,
		User:            mailConf.User,
		Pass:            mailConf.Pass,
		From:            chooseEmail(user.Email, user.AltEmail.String), // choose user (organizators) mails. Use primary email if alt_email is NULL. Otherwise use alt_email.
		ReplyTo:         user.Email,                                    // it is ignored anyway
		Sender:          mailConf.Sender,
		To:              []string{email},
		Subject:         msubject,
		Text:            parsedMail.String(),
		IgnoreCert:      mailConf.IgnoreCert,
		Hostname:        mailConf.Hostname,
		EmbededHTMLImgs: thp.EmbeddedImgs,
	}

	err = MailSend(mail)
	if err != nil {
		return err
	}
	return nil
}

// parseFormURI parses: /form/org/formname
func parseFormURI(uri string) (string, string) {
	ss := strings.Split(uri, "/")
	log.Printf("%+v", ss)
	if len(ss) == 4 { // first element is empty (before first /)
		return ss[2], ss[3]
	}
	return "", ""
}

func makeSureIsHTML(s string) string {
	if strings.Contains(s, "<html>") {
		return s
	}
	return "<html>" + s + "</html>"
}

func getBasicFields(j []FormAnswersJson, uniqID string) (string, string, string) {
	var name, surname, email string
	log.Println("json:", j)
	for i := range j {
		if j[i].Name == "name-REQUIRED" {
			if !isEmpty(j[i].UserData) {
				name = j[i].UserData[0]
			}
		}
		if j[i].Name == "surname-REQUIRED" {
			if !isEmpty(j[i].UserData) {
				surname = j[i].UserData[0]
			}
		}
		if j[i].Name == "email-REQUIRED" {
			if !isEmpty(j[i].UserData) {
				email = j[i].UserData[0]
			}
		}
	}
	if surname == "" {
		name = "- Anon -"
		surname = uniqID
	}
	log.Println(name, surname, email)
	return name, surname, email
}

func isEmpty[T comparable](t []T) bool {
	if len(t) > 0 {
		return false
	}
	return true
}

func FormTemplsGetAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		type FTJson struct {
			ID   int
			Name string
		}

		fts := []FTJson{}
		_, _, email, err := InitSession(w, r, cs, "/admin/login", true)
		if err != nil {
			log.Printf("info: FormTemplsGetAPI: %v", err)
			return
		}
		user, err := db.UserGetByEmail(email)

		if r.Method == "GET" {
			ff, err := db.FormTemplateGetAll(user.ID)
			if err != nil {
				log.Printf("FormTemplsGetAPI: error getting formtemplates from db, %v", err)
			}
			for i := range ff {
				fts = append(fts, FTJson{
					ID:   int(ff[i].ID),
					Name: ff[i].Name,
				})
			}
			// Set content type
			w.Header().Set("Content-Type", "application/json")

			// Encode and write JSON
			if err := json.NewEncoder(w).Encode(fts); err != nil {
				log.Printf("FormTemplsGetAPI: error encoding json, %v", err)
				http.Error(w, "Failed to encode JSON", http.StatusTeapot)
			}
		}
	}
}

// GenFormDefsDeltaOp represents a single operation in Quill's Delta format
type GenFormDefsDeltaOp struct {
	Insert     interface{}            `json:"insert"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// GenFormDefsDelta represents a Quill Delta object
type GenFormDefsDelta struct {
	Ops []GenFormDefsDeltaOp `json:"ops"`
}

func GenerateFormDefsAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		type DefJson struct {
			DefType     string `json:"type"`
			FormTemplID int64  `json:"id"`
		}
		type Response struct {
			Msg GenFormDefsDelta `json:"msg"`
		}

		w.Header().Set("Content-Type", "application/json")

		_, _, _, err := InitSession(w, r, cs, "/admin/login", true)
		if err != nil {
			log.Printf("info: session error: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		defer r.Body.Close()
		var dj DefJson
		if err := json.NewDecoder(r.Body).Decode(&dj); err != nil {
			log.Printf("GenerateFormDefsAPI: failed to decode JSON: %v", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		infopanel, notif := generateFormData(dj.FormTemplID, db)
		var res Response
		switch dj.DefType {
		case "notification":
			res = Response{Msg: notif}
		case "infopanel":
			res = Response{Msg: infopanel}
		default:
			res = Response{
				Msg: GenFormDefsDelta{
					Ops: []GenFormDefsDeltaOp{
						{Insert: fmt.Sprintf("unknown type: %s", dj.DefType)},
					},
				},
			}
			w.WriteHeader(http.StatusBadRequest)
		}

		if err := json.NewEncoder(w).Encode(res); err != nil {
			log.Printf("error: failed to encode response: %v", err)
		}
	}
}

// generateFormData generates Delta objects for sidebar/info panel (first returned struct)
// delta object is json object which quill editor expects
// mail notification (second struct)
// returned text is in Polish, as we currently need only this
func generateFormData(formTemplID int64, db *DB) (GenFormDefsDelta, GenFormDefsDelta) {
	sidebar := GenFormDefsDelta{Ops: []GenFormDefsDeltaOp{}}
	notification := GenFormDefsDelta{Ops: []GenFormDefsDeltaOp{}}

	ff, err := db.FormFieldGetAllForTmpl(formTemplID)
	if err != nil {
		log.Printf("generateFormData: error getting form data, %v", err)
		return GenFormDefsDelta{}, GenFormDefsDelta{}
	}

	// Sidebar Delta: Equivalent to <h2>Aktualny stan:</h2> Ilość deklaracji: <b>{{.Sum "forms"}}</b><br>
	sidebar.Ops = append(sidebar.Ops,
		GenFormDefsDeltaOp{
			Insert:     "Aktualny stan:",
			Attributes: map[string]interface{}{"header": 2},
		},
		GenFormDefsDeltaOp{Insert: "\n"},
		GenFormDefsDeltaOp{Insert: "Ilość deklaracji: "},
		GenFormDefsDeltaOp{
			Insert:     "{{.Sum \"forms\"}}",
			Attributes: map[string]interface{}{"bold": true},
		},
		GenFormDefsDeltaOp{Insert: "\n"},
	)

	//log.Printf("Fields from db: %+v\n", ff) //debug

	// Add dynamic fields for sidebar and notification
	for _, f := range ff {

		if strings.Contains(f.Name, "REQUIRED") {
			continue
		}

		SumFunc := ".Sum"

		// propose SumAlfa - the can use Sum also
		if f.Type == "checkbox-group" {
			SumFunc = ".SumAlfa"
		}

		sidebar.Ops = append(sidebar.Ops,
			GenFormDefsDeltaOp{Insert: f.Display + ": "},
			GenFormDefsDeltaOp{
				Insert:     fmt.Sprintf("{{%s \"%s\"}}/", SumFunc, f.Name), // {{.Sum FieldName}}
				Attributes: map[string]interface{}{"bold": true},
			},
			GenFormDefsDeltaOp{Insert: "\n"},
		)

		// Notification: Equivalent to Display: <b>{{.Field "name"}}</b><br>
		notification.Ops = append(notification.Ops, generateFormDataForNotif(f)...)
	}

	return sidebar, notification
}

func generateFormDataForNotif(f FormField) []GenFormDefsDeltaOp {
	var out []GenFormDefsDeltaOp

	switch f.Type {
	case "multiplierField":
		out = []GenFormDefsDeltaOp{
			GenFormDefsDeltaOp{Insert: f.Display + ": "},
			GenFormDefsDeltaOp{
				Insert:     fmt.Sprintf("{{.MAmmount \"%s\"}}", f.Name),
				Attributes: map[string]interface{}{"bold": true},
			},
			GenFormDefsDeltaOp{Insert: " biletów, cena: "},
			GenFormDefsDeltaOp{
				Insert:     fmt.Sprintf("{{.MTotal \"%s\"}}", f.Name),
				Attributes: map[string]interface{}{"bold": true},
			},
			GenFormDefsDeltaOp{Insert: " Kč"},
			GenFormDefsDeltaOp{Insert: "\n"},
		}
	default:
		out = []GenFormDefsDeltaOp{GenFormDefsDeltaOp{Insert: f.Display + ": "},
			GenFormDefsDeltaOp{
				Insert:     fmt.Sprintf("{{.Field \"%s\"}}", f.Name),
				Attributes: map[string]interface{}{"bold": true},
			},
			GenFormDefsDeltaOp{Insert: "\n"},
		}
	}
	return out

}

type FormRendererVars struct {
	LBLLang          string
	ImgBanner        string
	ImgBannerRootDir string
	LBLTitle         string
	UniqIDTitle      string
	UniqID           string // used for anonymous
	LBLHowTo         template.HTML
	FormDataVal      template.JS
	FormInfoPanel    template.HTML
	BTNSave          string
}

type FormFuncs struct {
	DB          *DB
	User        User
	FormID      int64
	Template    FormTemplate
	NameSurname string
	// EmbeddedImgs contains all images embedded in notifications
	// this is the way for method to let caller know, that it have
	// to include data for cid image(-s)
	EmbeddedImgs []EmbImg
}

// Sum returns sum of given form field.
// form field can be specified by name or display value,
// where name is always checked first.
// This function is forms specific!
func (i *FormFuncs) Sum(FormFieldName string) template.HTML {
	return i.sum(FormFieldName, false, false)
}

// SumAlfa sorts results by name (not by ammount) in case of string values
func (i *FormFuncs) SumAlfa(FormFieldName string) template.HTML {
	return i.sum(FormFieldName, true, false)
}

// SumStrings forces string values (even when they are numbers) and sorts by string name
func (i *FormFuncs) SumStrings(FormFieldName string) template.HTML {
	return i.sum(FormFieldName, true, true)
}

func (i *FormFuncs) sum(FormFieldName string, sortByName bool, forceStringVals bool) template.HTML {
	var sum int64

	// special value "forms" - counts how many forms are filled
	if strings.ToLower(FormFieldName) == "forms" {
		amm, err := i.DB.FormGetAmmount(i.User.ID, i.Template.ID)
		if err != nil {
			log.Printf("Sum: error getting ammount of forms for user %q, formTemplID %d, %v", i.User.URL, i.Template.ID, err)
		}
		return template.HTML(strconv.FormatInt(amm, 10))
	}

	// retrieve formfield id
	id, err := i.DB.FormFieldGetIDByName(FormFieldName, i.Template.ID)
	if err != nil {
		log.Printf("searching for display: %s", FormFieldName) //debug
		id, err = i.DB.FormFieldGetIDByDisplay(FormFieldName, i.Template.ID)
	}

	if !forceStringVals { // if strings are forced, skip int retrieval
		// try to get ints from db, if can not do that assume strings
		nrs, err := i.DB.FormAnswerGetAllAnswersForFieldInts(id)
		//log.Printf("field id: %v, nrs: %v", id, nrs)
		if err != nil {
			log.Printf("Sum: can not get data for field %s(id:%d), %v", FormFieldName, id, err)
		} else { // there are numbers, so return ordinary sum
			for i := range nrs {
				sum += nrs[i]
			}
			return template.HTML(strconv.FormatInt(sum, 10))
		}
	}

	// Let's sum string answers, we are counting the same string separated by ', ' across
	// all answers in given field
	strs, err := i.DB.FormAnswerGetAllAnswersForFieldStrings(id)
	type kv struct {
		K string
		V int
	}
	m := map[string]int{}
	for i := range strs {
		ss := strings.Split(strs[i], ", ")
		for n := range ss {
			val, ok := m[ss[n]]
			if ok {
				m[ss[n]] = val + 1
			} else {
				m[ss[n]] = 1
			}
		}
	}

	// sorting
	kvs := []kv{}
	for k, v := range m {
		kvs = append(kvs, kv{k, v})
	}
	if !sortByName {
		// let's sort result by ammount
		sort.Slice(kvs, func(i, j int) bool {
			return kvs[i].V > kvs[j].V
		})
	} else { // sort by key name
		sort.Slice(kvs, func(i, j int) bool {
			return kvs[i].K < kvs[j].K
		})
	}

	out := "<ul class=\"sum-list\">"
	for _, kv := range kvs { // format strings for new line - I do not like it hardcoded
		out += fmt.Sprintf("<li class=\"sum-list-el\">%s: <b>%d</b></li>", kv.K, kv.V)
	}
	out += "</ul>"
	return template.HTML(out)
}

// Field shows form field value as is in db.
// This function is forms specific!
func (i *FormFuncs) Field(FormFieldName string) string {
	FieldID, err := i.DB.FormFieldGetIDByName(FormFieldName, i.Template.ID)
	if err != nil {
		FieldID, err = i.DB.FormFieldGetIDByDisplay(FormFieldName, i.Template.ID)
	}

	log.Println("field id:", FieldID)
	s, err := i.DB.FormAnswerGetByField(i.FormID, FieldID)
	if err != nil {
		log.Printf("Field: can not get data for field %s(id:%d), formID: %d, %v", FormFieldName, FieldID, i.FormID, err)
	}

	return s
}

// this functions allows to get separated data from multiplicateField (3 * 500 = 1500)
func (i *FormFuncs) MAmmount(FormFieldName string) string {
	s := i.Field(FormFieldName)
	amm, _, _ := ConvertMultiplicationToAmmountMultiplTotal(s)
	return amm
}

func (i *FormFuncs) MMultipl(FormFieldName string) string {
	s := i.Field(FormFieldName)
	_, mult, _ := ConvertMultiplicationToAmmountMultiplTotal(s)
	return mult
}

func (i *FormFuncs) MTotal(FormFieldName string) string {
	s := i.Field(FormFieldName)
	_, _, total := ConvertMultiplicationToAmmountMultiplTotal(s)
	return total
}

func (i *FormFuncs) generateQRimg(accountName string) (string, string) {
	return GenerateQRimg(accountName, i.DB, i.User, i.Template, i.NameSurname, "", i.FormID)
}

func (i *FormFuncs) QRPay(accountName string) template.HTML {
	_, imgpath := i.generateQRimg(accountName)
	return template.HTML(fmt.Sprintf(`<img width="200" src="/%s" alt="QRError">`, imgpath))
}

func (i *FormFuncs) QRPayMail(accountName string) template.HTML {
	imgname, imgpath := i.generateQRimg(accountName)
	i.EmbeddedImgs = append(i.EmbeddedImgs, EmbImg{
		NamePath: imgpath,
		CID:      getCID(imgname, i.User.URL),
	})
	return template.HTML(fmt.Sprintf(`<img width="150" src="cid:%s" alt="QR Kod" />`, getCID(imgname, i.User.URL)))
}

type OrderFuncs struct {
	EventID             int64
	TotalPrice          string
	Sits, Prices, Rooms string
	Email               string
	Password            string
	Name, Surname       string
	Phone, Notes        string

	DB           *DB
	User         User
	NameSurname  string
	EmbeddedImgs []EmbImg
}

func (o *OrderFuncs) generateQRimg(accountName string) (string, string) {
	etp := FormTemplate{}
	return GenerateQRimg(accountName, o.DB, o.User, etp, o.NameSurname, o.TotalPrice, -1)
}

func (o *OrderFuncs) QRPay(accountName string) template.HTML {
	_, imgpath := o.generateQRimg(accountName)
	return template.HTML(fmt.Sprintf(`<img width="200" src="/%s" alt="QRError">`, imgpath))
}

func (o *OrderFuncs) QRPayMail(accountName string) template.HTML {
	imgname, imgpath := o.generateQRimg(accountName)
	o.EmbeddedImgs = append(o.EmbeddedImgs, EmbImg{
		NamePath: imgpath,
		CID:      getCID(imgname, o.User.URL),
	})
	return template.HTML(fmt.Sprintf(`<img width="150" src="cid:%s" alt="QR Kod" />`, getCID(imgname, o.User.URL)))
}

func GenerateQRimg(accountName string, db *DB, u User, t FormTemplate, NameSurname, TotalPrice string, FormID int64) (string, string) {
	ba, err := db.BankAccountGetByName(accountName, u.ID)
	if err != nil {
		log.Printf("GenerateQRimg: no account with this name found, name given by admin: %q, userID: %d(%s), %v", accountName, u.ID, u.URL, err)
	}

	templURL := strings.Join(strings.Fields(t.URL), "")
	imgname := fmt.Sprintf("%s.jpeg", templURL+"_"+NameSurname)
	imgpath := getImgPath(MEDIAROOT, u.URL, MEDIAQRCODESUBDIR, imgname)

	eft := FormTemplate{}
	am := ""
	// get money ammount
	if t != eft { // it is form
		am, err = db.FormAnswerGetByFieldName(FormID, t.ID, t.MoneyAmountFieldName.String)
		if err != nil {
			am, err = db.FormAnswerGetByFieldDisplay(FormID, t.ID, t.MoneyAmountFieldName.String)
			if err != nil {
				log.Printf("GenerateQRimg: problem getting money ammount field name and display %q, FormID %d, %v", t.MoneyAmountFieldName.String, FormID, err)
			}
		}
		// check and convert to sum if is from multiplicateField (it will be "3 * 500 = 1500")
		_, _, am = ConvertMultiplicationToAmmountMultiplTotal(am)
	} else {
		// this is event
		ss := strings.Split(TotalPrice, " ")
		if len(ss) == 2 {
			am = ss[0]
		} else {
			am = TotalPrice
		}
	}

	ami, err := strconv.ParseInt(am, 10, 64)
	if err != nil {
		log.Printf("GenerateQRimg: error converting money amount to int64 %q, %v", am, err)
	}
	qr := NewQRPayment(ba.IBAN, ba.RecipientName.String, float64(ami), ba.Currency, ba.Message.String+"_"+NameSurname, ba.VarSymbol.Int64)
	log.Println(ba)
	err = qr.GenCode(imgpath)
	if err != nil {
		log.Printf("GenerateQRimg: error generating qr image, %v", err)
	}
	return imgname, imgpath
}

func ConvertMultiplicationToAmmountMultiplTotal(s string) (string, string, string) {
	sum := ""
	ammount := ""
	multiplier := ""

	// if string doesn't contains "=" then it is ordinary ammount
	if strings.Contains(s, "=") {
		ss := strings.Split(s, "=")
		sum = strings.TrimSpace(ss[len(ss)-1])
		if strings.Contains(ss[0], "*") {
			sm := strings.Split(ss[0], "*")
			ammount = strings.TrimSpace(sm[0])
			multiplier = strings.TrimSpace(sm[1])
		} else {
			log.Printf("ConvertMultiplicationToAmmountMultiplSum: no '*' symbol in %q", s)
		}
	} else {
		return "", "", s // returning as is
	}
	return ammount, multiplier, sum
}

func FormRenderer(db *DB, lang string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var parsedInfoPanel bytes.Buffer
		var uniqID string

		user, templ, err := getOrganizationAndForm(w, r, db, lang)
		if err != nil {
			return // just end function, errors had been sent
		}

		infp := &FormFuncs{DB: db, User: user, Template: templ}

		tmpl, err := template.New("infopanel").Parse(templ.InfoPanel.String)
		if err != nil {
			log.Printf("FormRenderer: error parsing InfoPanel template, %v", err)
		}
		err = tmpl.ExecuteTemplate(&parsedInfoPanel, "infopanel", infp)
		if err != nil {
			log.Printf("FormRenderer: error executing InfoPanel template, %v", err)
		}

		if !strings.Contains(templ.Content.String, "surname-REQUIRED") {
			// this is anonymous form
			b := make([]byte, 4) //equals 8 characters
			rand.Read(b)
			uniqID = hex.EncodeToString(b)
		}

		log.Println(parsedInfoPanel.String()) // DEBUG

		rootpath := path.Join("/", MEDIAROOT, user.URL)

		// now actuall rendering

		pPL := FormRendererVars{
			LBLLang:          lang,
			LBLTitle:         templ.Name,
			ImgBannerRootDir: rootpath,
			ImgBanner:        templ.Banner.String,
			UniqIDTitle:      "Unikatowe ID formularza: ",
			UniqID:           uniqID,
			LBLHowTo:         template.HTML(templ.HowTo.String),
			FormDataVal:      template.JS(templ.Content.String),
			FormInfoPanel:    template.HTML(reallyEmpty(parsedInfoPanel.String())),
			BTNSave:          "Wyślij!",
		}

		t := template.Must(template.ParseFiles("tmpl/form_renderer.html", "tmpl/base.html"))
		err = t.ExecuteTemplate(w, "base", pPL)
		if err != nil {
			log.Print("ErrorHTML: template executing error: ", err) //log it
		}
	}
}

type ThankYouVars struct {
	LBLLang       string
	LBLTitle      string
	LBLStatus     string
	HiddenFormID  string
	HiddenName    string
	HiddenSurname string
	LBLStatusText template.HTML
	BTNOk         string
}

type ThankYou struct {
	FormFuncs
}

func FormThankYou(db *DB, lang string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var parsedThankYou bytes.Buffer
		user, templ, err := getOrganizationAndForm(w, r, db, lang)
		if err != nil {
			return
		}
		// FormID is POSTed by js in form_renderer
		err = r.ParseForm()
		if err != nil {
			log.Printf("FormThankYou: problem parsing form data, err: %v", err)
		}
		formID, err := strconv.ParseInt(r.FormValue("formID"), 10, 64)
		if err != nil {
			log.Printf("FormThankYou: problem getting formID posted by js in form_renderer.js, %v", err)
		}
		name := r.FormValue("name")
		surname := r.FormValue("surname")

		nsn := surname + "_" + name

		thp := &FormFuncs{DB: db, User: user, Template: templ, FormID: formID, NameSurname: nsn}

		tmpl, err := template.New("thankyou").Parse(templ.ThankYou.String)
		if err != nil {
			log.Printf("FormThankYou: error parsing thankyou(qrcode) template, %v", err)
		}
		err = tmpl.ExecuteTemplate(&parsedThankYou, "thankyou", thp)
		if err != nil {
			log.Printf("FormThankYou: error executing ThankYou template, %v", err)
		}

		log.Println(parsedThankYou.String())

		p := ThankYouVars{
			LBLLang:       lang,
			LBLTitle:      "Dziękujemy za wypełnienie formularza!",
			LBLStatus:     "",
			HiddenFormID:  strconv.FormatInt(formID, 10),
			HiddenName:    name,
			HiddenSurname: surname,
			LBLStatusText: template.HTML(parsedThankYou.String()),
			BTNOk:         "OK",
		}

		t := template.Must(template.ParseFiles("tmpl/order-status.html", "tmpl/base.html"))
		err = t.ExecuteTemplate(w, "base", p)
		if err != nil {
			log.Print("Form thankyou template executing error: ", err)
		}
	}
}

func getOrganizationAndForm(w http.ResponseWriter, r *http.Request, db *DB, lang string) (User, FormTemplate, error) {
	// get vars from url (from mux)
	userURL := mux.Vars(r)["userurl"]
	formtemplURL := mux.Vars(r)["formurl"]

	// validate url
	user, err := db.UserGetByURL(userURL)
	if err != nil {
		log.Printf("FormRenderer: error getting user(wrong link or user in url), %v", err)
		plErr := map[string]string{
			"title": "Nie znaleziono organizacji!",
			"text":  fmt.Sprintf("W bazie nie istnieje organizacja: %q\nProszę sprawdzić poprawność linka.\nJeżeli organizator twierdzi, że jest ok, to proszę o kontakt pod: admin (at) zori.cz.", userURL),
		}
		ErrorHTML(plErr["title"], plErr["text"], lang, w, r)
		return user, FormTemplate{}, err
	}
	templ, err := db.FormTemplateGetByURL(formtemplURL, user.ID)
	if err != nil {
		log.Printf("FormRenderer: error getting form(wrong link or formtemplurl in url), %v", err)
		plErr := map[string]string{
			"title": "Nie znaleziono formularza!",
			"text":  fmt.Sprintf("W bazie nie istnieje taki formularz: %q\nProszę sprawdzić poprawność linka.\nJeżeli organizator twierdzi, że jest ok, to proszę o kontakt pod: admin (at) zori.cz.", formtemplURL),
		}
		ErrorHTML(plErr["title"], plErr["text"], lang, w, r)
		return user, templ, err
	}
	return user, templ, nil
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

type FormChangeStatusMsg struct {
	FormID       int64  `json:"form_id"`
	FormAnswerID int64  `json:"form_answer_id"`
	Status       string `json:"status"`
}

// it is called from form raport, allows to change status in forms table
func FormChangeStatusAPI(db *DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var m FormChangeStatusMsg
		if r.Method == "POST" {
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&m)
			if err != nil {
				log.Printf("error: FormChangeStatusAPI: problem decoding json, err: %v", err)
			}
			fmt.Printf("%+v", m) //debug
			// TODO: add timestamp for status change
			timestamp := int64(0)
			if m.Status == "payed" {
				timestamp = time.Now().Unix()
			}
			_ = timestamp

			//if err := db.(m.Status, timestamp); err != nil {
			//		log.Printf("error: ReservationChangeStatusAPI: problem updating status to %q for FurnID: %d, EventID: %d, err: %v", m.Status, chair.ID, m.EventID, err)
			//	}
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

func ParseOrderTmpl(t string, o Order, db *DB, user User) (string, []EmbImg) {
	var buf bytes.Buffer

	nsn := o.Surname + "_" + o.Name

	of := &OrderFuncs{
		EventID:     o.EventID,
		TotalPrice:  o.TotalPrice,
		Sits:        o.Sits,
		Prices:      o.Prices,
		Rooms:       o.Rooms,
		Email:       o.Email,
		Password:    o.Password,
		Name:        o.Name,
		Surname:     o.Surname,
		Phone:       o.Phone,
		Notes:       o.Notes,
		DB:          db,
		User:        user,
		NameSurname: nsn,
	}

	tmpl, err := template.New("ordermail").Parse(t)
	if err != nil {
		log.Printf("error parsing template %q, OrderFuncs %+v, err: %v", t, of, err)
		return t, []EmbImg{}
	}
	err = tmpl.Execute(&buf, of)
	if err != nil {
		log.Printf("error executing template %q, OrderFuncs %+v, err: %v", t, of, err)
		return t, []EmbImg{}
	}
	return buf.String(), of.EmbeddedImgs
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

func ToNB(b bool) sql.NullBool {
	return sql.NullBool{
		Bool:  b,
		Valid: true,
	}
}

func ToDate(unix int64) string {
	t := time.Unix(unix, 0)
	return t.Format("2006-01-02")
}

func ToDateTime(unix int64) string {
	t := time.Unix(unix, 0)
	return t.Format("2006-01-02 15:04")
}

func ToUnix(s string, loc *time.Location, df string) (int64, error) {
	t, err := time.Parse(df, s)
	//t, err := time.ParseInLocation(df, s, loc) // we can not parse in location, other direction is parsed normaly
	if err == nil {
		return t.Unix(), nil
	}
	return -1, err
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

// getAttachments looks for ';' and splits
// filenames into slice, then it adds full path
// to the user (organization) media path
func getAttachments(atts, userDirPath, emailsubdir, mediaRootPath string) []string {
	fullPathAtts := []string{}
	if atts == "" {
		return fullPathAtts
	}
	ss := strings.Split(atts, ";")
	for i := range ss {
		fullPathAtts = append(fullPathAtts,
			fmt.Sprintf("%s/%s/%s/%s", mediaRootPath, userDirPath, emailsubdir, ss[i]))
	}
	return fullPathAtts
}

func getEmbeddedImgs(imgs, userDirPath, emailsubdir, mediaRootPath string) []EmbImg {
	//embs := getAttachments(event.MailEmbeddedImgsDelimited.String, user.URL+"/"+emailsubdir, MEDIAROOT)

	log.Println("imgs:", imgs) //debug
	if imgs == "" {
		return []EmbImg{}
	}

	ss := strings.Split(imgs, ";")
	log.Println("ss:", ss) //debug
	var ei = make([]EmbImg, len(ss))
	for i := range ss {
		ei[i] = EmbImg{
			NamePath: getImgPath(mediaRootPath, userDirPath, emailsubdir, ss[i]),
			CID:      getCID(ss[i], userDirPath),
		}
	}
	log.Println("ei:", ei) //debug
	return ei
}

func getCID(imgname, userURL string) string {
	return fmt.Sprintf("%s@%s.cz", removeNonAlphanumeric(imgname), userURL)
}

func getImgPath(mediaroot, userdir, subdir, imgname string) string {
	return fmt.Sprintf("%s/%s/%s/%s", mediaroot, userdir, subdir, imgname)
}

func removeNonAlphanumeric(str string) string {
	var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	return nonAlphanumericRegex.ReplaceAllString(str, "")
}

// chooseEmail - use main mail if alt_email is not defined
func chooseEmail(main, alt string) string {
	if alt != "" {
		return alt
	}
	return main
}

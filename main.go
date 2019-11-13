package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

func main() {

	loc, err := time.LoadLocation("Europe/Prague")
	if err != nil {
		log.Println(err)
	}
	dateFormat := "2006-01-02"

	//roomName := "Sala główna - parter"
	//roomName := "Sala na dole"
	roomNameB := "Balkon - 1. piętro"
	_ = roomNameB
	eventName := "Bal MS Karwina"

	mailpassf, err := ioutil.ReadFile(".mailpass")
	if err != nil {
		log.Fatalf("can not read mail password from .mailpass file, err: %v", err)
	}
	mailfsplit := strings.Split(string(mailpassf), "\n")
	if len(mailfsplit) < 2 {
		log.Fatal("missing second line (mail server name/address) in .mailpass file!")
	}
	mailpass := mailfsplit[0]
	mailserv := mailfsplit[1]

	db := initDB()
	defer db.Close()

	rtr := mux.NewRouter()
	rtr.HandleFunc("/res/{user}", ReservationHTML(db))
	rtr.HandleFunc("/", AboutHTML())

	handleStatic("js")
	handleStatic("css")
	http.Handle("/", rtr)
	//http.HandleFunc("/reservation", ReservationHTML(db, roomName, eventName))
	http.HandleFunc("/order", ReservationOrderHTML(db, eventName))
	http.HandleFunc("/order/status", ReservationOrderStatusHTML(db, eventName, strings.TrimSpace(mailpass), strings.TrimSpace(mailserv)))
	http.HandleFunc("/admin", AdminMainPage(db, loc, dateFormat))
	http.HandleFunc("/admin/designer", DesignerHTML(db, roomNameB, eventName))
	http.HandleFunc("/admin/event", EventEditor(db))
	http.HandleFunc("/api/room", DesignerSetRoomSize(db))
	http.HandleFunc("/api/furnit", DesignerMoveObject(db))
	http.HandleFunc("/api/furdel", DesignerDeleteObject(db))
	http.HandleFunc("/api/ordercancel", OrderCancel(db))
	http.HandleFunc("/api/renumber", DesignerRenumberType(db))

	log.Fatal(http.ListenAndServe(":3002", nil))
}

func initDB() *DB {
	howto := `<h1>Legenda:</h1>
	<ul>
		<li><span class="free-text">Zielony</span> = możliwa rezerwacja.</li>
		<li><span class="marked-text">Żółty</span> = ktoś wybrał miejsce, ale jeszcze nie dokonał rezerwacji.
		<li><span class="ordered-text">Pomarańczowy</span> = zarezerowane.</li>
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

	mailText := `Szanowni Państwo,
dziękujemy za dokonanie rezerwacji biletów na Bal Macierzy Szkolnej przy PSP w Karwinie-Frysztacie.
Niniejszym mailem potwierdzamy zamówienie miejsc: {{.Sits}}.
Łączna kwota biletów wynosi: {{.TotalPrice}}.
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
	roomDescription := `Koło Macierzy Szkolnej zaprasza wszystkich na bal pt. <b>„ROZTAŃCZMY PRZYJAŹŃ …”</b>,<br />
który odbędzie się w piątek <b>7 lutego 2020</b> od godziny 19:00 w Domu Przyjaźni w Karwinie.<br />
W celu zakupu biletów potrzebna jest wcześniejsza rezerwacja.<br />
W górnej części ekranu wybrać można zakładkę <b>"Sala główna - parter"</b> lub <b>"Balkon - 1. piętro"</b>.<br />
Proszę wybrać wolne miejsca (krzesła) i kliknąć na przycisk "Zamów", które przekieruje Państwa do formularza rezerwacji.`
	orderHowto := `W celu dokonania rezerwacji prosimy o wypełnienie poniższych danych. W przypadku kiedy Państwo dokonują rezerwacji większej ilości biletów, prosimy o podanie nazwisk osób, dla których są miejsca przeznaczone (wystarczy 1 nazwisko na 2 bilety).
Na podany przez Państwa mail zostanie wysłany mail z potwierdzeniem rezerwacji oraz z informacją na temat zakupu biletów.`
	orderedNote := `Na podany przez Państwa mail zostanie wysłany mail z potwierdzeniem rezerwacji oraz informacja na temat zakupu biletów.`

	db := DBInit("db.sql")
	db.MustConnect()

	db.StructureCreate()

	uID, err := db.UserAdd(&User{ID: 1, Email: "pspmacierzkarwina@seznam.cz", URL: "mskarwina", Passwd: "MagikINFO2019"})
	if err != nil {
		log.Println(err)
	}

	r1ID, err := db.RoomAdd(&Room{ID: 1, Name: "Sala główna - parter", Description: ToNS(roomDescription), Width: 1000, Height: 1000})
	if err != nil {
		log.Println(err)
	}
	r2ID, err := db.RoomAdd(&Room{ID: 2, Name: "Balkon - 1. piętro", Description: ToNS(roomDescription), Width: 500, Height: 500})
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
	eID, err := db.EventAdd(&Event{ID: 1, Name: "Bal MS Karwina", Date: 1581033600, FromDate: 1572998400, ToDate: 1580860800, DefaultPrice: 400, DefaultCurrency: "Kč", OrderHowto: orderHowto, OrderNotesDescription: "Prosimy o podanie nazwisk wszystkich rodzin, dla których przeznaczone są bilety.", OrderedNote: orderedNote, MailSubject: "Rezerwacja biletów na Bal Macierzy", MailText: mailText, HowTo: howto, UserID: 1})
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

	return db
}

func handleStatic(dir string) {
	fs := http.FileServer(http.Dir(dir))
	http.Handle("/"+dir+"/", http.StripPrefix("/"+dir+"/", fs))
}

type DesignerPage struct {
	TableNr, ChairNr, ObjectNr, LabelNr  int64
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
	LBLTitle string
	DesignerPage
	ReservationPage
	MainAdminPage
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
	LBLTitle string
	Event
	Rooms []RoomVars
}

type RoomVars struct {
	Room
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

func DesignerHTML(db *DB, roomName, eventName string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID := int64(-1)
		if r.Method == "POST" {
			err := r.ParseForm()
			if err != nil {
				log.Printf("DesignerHTML: error parsing form, err: %v", err)
			}
			roomID, err = strconv.ParseInt(r.FormValue("rooms-select"), 10, 64)
			if err != nil {
				log.Printf("error converting roomID to int, err: %v", err)
			}
		} else {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
			//return
		}

		p := GetPageVarsFromDB(db, roomID, eventName)
		//log.Printf("%+v", p)
		enPM := PageMeta{
			LBLTitle: "Designer",
			DesignerPage: DesignerPage{
				TableNr:             TableNr(p.Tables) + 1,
				ChairNr:             ChairNrFull(p.Chairs) + 1,
				ObjectNr:            ObjectNr(p.Objects) + 1,
				LabelNr:             LabelNr(p.Labels) + 1,
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

func ReservationHTML(db *DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		v := mux.Vars(r)
		u, err := db.UserGetByURL(v["user"])

		plErr := map[string]string{
			"title": "Nie znaleziono organizacji!",
			"text":  "W bazie nie istnieje organizacja: %q\nProszę sprawdzić poprawność linka.\nJeżeli organizator twierdzi, że jest ok, to proszę o kontakt pod: admin (at) zori.cz.",
		}

		if err != nil {
			log.Printf("error getting user %q, err: %v", v["user"], err)

			ErrorHTML(plErr["title"], plErr["text"], w, r)
			//http.Error(w, fmt.Sprintf("User %q not found! Check You have correct URL!", v["user"]), 500)
			return
		}
		e, err := EventGetCurrent(db, u.ID)
		if err != nil {
			log.Printf("error getting current event for userID: %d, err: %v", u.ID, err)
			ErrorHTML("Nie obecnie aktywnych imprez!", "Administrator nie obecnie żadnych otwartych imprez.\nProsimy o skontaktowanie się z organizatorem by stwierdzić, kiedy rezerwacje zostaną otwarte.", w, r)
			//http.Error(w, "User have no active events! Come back later, when reservations will be opened!", 500) //TODO: inform about closest user event and when it is
			return
		}
		rr, err := db.EventGetRooms(e.ID)
		if err != nil {
			log.Printf("error getting rooms for eventID: %d, err: %v", e.ID, err)
			plErr := map[string]string{
				"title": "Brak aktywnych sal dla tej imprezy!",
				"text":  "Administrator nie powiązał żadnej sali z wydarzeniem.\nJeżeli po stronie administracji wszystko wygląda ok, to prosimy o informację o tym zdarzeniu na mail: admin (at) zori.cz.\nProsimy o wysłanie nazwy organizacji, której dotyczy problem.",
			}
			ErrorHTML(plErr["title"], plErr["text"], w, r)
			//http.Error(w, fmt.Sprintf("Rooms for user: %q, event: %q not found!", v["user"], e.Name), 500)
			return
		}
		p := ReservationPageVars{
			//EN: LBLTitle: "Reservation",
			LBLTitle: "Rezerwacja",
			Event:    e,
			Rooms:    []RoomVars{},
		}
		for i := range rr {
			// TODO: remake it, GetPageVarsFromDB call the same again, move previous lines there
			rv := GetFurnituresFromDB(db, rr[i].Name, e.ID)
			rv.Room = rr[i]
			rv.HTMLRoomDescription = template.HTML(rr[i].Description.String)
			rv.HTMLHowTo = template.HTML(e.HowTo)
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

type ReservationOrderStatusVars struct {
	LBLTitle                 string
	LBLStatus, LBLStatusText string
	BTNOk                    string
}

func ReservationOrderStatusHTML(db *DB, eventName, mailpass, mailsrv string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		o := Order{}
		event, err := db.EventGetByName(eventName)
		if err != nil {
			log.Printf("error getting event by name: %q, err: %v", eventName, err)
		}
		// TODO: this will be different
		user, err := db.UserGetByID(event.UserID)
		if err != nil {
			log.Println(err)
		}

		if r.Method == "POST" {
			err := r.ParseForm()
			if err != nil {
				log.Printf("error parsing form data:, err: %v", err)
			}
			o.Sits = r.FormValue("sits")
			o.Prices = r.FormValue("prices")
			o.Rooms = r.FormValue("rooms")
			o.TotalPrice = r.FormValue("total-price")
			o.Email = r.FormValue("email")
			o.Password = r.FormValue("password")
			o.Name = r.FormValue("name")
			o.Surname = r.FormValue("surname")
			o.Phone = r.FormValue("phone")
			o.Notes = r.FormValue("notes")

			fmt.Printf("%+v", o)
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
				c, err = db.CustomerGetByEmail(o.Email)
				if err != nil {
					log.Printf("error getting user by mail %q, err: %v", o.Email, err)
				}
				cID = c.ID
				fmt.Printf("printing client, maybe? %+v", c)
				//log.Printf("stare: %v, nowe: %v", c.Passwd.String, o.Password)
				if c.Passwd.String == "" || c.Passwd.String != o.Password {
					plErr := map[string]string{
						"title": "Nie można zapisać osoby!",
						"text":  "W bazie już istnieje zamówienie powiązane z tym mailem, lecz nie zgadza się hasło.\nProsimy podać poprawne hasło.",
					}
					ErrorHTML(plErr["title"], plErr["text"], w, r)
					//http.Error(w, fmt.Sprintf("<html><body><b>Can not add customer: %+v, err: %v</b></body></html>", c, err), 500)
					return
				}
			}
			// password is correct, so continue
			fmt.Println("we survived till here!!!")
			//TODO: this will fail for returning user, maybe check if exist and do not insert?
			err = db.CustomerAppendToUser(user.ID, cID)
			if err != nil {
				log.Println(err)
			}
			log.Println("before Splitting")
			ss, rr, err := SplitSitsRooms(o.Sits, o.Rooms)
			if err != nil {
				log.Printf("ReservationOrderStatusHTML: %v", err)
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

				reservation, err := db.ReservationGet(chair.ID, event.ID)
				if err != nil {
					log.Printf("error retrieving reservation for chair: %d, eventID: %d, err: %v", chair.ID, event.ID, err)
				}
				reservation.Status = "ordered"
				reservation.CustomerID = cID
				if noteID != 0 {
					reservation.NoteID = ToNI(noteID)
				}
				log.Printf("debug: noteID: %v", noteID)
				err = db.ReservationMod(&reservation)
				if err != nil {
					log.Printf("error modyfing reservation for chair: %d, eventID: %d, err: %v", chair.ID, event.ID, err)
					plErr := map[string]string{
						"title": "Nie można zmienić stutusu zamówienia!",
						"text":  "Wystąpił problem ze zmianą stutusu zamówienia, przepraszamy za kłopot i prosimy o informację na\nmail: admin (at) zori.cz.\nProsimy o przesłanie info: email zamawiającego, numery zamawianych siedzień, nazwa sali/imprezy.",
					}
					ErrorHTML(plErr["title"], plErr["text"], w, r)
					//http.Error(w, fmt.Sprintf("Can not update reservation for chair: %d, eventID: %d, err: %v", chair.ID, event.ID, err), 500)
					return
				}
			}

			custMail := MailConfig{
				Server:     mailsrv,
				Port:       587,
				User:       "rezerwo@zori.cz",
				Pass:       mailpass,
				From:       user.Email,
				ReplyTo:    user.Email,
				Sender:     "rezerwo@zori.cz",
				To:         []string{o.Email},
				Subject:    event.MailSubject,
				Text:       ParseTmpl(event.MailText, o),
				IgnoreCert: true,
			}
			err = MailSend(custMail)
			if err != nil {
				log.Println(err)
			}
			userMail := MailConfig{
				Server:     mailsrv,
				Port:       587,
				User:       "rezerwo@zori.cz",
				Pass:       mailpass,
				From:       "rezerwo@zori.cz",
				ReplyTo:    "rezerwo@zori.cz",
				Sender:     "rezerwo@zori.cz",
				To:         []string{user.Email},
				Subject:    event.MailSubject,
				Text:       ParseTmpl("{{.Name}} {{.Surname}}\nkrzesła: {{.Sits}}\nŁączna cena: {{.TotalPrice}}\nEmail: {{.Email}}\nTel: {{.Phone}}\nNotatki:{{.Notes}}", o), //TODO
				IgnoreCert: true,
			}
			err = MailSend(userMail)
			if err != nil {
				log.Println(err)
			}
		}

		pEN := ReservationOrderStatusVars{
			LBLTitle:      "Order status",
			LBLStatus:     "Tickets for " + event.Name + " ordered!",
			LBLStatusText: event.OrderedNote,
			BTNOk:         "OK",
		}
		_ = pEN

		p := ReservationOrderStatusVars{
			LBLTitle:      "Zamówiono bilety!",
			LBLStatus:     "Dziękujemy za dokonanie rezerwacji biletów.",
			LBLStatusText: event.OrderedNote,
			BTNOk:         "OK",
		}

		t := template.Must(template.ParseFiles("tmpl/order-status.html", "tmpl/base.html"))
		err = t.ExecuteTemplate(w, "base", p)
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
func GetPageVarsFromDB(db *DB, roomID int64, eventName string) Page {
	room, err := db.RoomGetByID(roomID)
	if err != nil {
		log.Printf("error getting room by ID %d, err: %v", roomID, err)
	}
	event, err := db.EventGetByName(eventName)
	if err != nil {
		log.Printf("error getting event by name: %q, err: %v", eventName, err)
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
	LBLTitle                            string
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
	LBLTotalPrice, LBLTotalPriceValue   string
	BTNSubmit, BTNCancel                string
}

func ReservationOrderHTML(db *DB, eventName string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sits := ""
		prices := ""
		rooms := ""
		totalPrice := ""
		defaultCurrency := ""

		event, err := db.EventGetByName(eventName)
		if err != nil {
			log.Printf("ReservationOrderHTML: error getting event by name: %q, err: %v", eventName, err)
		}

		if r.Method == "POST" {
			err := r.ParseForm()
			if err != nil {
				log.Printf("ReservationOrderStatusHTML: error parsing form data:, err: %v", err)
			}
			sits = r.Form["sits"][0]
			prices = r.Form["prices"][0]
			rooms = r.Form["rooms"][0]
			totalPrice = r.Form["total-price"][0]
			defaultCurrency = r.Form["default-currency"][0]

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
					EventID:     event.ID,
					CustomerID:  -1, // this will be updated when customer is created
				})
				if err != nil {
					log.Printf("error adding reservation for chair: %d, eventID: %d, err: %v", chair.ID, event.ID, err)
					errPL := map[string]string{
						"title": "Nie udało się zarezerwować miejsca!",
						"text":  "Nie można zarezerwować wybranych miejsc, zostały one już zablokowane przez innego zamawiającego.\nProsimy o wybranie innych miejsc, lub poczekanie 5 minut. Po 5 minutach niezrealizowane zamówiania są automatycznie anulowane.",
					}
					ErrorHTML(errPL["title"], errPL["text"], w, r)
					//http.Error(w, fmt.Sprintf("<html><body><b>Can not add reservation for chair: %d, eventID: %d, err: %v</b></body></html>", chair.ID, event.ID, err), 500)
					return
				}
			}
		}
		//p := GetPageVarsFromDB(db, roomName, eventName)
		pEN := ReservationOrderVars{
			Event:                 event,
			LBLOrderHowto:         event.OrderHowto,
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
			Event:                 event,
			LBLOrderHowto:         event.OrderHowto,
			LBLTitle:              "Zamówienie",
			LBLEmail:              "Email",
			LBLEmailHelp:          "Na podany email zostanie wysłane potwierdzenie. Proszę sprawdzić, że podano poprawny!",
			LBLEmailPlaceholder:   "email",
			LBLPassword:           "Hasło",
			LBLPasswordHelp:       "Hasło jest opcjonalne, ale gorąco polecamy podanie go! Umożliwi to zmiany w zamówieniu oraz sprawdzenie stutusu zamówienia.",
			LBLName:               "Imię",
			LBLNamePlaceholder:    "imię",
			LBLSurname:            "Nazwisko",
			LBLSurnamePlaceholder: "nazwisko",
			LBLPhone:              "Nr telefonu",
			LBLPhonePlaceholder:   "00420 ",
			LBLNotes:              "Notatki",
			LBLNotesPlaceholder:   "notatki",
			LBLNotesHelp:          event.OrderNotesDescription,
			LBLPricesValue:        prices,
			LBLRoomsValue:         rooms,
			LBLSits:               "Numery krzeseł",
			LBLSitsValue:          sits,
			LBLTotalPrice:         "Łączna suma",
			LBLTotalPriceValue:    totalPrice + " " + defaultCurrency,
			BTNSubmit:             "Zamawiam",
			BTNCancel:             "Anuluj zamówienie",
		}

		t := template.Must(template.ParseFiles("tmpl/order.html", "tmpl/base.html"))
		err = t.ExecuteTemplate(w, "base", p)
		if err != nil {
			log.Print("Reservation template executing error: ", err)
		}
	}
}

type AdminPage struct {
	PageMeta
	Events     []Event
	Rooms      []Room
	Furnitures []Furniture
}

type MainAdminPage struct {
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
}

func AdminMainPage(db *DB, loc *time.Location, dateFormat string) func(w http.ResponseWriter, r *http.Request) {

	// TODO: get user from auth data
	userID := int64(1)

	return func(w http.ResponseWriter, r *http.Request) {

		dtype := ""
		// if event/room detail form sent some data,
		// save them to db and show result
		if r.Method == "POST" {
			err := r.ParseForm()
			if err != nil {
				log.Printf("error parsing form data:, err: %v", err)
			}
			dtype = r.Form["type"][0]
			if dtype == "event" {
				id, err := strconv.Atoi(r.Form["id"][0])
				if err != nil {
					log.Printf("problem converting %q to number, err: %v", r.Form["id"][0], err)
				}
				d, err := time.ParseInLocation(dateFormat, r.Form["date"][0], loc)
				if err != nil {
					log.Println(err)
				}

				fd, err := time.ParseInLocation(dateFormat, r.Form["from-date"][0], loc)
				if err != nil {
					log.Println(err)
				}
				td, err := time.ParseInLocation(dateFormat, r.Form["to-date"][0], loc)
				if err != nil {
					log.Println(err)
				}
				dp, err := strconv.Atoi(r.Form["default-price"][0])
				if err != nil {
					log.Println(err)
				}

				e := Event{
					ID: int64(id),
				}
				_ = e //TODO
				e.Name = r.Form["name"][0]
				e.Date = d.Unix()
				e.FromDate = fd.Unix()
				e.ToDate = td.Unix()
				e.DefaultPrice = int64(dp)
				e.DefaultCurrency = r.Form["default-currency"][0]
				e.MailSubject = r.Form["mail-subject"][0]
				e.AdminMailSubject = r.Form["admin-mail-subject"][0]
				e.AdminMailText = r.Form["admin-mail-text"][0]
				e.HowTo = r.Form["html-howto"][0]
				e.OrderedNote = r.Form["html-order-note"][0]

				log.Printf("%+v", e)
				org, _ := db.EventGetByID(e.ID)
				log.Printf("org: %+v", org)
				log.Println("test equal, is:", reflect.DeepEqual(e, org))
			}
		}

		// Prepare form with rooms and events
		rooms, err := db.RoomGetAllByUserID(userID)
		if err != nil {
			log.Printf("error getting all rooms, err: %q", err)
		}
		events, err := db.EventGetAllByUserID(userID)
		if err != nil {
			log.Printf("error getting event by name: %q, err: %v", "TODO", err)
		}
		enPM := PageMeta{
			LBLTitle: "Admin main page",
			MainAdminPage: MainAdminPage{
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
			},
		}

		rp := AdminPage{
			PageMeta: enPM,
			Events:   events,
			Rooms:    rooms,
		}
		t := template.Must(template.ParseFiles("tmpl/a_rooms.html", "tmpl/base.html"))
		err = t.ExecuteTemplate(w, "base", rp)
		if err != nil {
			log.Print("AdminMainPage template executing error: ", err)
		}
	}
}

type EventEditorVars struct {
	LBLTitle                                      string
	LBLID                                         string
	LBLIDValue                                    int64
	LBLName, LBLNameValue                         string
	NameHelpText                                  string
	LBLDate, LBLDateValue                         string
	LBLFromDate, LBLFromDateValue                 string
	LBLToDate, LBLToDateValue                     string
	LBLDefaultPrice                               string
	LBLDefaultPriceValue                          int64
	LBLDefaultCurrency, LBLDefaultCurrencyValue   string
	LBLMailSubject, LBLMailSubjectValue           string
	LBLMailText, LBLMailTextValue                 string
	LBLAdminMailSubject, LBLAdminMailSubjectValue string
	LBLAdminMailText, LBLAdminMailTextValue       string
	LBLOrderNote, LBLHowto                        string
	HTMLOrderNote, HTMLHowTo                      template.HTML
	BTNSave                                       string
	BTNCancel                                     string
}

func EventEditor(db *DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {

			err := r.ParseForm()
			if err != nil {
				log.Printf("EventEditor: problem parsing form data, err: %v", err)
			}
			eventID, err := strconv.Atoi(r.FormValue("events-select"))
			if err != nil {
				log.Printf("error converting eventID to int, err: %v", err)
			}
			event, err := db.EventGetByID(int64(eventID))
			if err != nil {
				log.Printf("error retrieving event with ID: %q from DB, err: %v", eventID, err)
			}

			rp := EventEditorVars{
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
				LBLOrderNote:             "After ordered note",
				HTMLOrderNote:            template.HTML(event.OrderedNote),
			}
			t := template.Must(template.ParseFiles("tmpl/a_event.html", "tmpl/base.html"))
			err = t.ExecuteTemplate(w, "base", rp)
			if err != nil {
				log.Print("AdminEventEditor template executing error: ", err)
			}
		}
	}
}

type AboutVars struct {
	LBLTitle                    string
	LBLAboutTitle, LBLAboutText template.HTML
}

func AboutHTML() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		pPL := AboutVars{
			LBLTitle:      "REZERWO",
			LBLAboutTitle: template.HTML("REZERWO - Zaolziański system do rezerwacji"),
			LBLAboutText: template.HTML(`
		Rezerwo to młody projekt utworzony z myślą o polskich organizacjach w Czechach. Ponieważ projekt powstał
		za pięć dwunasta, w tym roku działać będą tylko funkcje niezbędne do realizacji rezerwacji, i absolutne 
		podstawy administracji. Ale planów mam dużo ...<br />
		<b>Głównym celem projektu jest, by wszystkie bale (i podobne), których organizatorem są polskie organizacje w Czechach,
		były zarządzane za pomocą REZERWO.</b><br />
		Docelowo system będzie dostępny również w języku czeskim oraz angielskim. Zamierzam również udostępnić
		rozwiązanie subjektom komercyjnym jeżeli będą zainteresowane. Ale dla każdej organizacji polskiej w Czechach
		rozwiązanie zawsze będzie dostępne DARMOWO. <i>Oczywiście na miodule se zaprosić niechóm jeśli kiery beje nalegoł.</i><br />
		<br />
		Z góry przepraszam za możliwe niedociągnięcia, będą one stopniowo usuwane, tak by w przyszłym roku zachwyćić. Mam nadzieję.<br />
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
	LBLTitle      string
	LBLAlertTitle string
	LBLAlertText  string
	BTNBack       string
}

func ErrorHTML(errorTitle, errorText string, w http.ResponseWriter, r *http.Request) {
	errEN := ErrorVars{
		LBLTitle:      "Error",
		LBLAlertTitle: errorTitle,
		LBLAlertText:  errorText,
		BTNBack:       "OK",
	}
	_ = errEN
	errPL := ErrorVars{
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

	// TODO: RoomID and EventID should be something real
	eventID := int64(1)

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
				EventID:     eventID,
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
	RoomID int64  `json:"room_id"`
	Number int64  `json:"name"`
	Type   string `json:"type"`
}

func DesignerDeleteObject(db *DB) func(w http.ResponseWriter, r *http.Request) {

	// TODO: EventID should be something real
	eventID := int64(1)

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
			db.PriceDelByEventIDFurn(eventID, m.Number, m.Type)
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
			//log.Printf("%+v", m)
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

type Order struct {
	TotalPrice          string
	Sits, Prices, Rooms string
	Room                string
	Email               string
	Password            string
	Name, Surname       string
	Phone, Notes        string
}

func ParseTmpl(t string, o Order) string {
	var buf bytes.Buffer
	tmpl, err := template.New("test").Parse(t)
	if err != nil {
		log.Println("error parsing template %q, order %+v, err: %v", t, o, err)
		return t
	}
	err = tmpl.Execute(&buf, o)
	if err != nil {
		log.Println("error executing template %q, order %+v, err: %v", t, o, err)
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

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

	roomName := "Sala na dole"
	eventName := "Bal MS Karwina"

	mailpass, err := ioutil.ReadFile(".mailpass")
	if err != nil {
		log.Fatalf("can not read mail password from .mailpass file, err: %v", err)
	}
	db := initDB()
	defer db.Close()

	rtr := mux.NewRouter()
	rtr.HandleFunc("/res/{user}", ReservationHTML(db))

	handleStatic("js")
	handleStatic("css")
	//http.HandleFunc("/", DesignerOrg)
	http.Handle("/", rtr)
	//http.HandleFunc("/reservation", ReservationHTML(db, roomName, eventName))
	http.HandleFunc("/order", ReservationOrderHTML(db, eventName))
	http.HandleFunc("/order/status", ReservationOrderStatusHTML(db, eventName, strings.TrimSpace(string(mailpass))))
	http.HandleFunc("/admin", AdminMainPage(db, loc, dateFormat))
	http.HandleFunc("/admin/designer", DesignerHTML(db, roomName, eventName))
	http.HandleFunc("/admin/event", EventEditor(db))
	http.HandleFunc("/api/room", DesignerSetRoomSize(db))
	http.HandleFunc("/api/furnit", DesignerMoveObject(db))
	http.HandleFunc("/api/furdel", DesignerDeleteObject(db))
	http.HandleFunc("/api/renumber", DesignerRenumberType(db))

	log.Fatal(http.ListenAndServe(":3002", nil))
}

func initDB() *DB {
	howto := `<h1>Legenda:</h1>
	<ul>
		<li>Zielony = możliwa rezerwacja.</li>
		<li>Pomarańczowy = zarezerowane.</li>
		<li>Czerwony = zapłacona rezerwacja.</li>
	</ul>
	<p>Cena biletu: 300 Kč. W cenie biletu:</p>
	<ul>
		<li>Miejscówka</li>
		<li>Wstęp na bal</li>
		<li>Welcome drink</li>
		<li>Smaczna kolacja</li>
		<li>Woda i mały poczęstunek na stole</li>
	</ul>`
	mailText := `Dziękujemy za zamówienie!
Niniejszym mailem potwierdzamy zamówienie krzeseł:
{{.Sits}}
w cenie {{.TotalPrice}}.
Prosimy o przesłanie kwoty na rachunek:
1234567/6200

Do zobaczenia!

---
Macierz Szkolna
Karwina
`
	db := DBInit("db.sql")
	db.MustConnect()

	db.StructureCreate()

	uID, err := db.UserAdd(&User{ID: 1, Email: "pspmacierzkarwina@seznam.cz", URL: "mskarwina", Passwd: "MagikINFO2019"})
	if err != nil {
		log.Println(err)
	}

	r1ID, err := db.RoomAdd(&Room{ID: 1, Name: "Sala na dole", Description: ToNS("Tako fajno sala na dole."), Width: 1000, Height: 1000})
	if err != nil {
		log.Println(err)
	}
	r2ID, err := db.RoomAdd(&Room{ID: 2, Name: "Balkón", Description: ToNS("Na balkón bez dzieci."), Width: 500, Height: 500})
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
	eID, err := db.EventAdd(&Event{ID: 1, Name: "Bal MS Karwina", Date: 1581033600, FromDate: 1572998400, ToDate: 1580860800, DefaultPrice: 500, DefaultCurrency: "Kč", OrderedNote: "Dziękujemy. Dostaną państwo maila z informacją.", MailSubject: "Zamówienie biletów", MailText: mailText, HowTo: howto, UserID: 1})
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

func DesignerOrg(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.ParseFiles("tmpl/a_designer.html"))
	err := t.Execute(w, nil) //execute the template and pass it the HomePageVars struct to fill in the gaps
	if err != nil {
		log.Print("Designer template executing error: ", err) //log it
	}
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
	HTMLHowTo template.HTML
	BTNOrder  string
	Tables    []Furniture
	Chairs    []FurnitureFull
	Objects   []Furniture
	Labels    []Furniture
}

func DesignerHTML(db *DB, roomName, eventName string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		p := GetPageVarsFromDB(db, roomName, eventName)
		log.Printf("%+v", p)
		enPM := PageMeta{
			LBLTitle: "Designer",
			DesignerPage: DesignerPage{
				TableNr:             int64(len(p.Tables)) + 1,
				ChairNr:             int64(len(p.Chairs)) + 1,
				ObjectNr:            int64(len(p.Objects)) + 1,
				LabelNr:             int64(len(p.Labels)) + 1,
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
		if err != nil {
			log.Printf("error getting user %q, err: %v", v["user"], err)
			http.Error(w, fmt.Sprintf("<html><body><b>User %q not found! Check You have correct URL!<br />If problem persists, contact me at: ernierasta (at) zori.cz</b></body></html>", v["user"]), 500)
			return
		}
		e, err := EventGetCurrent(db, u.ID)
		if err != nil {
			log.Printf("error getting current event for userID: %d, err: %v", u.ID, err)
			http.Error(w, "<html><body><b>User have no active events! Come back later, when reservations will be opened!</b></body></html>", 500) //TODO: inform about closest user event and when it is
			return
		}
		fmt.Println(e.ID)
		rr, err := db.EventGetRooms(e.ID)
		if err != nil {
			log.Printf("error getting rooms for eventID: %d, err: %v", e.ID, err)
			http.Error(w, fmt.Sprintf("<html><body><b>Rooms for user: %q, event: %q not found!<br />If problem persists, contact me at: ernierasta (at) zori.cz</b></body></html>", v["user"], e.Name), 500)
			return
		}
		log.Println(rr)
		p := ReservationPageVars{
			LBLTitle: "Reservation",
			Event:    e,
			Rooms:    []RoomVars{},
		}
		for i := range rr {
			// TODO: remake it, GetPageVarsFromDB call the same again, move previous lines there
			rv := GetFurnituresFromDB(db, rr[i].Name, e.ID)
			rv.Room = rr[i]
			rv.HTMLHowTo = template.HTML(e.HowTo)
			rv.BTNOrder = "Order"

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

func ReservationOrderStatusHTML(db *DB, eventName, mailpass string) func(w http.ResponseWriter, r *http.Request) {
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
			o.Sits = r.Form["sits"][0]
			o.Prices = r.Form["prices"][0]
			o.Rooms = r.Form["rooms"][0]
			o.TotalPrice = r.Form["total-price"][0]
			o.Email = r.Form["email"][0]
			o.Password = r.Form["password"][0]
			o.Name = r.Form["name"][0]
			o.Surname = r.Form["surname"][0]
			o.Phone = r.Form["phone"][0]
			o.Notes = r.Form["notes"][0]

			ss := strings.Split(o.Sits, ",")
			pp := strings.Split(o.Prices, ",") // unused here, price is written to DB in INSERT
			rr := strings.Split(o.Rooms, ",")

			if len(ss) != len(pp) || len(ss) != len(rr) {
				log.Println("ReservationOrderHTML: error, POST - wrong lenght, ss: %q, pp: %q, rr: %q",
					o.Sits, o.Prices, o.Rooms)
			}

			for i := range ss {
				chairNumber, err := strconv.ParseInt(strings.TrimSpace(ss[i]), 10, 64)
				if err != nil {
					log.Println(err)
				}
				roomID, err := strconv.ParseInt(strings.TrimSpace(rr[i]), 10, 64)
				if err != nil {
					log.Println(err)
				}
				chair, err := db.FurnitureGetByTypeNumberRoom("chair", chairNumber, roomID)
				if err != nil {
					log.Println(err)
				}

				reservation, err := db.ReservationGet(chair.ID, event.ID)
				if err != nil {
					log.Printf("error retrieving reservation for chair: %d, eventID: %d, err: %v", chair.ID, event.ID, err)
				}
				reservation.Status = "ordered"
				err = db.ReservationMod(&reservation)
				if err != nil {
					log.Printf("error modyfing reservation for chair: %d, eventID: %d, err: %v", chair.ID, event.ID, err)
					http.Error(w, fmt.Sprintf("<html><body><b>Can not update reservation for chair: %d, eventID: %d, err: %v</b></body></html>", chair.ID, event.ID, err), 500)
					return
				}

			}

			custMail := MailConfig{
				Server:  "magikinfo.cz",
				Port:    587,
				User:    "rezerwo@zori.cz",
				Pass:    mailpass,
				From:    user.Email,
				ReplyTo: user.Email,
				Sender:  "rezerwo@zori.cz",
				To:      []string{o.Email},
				Subject: event.MailSubject,
				Text:    ParseTmpl(event.MailText, o),
			}
			err = MailSend(custMail)
			if err != nil {
				log.Println(err)
			}
			userMail := MailConfig{
				Server:  "magikinfo.cz",
				Port:    587,
				User:    "rezerwo@zori.cz",
				Pass:    mailpass,
				From:    "rezerwo@zori.cz",
				ReplyTo: "rezerwo@zori.cz",
				Sender:  "rezerwo@zori.cz",
				To:      []string{user.Email},
				Subject: event.MailSubject,
				Text:    ParseTmpl("{{.Name}} {{.Surname}}\nkrzesła: {{.Sits}}\nŁączna cena: {{.TotalPrice}}\nEmail: {{.Email}}\nTel: {{.Phone}}\nNotatki:{{.Notes}}", o), //TODO
			}
			err = MailSend(userMail)
			if err != nil {
				log.Println(err)
			}
		}

		p := ReservationOrderStatusVars{
			LBLTitle:      "Order status",
			LBLStatus:     "Tickets for " + event.Name + " ordered!",
			LBLStatusText: event.OrderedNote,
			BTNOk:         "Ok",
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

func GetPageVarsFromDB(db *DB, roomName, eventName string) Page {
	room, err := db.RoomGetByName(roomName)
	if err != nil {
		log.Printf("error getting room by name %q, err: %v", roomName, err)
	}
	event, err := db.EventGetByName(eventName)
	if err != nil {
		log.Printf("error getting event by name: %q, err: %v", eventName, err)
	}
	chairs, err := db.FurnitureFullGetChairs(event.ID, roomName)
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
	BTNSubmit                           string
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
			log.Printf("error getting event by name: %q, err: %v", eventName, err)
		}

		if r.Method == "POST" {
			err := r.ParseForm()
			if err != nil {
				log.Printf("error parsing form data:, err: %v", err)
			}
			sits = r.Form["sits"][0]
			prices = r.Form["prices"][0]
			rooms = r.Form["rooms"][0]
			totalPrice = r.Form["total-price"][0]
			defaultCurrency = r.Form["default-currency"][0]

			ss := strings.Split(sits, ",")
			pp := strings.Split(prices, ",")
			rr := strings.Split(rooms, ",")

			if len(ss) != len(pp) || len(ss) != len(rr) {
				log.Println("ReservationOrderHTML: error, POST - wrong lenght, ss: %q, pp: %q, rr: %q",
					sits, prices, rooms)
			}

			for i := range ss {
				chairNumber, err := strconv.ParseInt(strings.TrimSpace(ss[i]), 10, 64)
				if err != nil {
					log.Println(err)
				}
				roomID, err := strconv.ParseInt(strings.TrimSpace(rr[i]), 10, 64)
				if err != nil {
					log.Println(err)
				}
				chair, err := db.FurnitureGetByTypeNumberRoom("chair", chairNumber, roomID)
				if err != nil {
					log.Println(err)
				}
				chairPrice, err := strconv.ParseInt(strings.TrimSpace(pp[i]), 10, 64)
				if err != nil {
					log.Println(err)
				}

				_, err = db.ReservationAdd(&Reservation{
					OrderedDate: ToNI(time.Now().Unix()),
					Price:       ToNI(chairPrice),
					Currency:    ToNS(defaultCurrency),
					Status:      "marked", // this is very important
					FurnitureID: chair.ID,
					EventID:     event.ID,
					CustomerID:  -1, // this will be updated when customer is created
				})
				if err != nil {
					log.Printf("error adding reservation for chair: %d, eventID: %d, err: %v", chair.ID, event.ID, err)
					http.Error(w, fmt.Sprintf("<html><body><b>Can not add reservation for chair: %d, eventID: %d, err: %v</b></body></html>", chair.ID, event.ID, err), 500)
					return
				}
			}
		}
		//p := GetPageVarsFromDB(db, roomName, eventName)
		p := ReservationOrderVars{
			Event:                 event,
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
			LBLNotesHelp:          "Additional notes",
			LBLPricesValue:        prices,
			LBLRoomsValue:         rooms,
			LBLSits:               "Sits",
			LBLSitsValue:          sits,
			LBLTotalPrice:         "Total price",
			LBLTotalPriceValue:    totalPrice + " " + defaultCurrency,
			BTNSubmit:             "Confirm order",
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
	LBLNewEventPlaceholder string
	LBLSelectEvent         string
	BTNEventEdit           string
	BTNEventDelete         string
	LBLMsgTitle            string
}

func AdminMainPage(db *DB, loc *time.Location, dateFormat string) func(w http.ResponseWriter, r *http.Request) {
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

		rooms, err := db.RoomGetAll()
		if err != nil {
			log.Printf("error getting all rooms, err: %q", err)
		}
		events, err := db.EventGetAll()
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
				log.Printf("problem parsing form data, err: %v", err)
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

type RoomMsg struct {
	Name   string `json:"name"`
	Width  int64  `json:"width"`
	Height int64  `json:"height"`
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
				Name:   m.Name,
				Height: m.Height,
				Width:  m.Width,
			}
			log.Printf("write to db: %+v\n", room)
			err = db.RoomModSizeByName(&room)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

type MoveMsg struct {
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
	roomID := int64(1)
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
				RoomID:      roomID,
			}

			fID, err := db.FurnitureAdd(&f)
			if err != nil {
				log.Printf("info: inserting furniture failed, trying to update, err: %v", err)
				err := db.FurnitureModByNumberTypeRoom(&f)
				if err != nil {
					log.Printf("error: furniture insert failed, now also update failed, f: %+v, err: %v", f, err)
				}
			}

			// only for logging
			f.ID = fID
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
				FurnitureID: fID,
			}
			_, err = db.PriceAdd(&p)
			if err != nil {
				log.Printf("info: price inserting failed, trying update, err: %v", err)
				err = db.PriceMod(&p)
				if err != nil {
					log.Printf("error: price insert failed, now also update failed, p: %+v, err: %v", p, err)
				}

			}
		}
	}
}

type DeleteMsg struct {
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
			err = db.FurnitureDelByNumberType(m.Number, m.Type)
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
			log.Println(ff)
			ff, err = FurnitureRenumber(ff)
			if err != nil {
				log.Println(err)
			}
			log.Println(ff)
			for i := range ff {
				err := db.FurnitureMod(&ff[i])
				if err != nil {
					log.Println(err)
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
	return sql.NullString{
		String: s,
		Valid:  true,
	}
}

func ToNI(i int64) sql.NullInt64 {
	return sql.NullInt64{
		Int64: i,
		Valid: true,
	}
}

func ToDate(unix int64) string {
	t := time.Unix(unix, 0)
	return t.Format("2006-01-02")
}

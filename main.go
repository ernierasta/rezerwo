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
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

func main() {

	roomName := "Sala na dole"
	eventName := "Balik maskowy"

	mailpass, err := ioutil.ReadFile(".mailpass")
	if err != nil {
		log.Fatalf("can not read mail password from .mailpass file, err: %v", err)
	}

	db := initDB()
	defer db.Close()

	rtr := mux.NewRouter()
	rtr.HandleFunc("/res/{user}", ReservationHTML(db, roomName, eventName))

	handleStatic("js")
	handleStatic("css")
	//http.HandleFunc("/", DesignerOrg)
	http.Handle("/", rtr)
	http.HandleFunc("/reservation", ReservationHTML(db, roomName, eventName))
	http.HandleFunc("/order", ReservationOrderHTML(db, eventName))
	http.HandleFunc("/order/status", ReservationOrderStatusHTML(db, eventName, strings.TrimSpace(string(mailpass))))
	http.HandleFunc("/admin", AdminMainPage(db))
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
	db.UserAdd(&User{Email: "sales@a.com", Passwd: "a"})
	db.RoomAdd(&Room{ID: 1, Name: "Sala na dole", Description: ToNS("Tako fajno sala na dole."), Width: 1000, Height: 1000})
	db.EventAddOrUpdate(&Event{Name: "Balik maskowy", Date: 1572601325, FromDate: 1569888000, ToDate: 1572601325, DefaultPrice: 500, DefaultCurrency: "Kč", OrderedNote: "Dziękujemy. Dostaną państwo maila z informacją.", MailSubject: "Zamówienie biletów", MailText: mailText, HowTo: howto, UserID: 1})
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

func ReservationHTML(db *DB, roomName, eventName string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(mux.Vars(r))
		p := GetPageVarsFromDB(db, roomName, eventName)
		enPM := PageMeta{
			LBLTitle: "Reservation",
			ReservationPage: ReservationPage{
				HTMLHowTo: template.HTML(p.Event.HowTo),
				BTNOrder:  "Order",
			},
		}
		p.PageMeta = enPM
		t := template.Must(template.ParseFiles("tmpl/reservation.html", "tmpl/base.html"))
		err := t.ExecuteTemplate(w, "base", p)
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

		if r.Method == "POST" {
			err := r.ParseForm()
			if err != nil {
				log.Printf("error parsing form data:, err: %v", err)
			}
			o.Sits = r.Form["sits"][0]
			o.TotalPrice = r.Form["total-price"][0]
			o.Email = r.Form["email"][0]
			o.Password = r.Form["password"][0]
			o.Name = r.Form["name"][0]
			o.Surname = r.Form["surname"][0]
			o.Phone = r.Form["phone"][0]
			o.Notes = r.Form["notes"][0]

			client := MailConfig{
				Server:  "magikinfo.cz",
				Port:    587,
				User:    "rezerwo@zori.cz",
				Pass:    mailpass,
				From:    "rezerwo@zori.cz",
				To:      []string{o.Email},
				Subject: event.MailSubject,
				Text:    ParseTmpl(event.MailText, o),
			}
			err = MailSend(client)
			if err != nil {
				log.Println(err)
			}
		}

		p := ReservationOrderStatusVars{
			LBLTitle:      "Order status",
			LBLStatus:     "TODO: status name " + event.Name,
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
	LBLSits, LBLSitsValue               string
	LBLTotalPrice, LBLTotalPriceValue   string
	BTNSubmit                           string
}

func ReservationOrderHTML(db *DB, eventName string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sits := ""
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
			totalPrice = r.Form["total-price"][0]
			defaultCurrency = r.Form["default-currency"][0]
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

func AdminMainPage(db *DB) func(w http.ResponseWriter, r *http.Request) {
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
				e := Event{
					ID: int64(id),
				}
				_ = e //TODO
			}
			log.Println(dtype)
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
	LBLTitle         string
	LBLID            string
	LBLIDValue       int64
	LBLName          string
	LBLNameValue     string
	NameHelpText     string
	LBLFromDate      string
	LBLFromDateValue string
	LBLToDate        string
	LBLToDateValue   string
	HTMLHowTo        template.HTML
	BTNSave          string
	BTNCancel        string
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
				LBLTitle:         "Event details",
				LBLID:            "ID",
				LBLIDValue:       event.ID,
				LBLName:          "Name",
				LBLNameValue:     event.Name,
				LBLFromDate:      "From date",
				LBLFromDateValue: ToDate(event.FromDate),
				NameHelpText:     "Name help text",
				LBLToDate:        "To date",
				LBLToDateValue:   ToDate(event.ToDate),
				BTNSave:          "Save",
				BTNCancel:        "Cancel",
				HTMLHowTo:        template.HTML(event.HowTo),
			}
			t := template.Must(template.ParseFiles("tmpl/a_event.html", "tmpl/base.html"))
			err = t.ExecuteTemplate(w, "base", rp)
			if err != nil {
				log.Print("AdminEventEditor template executing error: ", err)
			}
			//b, err := ioutil.ReadAll(r.Body)
			//if err != nil {
			//	log.Println(err)
			//}
			//log.Println(b)
			w.Write([]byte("bla bla"))
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

			fID, err := db.FurnitureAddOrUpdate(&f)
			if err != nil {
				log.Println(err)
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
			_, err = db.PriceAddOrUpdate(&p)
			//log.Printf("write PRICE to db: %+v", p)
			if err != nil {
				log.Println(err)
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
			ff = FurnitureRenumber(ff)
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

type Order struct {
	TotalPrice    string
	Sits, Room    string
	Email         string
	Password      string
	Name, Surname string
	Phone, Notes  string
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

package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/microcosm-cc/bluemonday"
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
	cookieStore.Options = sessionOptions(false) // 2 godziny, patrz auth.go

	lang := "pl" // cs

	mailConf := &MailConfig{Server: conf.MailServer, Port: int(conf.MailPort), From: conf.MailFrom, User: conf.MailUser, Pass: conf.MailPass, IgnoreCert: conf.MailIgnoreCert, Hostname: conf.MailHostname}

	db := initDB()
	defer db.Close()
	HashPlainPasswords(db)

	stop := make(chan bool)
	Ticker5min(db, stop)
	defer TickerStop(stop)

	if err := LoadLocales(); err != nil {
		log.Fatalf("can not load interface translations: %v", err)
	}

	rtr := mux.NewRouter()
	// nowy panel (redesign): strony z ID w adresie, do zakładek i nowej karty
	rtr.HandleFunc("/admin", AdminStartPage(db, cookieStore))
	rtr.Handle("/admin/start", http.RedirectHandler("/admin", http.StatusFound))
	rtr.HandleFunc("/admin/events/{id:[0-9]+}", EventPage(db, cookieStore))
	rtr.HandleFunc("/api/events", EventCreateAPI(db, cookieStore))
	rtr.HandleFunc("/api/events/{id:[0-9]+}/section", EventSectionSave(db, loc, dateFormat, cookieStore))
	rtr.HandleFunc("/admin/events/{id:[0-9]+}/reservations", ReservationsPage(db, cookieStore))
	rtr.HandleFunc("/admin/events/{id:[0-9]+}/rooms/{roomid:[0-9]+}", RoomDesignerPage(db, cookieStore))
	rtr.HandleFunc("/admin/rooms/{roomid:[0-9]+}", RoomDesignerPage(db, cookieStore))
	rtr.HandleFunc("/api/events/{id:[0-9]+}/reservations", ReservationsListAPI(db, cookieStore)).Methods("GET")
	rtr.HandleFunc("/api/events/{id:[0-9]+}/reservations", ReservationsDeleteAPI(db, cookieStore)).Methods("DELETE")
	rtr.HandleFunc("/api/events/{id:[0-9]+}/reservations.csv", ReservationsCSVAPI(db, cookieStore))
	rtr.HandleFunc("/api/events/{id:[0-9]+}/report.csv", EventReportCSVAPI(db, cookieStore))
	rtr.HandleFunc("/api/events/{id:[0-9]+}/addons", EventAddonsCreateAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/events/{id:[0-9]+}/addons.csv", EventAddonsCSVAPI(db, cookieStore))
	rtr.HandleFunc("/api/events/{id:[0-9]+}/orders/{oid:[0-9]+}/items", OrderItemAddAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/events/{id:[0-9]+}/orders/{oid:[0-9]+}/items/{iid:[0-9]+}/status", OrderItemStatusAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/events/{id:[0-9]+}/orders/{oid:[0-9]+}/items/{iid:[0-9]+}", OrderItemDeleteAPI(db, cookieStore)).Methods("DELETE")
	rtr.HandleFunc("/api/events/{id:[0-9]+}/reservations/status", ReservationsStatusAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/events/{id:[0-9]+}/reservations/history", ReservationsHistoryAPI(db, cookieStore)).Methods("GET")
	rtr.HandleFunc("/api/events/{id:[0-9]+}/reservations/notes", ReservationNotesAPI(db, cookieStore)).Methods("PATCH")
	rtr.HandleFunc("/api/events/{id:[0-9]+}/reservations/mail", ReservationsMailAPI(db, *mailConf, cookieStore)).Methods("POST")
	rtr.HandleFunc("/admin/catalogs", CatalogsPage(db, cookieStore))
	rtr.HandleFunc("/admin/mails/{id:[0-9]+}", MailPage(db, cookieStore))
	rtr.HandleFunc("/admin/accounts/{id:[0-9]+}", AccountPage(db, cookieStore))
	rtr.HandleFunc("/api/mails", MailCreateAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/mails/{id:[0-9]+}", MailSaveAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/mails/sources", MailSourcesAPI(db, cookieStore)).Methods("GET")
	rtr.HandleFunc("/api/mails/preview", MailSourcePreviewAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/media/banners", BannerFilesAPI(db, cookieStore)).Methods("GET", "POST")
	rtr.HandleFunc("/api/mails/{id:[0-9]+}/preview", MailPreviewAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/mails/{id:[0-9]+}/test", MailTestAPI(db, *mailConf, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/accounts", AccountCreateAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/accounts/{id:[0-9]+}", AccountSaveAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/admin/forms/{id:[0-9]+}", FormPage(db, cookieStore))
	rtr.HandleFunc("/admin/forms/{id:[0-9]+}/answers", AnswersPage(db, cookieStore))
	rtr.HandleFunc("/admin/forms/{id:[0-9]+}/fields", FormFieldsPage(db, cookieStore))
	rtr.HandleFunc("/api/forms/{id:[0-9]+}/fields", FormFieldsSaveAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/forms", FormCreateAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/forms/url", FormURLSuggestAPI(db, cookieStore)).Methods("GET")
	rtr.HandleFunc("/api/forms/{id:[0-9]+}/infopanel", FormPanelPreviewAPI(db, cookieStore)).Methods("GET")
	rtr.HandleFunc("/api/forms/{id:[0-9]+}", FormDeleteAPI(db, cookieStore)).Methods("DELETE")
	rtr.HandleFunc("/api/forms/{id:[0-9]+}/section", FormSectionSave(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/forms/{id:[0-9]+}/answers", AnswersListAPI(db, cookieStore)).Methods("GET")
	rtr.HandleFunc("/api/forms/{id:[0-9]+}/answers", AnswersDeleteAPI(db, cookieStore)).Methods("DELETE")
	rtr.HandleFunc("/api/forms/{id:[0-9]+}/answers.csv", AnswersCSVAPI(db, cookieStore))
	rtr.HandleFunc("/api/forms/{id:[0-9]+}/answers/status", AnswersStatusAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/forms/{id:[0-9]+}/answers/mail", AnswersMailAPI(db, *mailConf, cookieStore)).Methods("POST")
	rtr.HandleFunc("/locales/{file}", LocalesJSONHandler())

	// strony dla gości
	rtr.HandleFunc("/res/{user}", GuestReservationPage(db, lang, cookieStore))
	rtr.HandleFunc("/order", GuestOrderPage(db, lang))
	rtr.HandleFunc("/form/{userurl}/{formurl}", GuestFormPage(db, cookieStore))
	rtr.HandleFunc("/form/{userurl}/{formurl}/send", GuestFormSendAPI(db, *mailConf)).Methods("POST")
	rtr.HandleFunc("/form/{userurl}/{formurl}/done", GuestFormDonePage(db))
	rtr.HandleFunc("/", AboutHTML(lang))

	handleStatic("js")
	handleStatic("css")
	handleStatic("img")
	handleStatic("media") //user data
	http.Handle("/", rtr)
	http.HandleFunc("/order/status", ReservationOrderStatusHTML(db, lang, mailConf))
	http.HandleFunc("/admin/login", AdminLoginHTML(db, lang, cookieStore))
	http.HandleFunc("/admin/logout", AdminLogout(cookieStore))
	http.HandleFunc("/admin/devlogin", DevLogin(db, cookieStore))
	http.HandleFunc("/admin/lang", SetPanelLangHandler(db, cookieStore))
	http.HandleFunc("/api/login", LoginAPI(db, cookieStore))
	http.HandleFunc("/api/room", DesignerSetRoomSize(db, cookieStore))
	http.HandleFunc("/api/roomcopy", CopyRoomAPI(db, cookieStore))
	http.HandleFunc("/api/eventcopyinfo", EventCopyInfoAPI(db, cookieStore))
	http.HandleFunc("/api/eventcopy", EventCopyAPI(db, dateFormat, cookieStore))
	http.HandleFunc("/api/furnit", DesignerMoveObject(db, cookieStore))
	http.HandleFunc("/api/furdel", DesignerDeleteObject(db, cookieStore))
	http.HandleFunc("/api/ordercancel", OrderCancel(db))
	http.HandleFunc("/api/renumber", DesignerRenumberType(db, cookieStore))

	// REZERWO_ADDR allows running second (test) instance next to the default one
	addr := ":3002"
	if a := os.Getenv("REZERWO_ADDR"); a != "" {
		addr = a
	}
	log.Fatal(http.ListenAndServe(addr, nil))
}

func initDB() *DB {
	db := DBInit("db.sql")
	db.MustConnect()
	log.Printf("connected to database ...")
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
	BTNSelectTables, BTNSelectChairs     string
	LBLTableRect, LBLTableRound          string
	LBLTableOval                         string
	LBLTableRectVertical                 string
	LBLTableRectHorizontal               string
	LBLTableOvalVertical                 string
	LBLTableOvalHorizontal               string
	BTNChangeTableType                   string
	LBLGridSize, LBLGridShow             string
	LBLGridSnap                          string
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
	LBLFreeSits         string
	BTNClearSelection   string
	SitsTotal           int
	SitsFree            int
}

// userOwnsRoom checks if room belongs to organization of logged in admin
func userOwnsRoom(db *DB, email string, roomID int64) bool {
	user, err := db.UserGetByEmail(email)
	if err != nil {
		log.Printf("userOwnsRoom: can not get user by mail %q, err: %v", email, err)
		return false
	}
	rooms, err := db.RoomGetAllByUserID(user.ID)
	if err != nil {
		log.Printf("userOwnsRoom: can not get rooms for user %d, err: %v", user.ID, err)
		return false
	}
	for _, room := range rooms {
		if room.ID == roomID {
			return true
		}
	}
	return false
}

// designerAPIAuth returns true if request comes from admin owning the room,
// otherwise it writes error response
func designerAPIAuth(w http.ResponseWriter, r *http.Request, db *DB, cs *sessions.CookieStore, roomID int64) bool {
	_, _, email, err := InitSession(w, r, cs, "/admin/login", true)
	if err != nil {
		log.Printf("designerAPIAuth: session error: %v", err)
		return false
	}
	if !userOwnsRoom(db, email, roomID) {
		log.Printf("designerAPIAuth: user %q does not own room %d", email, roomID)
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
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
	Theme         string // atrybut data-theme (themes.go)
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

			// zebrane po drodze, żeby po pętli spiąć je w jedno zamówienie
			reservationIDs := []int64{}

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
				reservationIDs = append(reservationIDs, reservation.ID)
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

			// Zamówienie jako byt: spina rezerwacje z klientem, a przy okazji
			// zapisuje dodatki (odpowiedzi i dopłaty). Kwota w o.TotalPrice
			// przychodzi z formularza już z dopłatami, więc jej nie zmieniamy.
			SaveOrderWithAddons(db, event, user, cID, reservationIDs, o.Notes, r.Form)

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
				Subject:         OrderSubject(cstMail.Title.String, orderFuncsFromOrder(o, db, user)),
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

			subj := OrderSubject(adminMail.Title.String, orderFuncsFromOrder(o, db, user))
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
			Theme:         themeAttr(event.Theme.String),
		}

		// nowy wygląd strony statusu zamówienia (redesign, etap 4);
		// szablon jest samodzielny, bez starego a_base.html i Bootstrapa
		t := template.Must(template.ParseFiles("tmpl/n_order_status.html"))
		err := t.Execute(w, p)
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

func AdminLoginHTML(db *DB, lang string, cookieStore *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		lng := PanelLang(r)
		guestRender(w, lng, struct{ Title string }{T(lng, "login.title")}, "tmpl/n_login.html")
	}
}

// AdminLogout removes admin session cookie and redirects to login page
func AdminLogout(cookieStore *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := cookieStore.Get(r, AUTHCOOKIE)
		if err != nil {
			// invalid cookie, new session is returned, we still overwrite the cookie
			log.Printf("AdminLogout: session error: %v", err)
		}
		session.Values = map[interface{}]interface{}{}
		session.Options.MaxAge = -1
		if err := session.Save(r, w); err != nil {
			log.Printf("AdminLogout: can not remove session, err: %v", err)
		}
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	}
}

type AdminMainPageVars struct {
	TabEvents                     string
	TabForms                      string
	TabSettings                   string
	LBLRooms                      string
	LBLEvents                     string
	LBLRoomEventTitle             string
	LBLRoomEventText              template.HTML
	BTNSelect                     string
	BTNClose                      string
	BTNAddRoom                    string
	BTNAddEvent                   string
	LBLSelectRoom                 string
	BTNRoomEdit                   string
	BTNRoomCopy                   string
	LBLRoomCopyTitle              string
	LBLRoomCopyText               string
	BTNRoomDelete                 string
	LBLNewEventPlaceholder        string
	LBLSelectEvent                string
	BTNEventEdit                  string
	BTNEventCopy                  string
	BTNEventDelete                string
	LBLEventCopyTitle             string
	LBLEventCopyName              string
	LBLEventCopyDate              string
	LBLEventCopyFromDate          string
	LBLEventCopyRooms             string
	LBLEventCopyOnlyMine          string
	LBLEventCopyMakeRoomCopy      string
	LBLRoomCopyName               string
	BTNEventCopyAddRoom           string
	LBLEventCopyNotifs            string
	LBLEventCopyNoNotifs          string
	LBLEventCopyThankYouName      string
	LBLEventCopyAdminName         string
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
	BTNFormDel                    string
	BTNFormCopy                   string
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

func AboutHTML(lang string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			ErrorHTML(T(PanelLang(r), "about.not_found_title"), T(PanelLang(r), "about.not_found"), PanelLang(r), w, r)
			return
		}
		lng := PanelLang(r)
		guestRender(w, lng, struct{ Title string }{T(lng, "app.name")}, "tmpl/n_about.html")
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

func ErrorHTML(errorTitle, errorText, lang string, w http.ResponseWriter, r *http.Request) {
	guestRender(w, KnownLang(lang), nErrorVars{Title: errorTitle, Text: errorText, Back: true}, "tmpl/n_error.html")
}

type RoomMsg struct {
	RoomID int64 `json:"room_id"`
	Width  int64 `json:"width"`
	Height int64 `json:"height"`
}

func DesignerSetRoomSize(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var m RoomMsg
		if r.Method == "POST" {
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&m)
			if err != nil {
				log.Println(err)
			}
			if !designerAPIAuth(w, r, db, cs, m.RoomID) {
				return
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

type RoomIDMsg struct {
	RoomID int64  `json:"room_id"`
	Name   string `json:"name"` // new name for room copy
}

func CopyRoomAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		_, _, email, err := InitSession(w, r, cs, "/admin/login", true)
		if err != nil {
			log.Printf("CopyRoomAPI: session error: %v", err)
			return
		}

		user, err := db.UserGetByEmail(email)
		if err != nil {
			log.Printf("CopyRoomAPI: can not get user by mail %q, %v", email, err)
			return
		}

		if r.Method == "POST" {
			var m RoomIDMsg
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&m)
			if err != nil {
				log.Println(err)
			}
			if !userOwnsRoom(db, email, m.RoomID) {
				log.Printf("CopyRoomAPI: user %q does not own room %d", email, m.RoomID)
				jsonMsg(w, http.StatusForbidden, map[string]interface{}{"msg": "forbidden"})
				return
			}
			if strings.TrimSpace(m.Name) == "" {
				jsonMsg(w, http.StatusBadRequest, map[string]interface{}{"msg": "empty name"})
				return
			}
			roomID, err := db.RoomWithFurnituresCopy(m.RoomID, strings.TrimSpace(m.Name))
			if err != nil {
				log.Println(err)
				jsonMsg(w, http.StatusTeapot, map[string]interface{}{"msg": "Nie udało się skopiować pomieszczenia!\n" + err.Error()})
				return
			}
			err = db.RoomAssignToUser(user.ID, roomID)
			if err != nil {
				log.Printf("CopyRoomAPI: room assignment error, %v", err)
				jsonMsg(w, http.StatusTeapot, map[string]interface{}{"msg": "Nie udało się przypisać pomieszczenia!\n" + err.Error()})
				return
			}
			jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": fmt.Sprintf("copied room %d to %d", m.RoomID, roomID), "room_id": roomID})
		}
	}
}

// jsonMsg writes v as JSON response with given status code
func jsonMsg(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("jsonMsg: error encoding response, %v", err)
	}
}

type EventCopyRoom struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Mine bool   `json:"mine"`
}

type EventCopyNotif struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type EventCopyInfo struct {
	EventID  int64           `json:"event_id"`
	Name     string          `json:"name"`
	Date     string          `json:"date"`
	FromDate string          `json:"from_date"`
	RoomIDs  []int64         `json:"room_ids"`
	AllRooms []EventCopyRoom `json:"all_rooms"`
	ThankYou *EventCopyNotif `json:"thankyou"`
	Admin    *EventCopyNotif `json:"admin"`
}

// eventCopyGetSource returns event which user is allowed to copy (own or sharable)
func eventCopyGetSource(db *DB, eventID int64, user User) (Event, error) {
	ev, err := db.EventGetByID(eventID)
	if err != nil {
		return ev, fmt.Errorf("nie znaleziono imprezy %d: %v", eventID, err)
	}
	if ev.UserID != user.ID && !(ev.Sharable.Valid && ev.Sharable.Bool) {
		return ev, fmt.Errorf("impreza %d nie należy do użytkownika", eventID)
	}
	return ev, nil
}

// EventCopyInfoAPI returns data needed by event copy dialog
func EventCopyInfoAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, email, err := InitSession(w, r, cs, "/admin/login", true)
		if err != nil {
			log.Printf("EventCopyInfoAPI: session error: %v", err)
			return
		}
		user, err := db.UserGetByEmail(email)
		if err != nil {
			log.Printf("EventCopyInfoAPI: can not get user by mail %q, %v", email, err)
			jsonMsg(w, http.StatusTeapot, map[string]interface{}{"msg": "Nieznany użytkownik"})
			return
		}
		eventID, err := strconv.ParseInt(r.FormValue("event_id"), 10, 64)
		if err != nil || eventID == 0 {
			jsonMsg(w, http.StatusTeapot, map[string]interface{}{"msg": "Wybierz imprezę do skopiowania."})
			return
		}
		ev, err := eventCopyGetSource(db, eventID, user)
		if err != nil {
			log.Printf("EventCopyInfoAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]interface{}{"msg": err.Error()})
			return
		}

		info := EventCopyInfo{EventID: ev.ID, Name: ev.Name, Date: ToDate(ev.Date), FromDate: ToDate(ev.FromDate), RoomIDs: []int64{}, AllRooms: []EventCopyRoom{}}

		evRooms, err := db.EventGetRooms(ev.ID)
		if err != nil {
			log.Printf("EventCopyInfoAPI: error getting rooms for event %d, %v", ev.ID, err)
		}
		for _, room := range evRooms {
			info.RoomIDs = append(info.RoomIDs, room.ID)
		}

		userRooms, err := db.RoomGetAllByUserID(user.ID)
		if err != nil {
			log.Printf("EventCopyInfoAPI: error getting rooms for user %d, %v", user.ID, err)
		}
		mine := map[int64]bool{}
		for _, room := range userRooms {
			mine[room.ID] = true
		}
		allRooms, err := db.RoomGetAll()
		if err != nil {
			log.Printf("EventCopyInfoAPI: error getting all rooms, %v", err)
		}
		for _, room := range allRooms {
			info.AllRooms = append(info.AllRooms, EventCopyRoom{ID: room.ID, Name: room.Name, Mine: mine[room.ID]})
		}

		if ev.ThankYouNotificationsID.Valid && ev.ThankYouNotificationsID.Int64 != 0 {
			if n, err := db.NotificationGetByIDUnsafe(ev.ThankYouNotificationsID.Int64); err == nil {
				info.ThankYou = &EventCopyNotif{ID: n.ID, Name: n.Name}
			}
		}
		if ev.AdminNotificationsID.Valid && ev.AdminNotificationsID.Int64 != 0 {
			if n, err := db.NotificationGetByIDUnsafe(ev.AdminNotificationsID.Int64); err == nil {
				info.Admin = &EventCopyNotif{ID: n.ID, Name: n.Name}
			}
		}

		jsonMsg(w, http.StatusOK, info)
	}
}

type EventCopyMsg struct {
	EventID  int64  `json:"event_id"`
	Name     string `json:"name"`
	Date     string `json:"date"`
	FromDate string `json:"from_date"`
	Rooms    []struct {
		RoomID int64  `json:"room_id"`
		Copy   bool   `json:"copy"`
		Name   string `json:"name"` // name of room copy, empty = original name
	} `json:"rooms"`
	CopyNotifications bool   `json:"copy_notifications"`
	ThankYouName      string `json:"thankyou_name"`
	AdminName         string `json:"admin_name"`
}

// EventCopyAPI creates copy of event, with rooms (linked or copied), prices,
// addons and optionally copies of notifications. Reservation ends day after event date.
func EventCopyAPI(db *DB, dF string, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, email, err := InitSession(w, r, cs, "/admin/login", true)
		if err != nil {
			log.Printf("EventCopyAPI: session error: %v", err)
			return
		}
		user, err := db.UserGetByEmail(email)
		if err != nil {
			log.Printf("EventCopyAPI: can not get user by mail %q, %v", email, err)
			jsonMsg(w, http.StatusTeapot, map[string]interface{}{"msg": "Nieznany użytkownik"})
			return
		}
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		fail := func(format string, a ...interface{}) {
			msg := fmt.Sprintf(format, a...)
			log.Printf("EventCopyAPI: %s", msg)
			jsonMsg(w, http.StatusTeapot, map[string]interface{}{"msg": msg})
		}

		var m EventCopyMsg
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			fail("Błędne dane: %v", err)
			return
		}
		m.Name = strings.TrimSpace(m.Name)
		m.ThankYouName = strings.TrimSpace(m.ThankYouName)
		m.AdminName = strings.TrimSpace(m.AdminName)

		src, err := eventCopyGetSource(db, m.EventID, user)
		if err != nil {
			fail("%v", err)
			return
		}
		if m.Name == "" {
			fail("Podaj nazwę nowej imprezy.")
			return
		}
		date, err := ToUnix(m.Date, nil, dF)
		if err != nil {
			fail("Podaj poprawną datę imprezy.")
			return
		}
		fromDate, err := ToUnix(m.FromDate, nil, dF)
		if err != nil {
			fail("Podaj poprawną datę początku rezerwacji.")
			return
		}
		if fromDate > date {
			fail("Początek rezerwacji musi być przed datą imprezy.")
			return
		}
		exists, err := db.EventNameExists(m.Name, user.ID)
		if err != nil {
			fail("Błąd bazy danych: %v", err)
			return
		}
		if exists {
			fail("Impreza o nazwie %q już istnieje, wybierz inną nazwę.", m.Name)
			return
		}
		srcRooms, err := db.EventGetRooms(src.ID)
		if err != nil {
			fail("Nie udało się pobrać pomieszczeń imprezy: %v", err)
			return
		}
		srcRoomIDs := map[int64]bool{}
		for _, room := range srcRooms {
			srcRoomIDs[room.ID] = true
		}
		seen := map[int64]bool{}
		for _, rm := range m.Rooms {
			if _, err := db.RoomGetByID(rm.RoomID); err != nil {
				fail("Nie znaleziono pomieszczenia %d.", rm.RoomID)
				return
			}
			if !rm.Copy {
				if seen[rm.RoomID] {
					fail("Pomieszczenie %d jest przypisane dwa razy.", rm.RoomID)
					return
				}
				seen[rm.RoomID] = true
			}
		}

		// notifications
		copyNotif := func(id sql.NullInt64, name string) (sql.NullInt64, error) {
			n, err := db.NotificationGetByIDUnsafe(id.Int64)
			if err != nil {
				return id, fmt.Errorf("nie znaleziono notyfikacji %d: %v", id.Int64, err)
			}
			if n.UserID != user.ID && !n.Sharable {
				return id, fmt.Errorf("notyfikacja %d nie należy do użytkownika", n.ID)
			}
			if name == "" {
				return id, fmt.Errorf("podaj nazwę kopii notyfikacji %q", n.Name)
			}
			n.ID = 0
			n.Name = name
			n.UserID = user.ID
			n.Sharable = false
			n.CreatedDate = time.Now().Unix()
			n.UpdatedDate = sql.NullInt64{}
			newID, err := db.NotificationAdd(&n)
			if err != nil {
				return id, fmt.Errorf("nie udało się skopiować notyfikacji %q: %v", name, err)
			}
			return ToNI(newID), nil
		}

		ev := src
		ev.ID = 0
		ev.Name = m.Name
		ev.Date = date
		ev.FromDate = fromDate
		ev.ToDate = time.Unix(date, 0).UTC().AddDate(0, 0, 1).Unix()
		if src.UserID != user.ID {
			ev.UserID = user.ID
			ev.Sharable = ToNB(false)
		}

		hasThankYou := src.ThankYouNotificationsID.Valid && src.ThankYouNotificationsID.Int64 != 0
		hasAdmin := src.AdminNotificationsID.Valid && src.AdminNotificationsID.Int64 != 0
		if m.CopyNotifications {
			if hasThankYou && m.ThankYouName == "" || hasAdmin && m.AdminName == "" {
				fail("Podaj nazwy kopiowanych notyfikacji.")
				return
			}
			if hasThankYou {
				ev.ThankYouNotificationsID, err = copyNotif(src.ThankYouNotificationsID, m.ThankYouName)
				if err != nil {
					fail("%v", err)
					return
				}
			}
			if hasAdmin {
				if hasThankYou && src.AdminNotificationsID.Int64 == src.ThankYouNotificationsID.Int64 {
					ev.AdminNotificationsID = ev.ThankYouNotificationsID // same notification used for both
				} else {
					ev.AdminNotificationsID, err = copyNotif(src.AdminNotificationsID, m.AdminName)
					if err != nil {
						fail("%v", err)
						return
					}
				}
			}
		}

		newEventID, err := db.EventAdd(&ev)
		if err == nil {
			// kopia wie, z czego powstała - stąd porównanie sprzedaży edycji
			if perr := db.EventSetPreviousEdition(newEventID, user.ID, ToNI(src.ID)); perr != nil {
				log.Printf("EventCopyAPI: previous edition: %v", perr)
			}
			if terr := db.EventSetTheme(newEventID, user.ID, ThemeKey(src.Theme.String)); terr != nil {
				log.Printf("EventCopyAPI: theme: %v", terr)
			}
		}
		if err != nil {
			fail("Nie udało się zapisać imprezy: %v", err)
			return
		}

		// rooms + prices
		var problems []string
		for _, rm := range m.Rooms {
			roomID := rm.RoomID
			if rm.Copy {
				roomID, err = db.RoomWithFurnituresCopy(rm.RoomID, strings.TrimSpace(rm.Name))
				if err != nil {
					problems = append(problems, fmt.Sprintf("kopia pomieszczenia %d: %v", rm.RoomID, err))
					continue
				}
				if err := db.RoomAssignToUser(user.ID, roomID); err != nil {
					problems = append(problems, fmt.Sprintf("przypisanie pomieszczenia %d: %v", roomID, err))
				}
			}
			if err := db.EventAddRoom(newEventID, roomID); err != nil {
				problems = append(problems, fmt.Sprintf("przypisanie pomieszczenia %d do imprezy: %v", roomID, err))
				continue
			}
			if srcRoomIDs[rm.RoomID] {
				if _, err := db.PriceCopyForEventRoom(src.ID, rm.RoomID, newEventID, roomID); err != nil {
					problems = append(problems, fmt.Sprintf("ceny pomieszczenia %d: %v", roomID, err))
				}
			}
		}
		if _, err := db.EventAddonCopy(src.ID, newEventID, user.ID); err != nil {
			problems = append(problems, fmt.Sprintf("dodatki imprezy: %v", err))
		}

		if len(problems) > 0 {
			fail("Impreza %q została utworzona, ale wystąpiły błędy:\n%s", m.Name, strings.Join(problems, "\n"))
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": fmt.Sprintf("copied event %d to %d", src.ID, newEventID), "event_id": newEventID})
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

func DesignerMoveObject(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var m MoveMsg
		if r.Method == "POST" {
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&m)
			if err != nil {
				log.Println(err)
			}
			if !designerAPIAuth(w, r, db, cs, m.RoomID) {
				return
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

func DesignerDeleteObject(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var m DeleteMsg
		if r.Method == "POST" {
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&m)
			if err != nil {
				log.Println(err)
			}
			if !designerAPIAuth(w, r, db, cs, m.RoomID) {
				return
			}
			// Krzesła z rezerwacjami są chronione (rozdz. 5.3 HARMONIJKA.md):
			// nie da się skasować ani takiego krzesła, ani stołu, przy którym stoi.
			if n, what, err := reservedUnder(db, m); err != nil {
				log.Printf("DesignerDeleteObject: reservation check: %v", err)
			} else if n > 0 {
				lang := PanelLang(r)
				key := "designer.chair_reserved"
				if what == "table" {
					key = "designer.table_reserved"
				}
				jsonMsg(w, http.StatusConflict, map[string]interface{}{
					"msg":          TN(lang, key, n),
					"reservations": n,
				})
				return
			}

			log.Printf("deleting from db: %+v\n", m)
			err = db.FurnitureDelByNumberTypeRoom(m.Number, m.Type, m.RoomID)
			if err != nil {
				log.Println(err)
				jsonMsg(w, http.StatusTeapot, map[string]interface{}{"msg": err.Error()})
				return
			}
			db.PriceDelByEventIDFurn(m.EventID, m.Number, m.Type)
			jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "deleted"})
		}
	}
}

type RenumberMsg struct {
	RoomID int64  `json:"room_id"`
	Type   string `json:"type"`
}

func DesignerRenumberType(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		var m RenumberMsg
		if r.Method == "POST" {
			dec := json.NewDecoder(r.Body)
			err := dec.Decode(&m)
			if err != nil {
				log.Println(err)
			}
			if !designerAPIAuth(w, r, db, cs, m.RoomID) {
				return
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

type FormAnswersJson struct {
	Name     string   `json:"name"`
	UserData []string `json:"userData"`
}

// sendMail will send mail, it takes parsed texts and EmbeddedImgs (from template functions) and adds it to mailConf.EmbeddedImgs
func prepMailConfSendMail(email, msubject, parsedMailText string, user User, EmbeddedImgs []EmbImg, mailConf MailConfig) error {

	// if email is empty we assume it anonymous form. We will not send any mail
	if email == "" {
		return fmt.Errorf("prepareAndSendMail: no mail sent, empty mail(anonymous form?)")
	}

	EmbeddedImgs = append(EmbeddedImgs, mailConf.EmbededHTMLImgs...) // combine qr code with embedded images

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
		Text:            parsedMailText,
		IgnoreCert:      mailConf.IgnoreCert,
		Hostname:        mailConf.Hostname,
		Files:           mailConf.Files,
		EmbededHTMLImgs: EmbeddedImgs,
	}

	err := MailSend(mail)
	if err != nil {
		return err
	}
	return nil
}

func parseFormMailTemplate(name, surname, mtext string, formID int64, user User, formTempl FormTemplate, db *DB) (string, []EmbImg) {
	var parsedMail bytes.Buffer

	nsn := surname + "_" + name

	thp := &FormFuncs{DB: db, User: user, Template: formTempl, FormID: formID, NameSurname: nsn}

	tmpl, err := template.New("thankyou").Parse(mtext)
	if err != nil {
		log.Printf("parseFormMailTemplate: error parsing ThankYou mail template, %v", err)
	}
	err = tmpl.ExecuteTemplate(&parsedMail, "thankyou", thp)
	if err != nil {
		log.Printf("parseFormMailTemplate: error executing ThankYou template, %v", err)
	}

	return parsedMail.String(), thp.EmbeddedImgs
}

func parseEventMailTemplate(name, surname, mtext, phone, email, notes, eventName string, sits, prices []int, rooms []string, eventID, totalPrice int64, user User, db *DB) (string, []EmbImg, []string) {
	var parsedMail bytes.Buffer

	of := newEventOrderFuncs(name, surname, phone, email, notes, eventName, sits, prices, rooms, eventID, totalPrice, user, db)

	tmpl, err := template.New("eventmail").Parse(mtext)
	if err != nil {
		log.Printf("parseEventMailTemplate: error parsing 'eventmail' mail template, %v", err)
	}
	err = tmpl.ExecuteTemplate(&parsedMail, "eventmail", of)
	if err != nil {
		log.Printf("parseEventMailTemplate: error executing 'eventmail' template, %v", err)
	}

	log.Printf("parseEventMailTemplate: attached files: %v", of.AttachedFiles) // DEBUG

	return parsedMail.String(), of.EmbeddedImgs, of.AttachedFiles
}

func makeSureIsHTML(s string) string {
	if strings.Contains(s, "<html") {
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

// GenFormDefsDeltaOp represents a single operation in Quill's Delta format
type GenFormDefsDeltaOp struct {
	Insert     interface{}            `json:"insert"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// GenFormDefsDelta represents a Quill Delta object
type GenFormDefsDelta struct {
	Ops []GenFormDefsDeltaOp `json:"ops"`
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

	// answers - odpowiedzi wczytane raz dla .Summary i .OptionCount (form_stats.go)
	answers formAnswers
}

// Sum returns sum of given form field.
// form field can be specified by name or display value,
// where name is always checked first.
// This function is forms specific!
func (i *FormFuncs) Sum(FormFieldName string) template.HTML {
	return i.sum(FormFieldName, false, false, "")
}

// SumAlfa sorts results by name (not by ammount) in case of string values
func (i *FormFuncs) SumAlfa(FormFieldName string) template.HTML {
	return i.sum(FormFieldName, true, false, "")
}

// SumStrings forces string values (even when they are numbers) and sorts by string name
func (i *FormFuncs) SumStrings(FormFieldName string) template.HTML {
	return i.sum(FormFieldName, true, true, "")
}

// SumOnly returns numerical output, it counts only given string as +1.
// Usefull for checkbox type with only one answer ("yes" for example)
// or for radio grups with "yes" and "no" options ("no" is ignored).
func (i *FormFuncs) SumOnly(FormFieldName, countOnlyThis string) template.HTML {
	return i.sum(FormFieldName, false, false, countOnlyThis)
}

func (i *FormFuncs) sum(FormFieldName string, sortByName, forceStringVals bool, countOnlyThis string) template.HTML {
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
		//log.Printf("field id: %v, nrs: %v", id, nrs) // DEBUG
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
	// log.Printf("Sum: strings I got: %v", strs) // DEBUG

	// there is special kind of string - miltiplication field, check for it
	// then it will be: 3 * 600 = 1200 and we are interested only in "3"
	if len(strs) > 0 {
		if strings.Contains(strs[0], " = ") && strings.Contains(strs[0], " * ") { // we are sure it is multiplictation
			//log.Printf("Sum: we have multiplication field") //DEBUQ
			sum := 0
			for i := range strs {
				am, _, _ := ConvertMultiplicationToAmmountMultiplTotal(strs[i])
				ami, err := strconv.Atoi(am)
				if err != nil {
					log.Printf("Sum: error converting multiplication field ammount %s into integer, %v", am, err)
				}
				sum += ami
			}
			return template.HTML(strconv.Itoa(sum))

		}
	}

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

	// if countOnlyThis is defined, then return number for answer named as given
	// f.e.: we are interested in only "yes" values
	if len(countOnlyThis) != 0 {
		return template.HTML(strconv.Itoa(m[countOnlyThis]))
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

// This shows data from db or "alternative" string. Used for checkboxes with only one answer "yes", alternative will be "no".
// Usefull for mail notifications.
func (i *FormFuncs) FieldOr(FormFieldName, Alternative string) string {
	res := i.Field(FormFieldName)
	if res == "" {
		return Alternative
	}
	return res
}

// this functions allows to get separated data from multiplicateField (3 * 500 = 1500)
// MAmmount return ammount (of tickets usually)
func (i *FormFuncs) MAmmount(FormFieldName string) string {
	s := i.Field(FormFieldName)
	amm, _, _ := ConvertMultiplicationToAmmountMultiplTotal(s)
	return amm
}

// MMultipl returns money it is multiplied by
func (i *FormFuncs) MMultipl(FormFieldName string) string {
	s := i.Field(FormFieldName)
	_, mult, _ := ConvertMultiplicationToAmmountMultiplTotal(s)
	return mult
}

// MTotal return total price to be payed
func (i *FormFuncs) MTotal(FormFieldName string) string {
	s := i.Field(FormFieldName)
	_, _, total := ConvertMultiplicationToAmmountMultiplTotal(s)
	return total
}

func (i *FormFuncs) generateQRimg(accountName string) (string, string) {
	return GenerateQRimg(accountName, i.DB, i.User, i.Template, i.NameSurname, "", i.FormID, "", "", i.Template.PayMessage)
}

func (i *FormFuncs) QRPay(accountName string) template.HTML {
	_, imgpath := i.generateQRimg(accountName)
	return template.HTML(fmt.Sprintf(`<img width="200" src="/%s" alt="QRError">`, imgpath))
}

// QRPayPrefixField like QRPay, but requires 3 argumenst and allows to:
// customize imgPrefix (avoid using the same qrcode file if multiple QRImages for one form),
// customize field name (again, when multiple QRcodes should be generated).
func (i *FormFuncs) QRPayFieldPrefix(accountName, field, imgPrefix string) template.HTML {
	_, imgpath := GenerateQRimg(accountName, i.DB, i.User, i.Template, i.NameSurname, "", i.FormID, field, imgPrefix, i.Template.PayMessage)
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

// QRPayMailFieldPrefix the same as QRPayPrefixField but for mail notifications
func (i *FormFuncs) QRPayMailFieldPrefix(accountName, field, imgPrefix string) template.HTML {
	imgname, imgpath := GenerateQRimg(accountName, i.DB, i.User, i.Template, i.NameSurname, "", i.FormID, field, imgPrefix, i.Template.PayMessage)
	i.EmbeddedImgs = append(i.EmbeddedImgs, EmbImg{
		NamePath: imgpath,
		CID:      getCID(imgname, i.User.URL),
	})
	return template.HTML(fmt.Sprintf(`<img width="150" src="cid:%s" alt="QR Kod Error" />`, getCID(imgname, i.User.URL)))
}

type OrderFuncs struct {
	EventID             int64
	EventName           string
	EventDate           string
	TotalPrice          string
	Sits, Rooms, Prices string
	SitsL, PricesL      []int
	RoomsL              []string
	Email               string
	Password            string
	Name, Surname       string
	Phone, Notes        string

	DB            *DB
	User          User
	NameSurname   string
	EmbeddedImgs  []EmbImg
	AttachedFiles []string

	// Preview - podgląd w edytorze maila: kod QR jako zwykły obrazek,
	// bez generowania biletów PDF (zaznaczamy tylko, że byłyby w załączniku)
	Preview     bool
	PDFAttached bool

	// Lang - język etykiet w danych do przelewu (.PayInfo), język imprezy
	Lang string

	// payPrefix - początek wiadomości dla odbiorcy z imprezy (complete)
	payPrefix string
}

func (o *OrderFuncs) generateQRimg(accountName string) (string, string) {
	etp := FormTemplate{}
	return GenerateQRimg(accountName, o.DB, o.User, etp, o.NameSurname, o.TotalPrice, -1, "", "", o.payPrefix)
}

func (o *OrderFuncs) QRPay(accountName string) template.HTML {
	_, imgpath := o.generateQRimg(accountName)
	return template.HTML(fmt.Sprintf(`<img width="200" src="/%s" alt="QRError">`, imgpath))
}

func (o *OrderFuncs) QRPayMail(accountName string) template.HTML {
	if o.Preview {
		// w podglądzie szablonu konto może być jeszcze niewybrane
		if _, err := o.DB.BankAccountGetByName(accountName, o.User.ID); err != nil {
			return template.HTML(`<span style="display:inline-block;width:150px;height:150px;border:2px dashed #b9c3cf;color:#6e7885;text-align:center;line-height:150px">QR</span>`)
		}
		return o.QRPay(accountName)
	}
	imgname, imgpath := o.generateQRimg(accountName)
	o.EmbeddedImgs = append(o.EmbeddedImgs, EmbImg{
		NamePath: imgpath,
		CID:      getCID(imgname, o.User.URL),
	})
	return template.HTML(fmt.Sprintf(`<img width="150" src="cid:%s" alt="QR Kod" />`, getCID(imgname, o.User.URL)))
}

// PDF generates and attaches generated pdf files
func (o *OrderFuncs) PDF(info string) template.HTML {
	o.PDFAttached = true
	if o.Preview {
		return template.HTML("")
	}
	p := &TicketParams{
		UserName:         o.User.Organization.String,
		UserURL:          o.User.URL,
		EventName:        o.EventName,
		EventDescription: info,
		SeatNrs:          o.SitsL,
		Rooms:            o.RoomsL,
	}

	attFiles, err := GenerateTicketsPDF(p)
	if err != nil {
		log.Printf("OrderFuncs-PDF: problem generating pdf ticket, %v", err)
	}

	o.AttachedFiles = append(o.AttachedFiles, attFiles...)
	return template.HTML("")
}

// GenerateQRimg prepare data for QR code generation.
// Options: when TotalPrice is given it is used, otherwise FormID need to be provided to determine money field name.
// If FieldName is given, it is used as FieldName or display name failback, but FormID is still needed.
// Function also handles multiplication field well (extracts sum from it).
// msgPrefix - początek wiadomości dla odbiorcy (z imprezy albo formularza).
func GenerateQRimg(accountName string, db *DB, u User, t FormTemplate, NameSurname, TotalPrice string, FormID int64, FieldName string, ImgPrefix string, msgPrefix string) (string, string) {
	ba, err := db.BankAccountGetByName(accountName, u.ID)
	if err != nil {
		log.Printf("GenerateQRimg: no account with this name found, name given by admin: %q, userID: %d(%s), %v", accountName, u.ID, u.URL, err)
	}

	templURL := strings.Join(strings.Fields(t.URL), "")

	imgname := fmt.Sprintf("%s.jpeg", templURL+"_"+NameSurname)
	if len(ImgPrefix) > 0 { //add given prefix to img name, allows for multiple QRs in one form
		imgname = fmt.Sprintf("%s.jpeg", templURL+"_"+ImgPrefix+"_"+NameSurname)
	}
	imgpath := getImgPath(MEDIAROOT, u.URL, MEDIAQRCODESUBDIR, imgname)

	eft := FormTemplate{}
	am := ""
	// get money ammount
	if t != eft { // it is form
		field := t.MoneyAmountFieldName.String
		if len(FieldName) > 0 { // if FieldName is given, use it
			field = FieldName
		}
		am, err = db.FormAnswerGetByFieldName(FormID, t.ID, field)
		if err != nil {
			am, err = db.FormAnswerGetByFieldDisplay(FormID, t.ID, field)
			if err != nil {
				log.Printf("GenerateQRimg: problem getting money ammount field name and display %q, FormID %d, %v", field, FormID, err)
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
	qr := NewQRPayment(ba.IBAN, ba.RecipientName.String, float64(ami), ba.Currency, payMessage(msgPrefix, NameSurname), ba.VarSymbol.Int64)
	log.Println(ba)
	err = qr.GenCode(imgpath)
	if err != nil {
		log.Printf("GenerateQRimg: error generating qr image, %v", err)
	}
	return imgname, imgpath
}

// ConvertMultiplicationToAmmountMultiplTotal returns:
// ammount, multiplier, sum
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

type ThankYou struct {
	FormFuncs
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

	of := orderFuncsFromOrder(o, db, user)

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

// getEmbeddedImgs takes delimited embedded images and returns []EmbImg
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
	return imgname
	//return fmt.Sprintf("%s@%s.cz", removeNonAlphanumeric(imgname), userURL)
}

func getImgPath(mediaroot, userdir, subdir, imgname string) string {
	return fmt.Sprintf("%s/%s/%s/%s", mediaroot, userdir, subdir, imgname)
}

// chooseEmail - use main mail if alt_email is not defined
func chooseEmail(main, alt string) string {
	if alt != "" {
		return alt
	}
	return main
}

// removeTrailingEmptyParagraphs removes empty nonsense added by quill editor.
// this functions should run on all quills outputs!
func removeTrailingEmptyParagraphs(html string) string {
	const trailing = "<p><br></p>"
	html = strings.TrimSpace(html) // optional: remove leading/trailing whitespace
	for strings.HasSuffix(html, trailing) {
		html = strings.TrimSuffix(html, trailing)
	}
	// Re-trim space if needed after removal
	return strings.TrimSpace(html)
}

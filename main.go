package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
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

	cookieStore := sessions.NewCookieStore(conf.AuthenticationKey, conf.EncryptionKey)
	cookieStore.Options = sessionOptions(false) // 2 godziny, patrz auth.go

	mailConf := MailConfig{Server: conf.MailServer, Port: int(conf.MailPort), From: conf.MailFrom, User: conf.MailUser, Pass: conf.MailPass, IgnoreCert: conf.MailIgnoreCert}

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
	rtr.HandleFunc("/api/events/{id:[0-9]+}/section", EventSectionSave(db, cookieStore))
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
	rtr.HandleFunc("/api/events/{id:[0-9]+}/reservations/mail", ReservationsMailAPI(db, mailConf, cookieStore)).Methods("POST")
	rtr.HandleFunc("/admin/catalogs", CatalogsPage(db, cookieStore))
	rtr.HandleFunc("/admin/mails/{id:[0-9]+}", MailPage(db, cookieStore))
	rtr.HandleFunc("/admin/accounts/{id:[0-9]+}", AccountPage(db, cookieStore))
	rtr.HandleFunc("/api/mails", MailCreateAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/mails/{id:[0-9]+}", MailSaveAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/mails/sources", MailSourcesAPI(db, cookieStore)).Methods("GET")
	rtr.HandleFunc("/api/mails/preview", MailSourcePreviewAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/media/banners", BannerFilesAPI(db, cookieStore)).Methods("GET", "POST")
	rtr.HandleFunc("/api/mails/{id:[0-9]+}/preview", MailPreviewAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/mails/{id:[0-9]+}/test", MailTestAPI(db, mailConf, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/accounts", AccountCreateAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/accounts/sources", AccountSourcesAPI(db, cookieStore)).Methods("GET")
	rtr.HandleFunc("/api/rooms", RoomCreateAPI(db, cookieStore)).Methods("POST")
	rtr.HandleFunc("/api/rooms/sources", RoomSourcesAPI(db, cookieStore)).Methods("GET")
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
	rtr.HandleFunc("/api/forms/{id:[0-9]+}/answers/mail", AnswersMailAPI(db, mailConf, cookieStore)).Methods("POST")
	rtr.HandleFunc("/locales/{file}", LocalesJSONHandler())

	// strony dla gości
	rtr.HandleFunc("/res/{user}", GuestReservationPage(db, cookieStore))
	rtr.HandleFunc("/order", GuestOrderPage(db))
	rtr.HandleFunc("/form/{userurl}/{formurl}", GuestFormPage(db, cookieStore))
	rtr.HandleFunc("/form/{userurl}/{formurl}/send", GuestFormSendAPI(db, mailConf)).Methods("POST")
	rtr.HandleFunc("/form/{userurl}/{formurl}/done", GuestFormDonePage(db))
	rtr.HandleFunc("/", AboutHTML())

	handleStatic("js")
	handleStatic("css")
	handleStatic("img")
	handleStatic("media") //user data
	http.Handle("/", rtr)
	http.HandleFunc("/order/status", ReservationOrderStatusHTML(db, mailConf))
	http.HandleFunc("/admin/login", AdminLoginHTML())
	http.HandleFunc("/admin/logout", AdminLogout(cookieStore))
	http.HandleFunc("/admin/devlogin", DevLogin(db, cookieStore))
	http.HandleFunc("/admin/lang", SetPanelLangHandler(db, cookieStore))
	http.HandleFunc("/api/login", LoginAPI(db, cookieStore))
	http.HandleFunc("/api/room", DesignerSetRoomSize(db, cookieStore))
	http.HandleFunc("/api/eventcopyinfo", EventCopyInfoAPI(db, cookieStore))
	http.HandleFunc("/api/eventcopy", EventCopyAPI(db, cookieStore))
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

func AboutHTML() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		lng := PanelLang(r)
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			ErrorHTML(T(lng, "about.not_found_title"), T(lng, "about.not_found"), lng, w, r)
			return
		}
		guestRender(w, lng, struct{ Title string }{T(lng, "app.name")}, "tmpl/n_about.html")
	}
}

// reallyEmpty - "" gdy w HTML nie ma żadnego tekstu (same puste akapity edytora)
func reallyEmpty(s string) string {
	if bluemonday.StripTagsPolicy().Sanitize(s) == "" {
		return ""
	}
	return s
}

func ErrorHTML(errorTitle, errorText, lang string, w http.ResponseWriter, r *http.Request) {
	guestRender(w, KnownLang(lang), nErrorVars{Title: errorTitle, Text: errorText, Back: true}, "tmpl/n_error.html")
}

// jsonMsg writes v as JSON response with given status code
func jsonMsg(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("jsonMsg: error encoding response, %v", err)
	}
}

// ToNS, ToNI - pusty napis i zero zapisujemy w bazie jako NULL
func ToNS(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func ToNI(i int64) sql.NullInt64 {
	return sql.NullInt64{Int64: i, Valid: i != 0}
}

func ToNB(b bool) sql.NullBool {
	return sql.NullBool{Bool: b, Valid: true}
}

func ToDate(unix int64) string {
	t := time.Unix(unix, 0)
	return t.Format("2006-01-02")
}

// ToUnix - data "2006-01-02" z formularza panelu (UTC)
func ToUnix(s string) (int64, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return -1, err
	}
	return t.Unix(), nil
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

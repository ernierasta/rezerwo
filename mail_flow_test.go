package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/http/httptest"
	gomail "net/mail"
	"regexp"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/wneessen/go-mail"
)

// Testy całej drogi maila: od żądania (formularz gościa, wysyłka z panelu,
// mail testowy, zamówienie) do gotowej wiadomości MIME, tak jak dostaje ją
// serwer SMTP. Treści maili są w bazie fragmentami HTML z edytora (bez
// <html>), więc każda wysłana wiadomość musi mieć część text/html i czystą
// część tekstową - inaczej program pocztowy pokazuje gościowi tagi.

// sentMail - przechwycona wiadomość, rozłożona na części
type sentMail struct {
	To, Subject  string
	Plain, HTML  string
	ContentTypes []string // typy wszystkich części (bez multipart/*)
}

// captureMails podmienia wysyłkę SMTP na zapis wiadomości do listy.
func captureMails(t *testing.T) *[]sentMail {
	t.Helper()
	var sent []sentMail
	orig := mailDeliver
	mailDeliver = func(c MailConfig, msg *mail.Msg) error {
		var buf bytes.Buffer
		if _, err := msg.WriteTo(&buf); err != nil {
			return err
		}
		m, err := parseMIME(buf.Bytes())
		if err != nil {
			t.Errorf("sent message is not valid MIME: %v\n%s", err, buf.String())
			return err
		}
		sent = append(sent, m)
		return nil
	}
	t.Cleanup(func() { mailDeliver = orig })
	return &sent
}

func parseMIME(raw []byte) (sentMail, error) {
	msg, err := gomail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return sentMail{}, err
	}
	dec := new(mime.WordDecoder)
	subject, _ := dec.DecodeHeader(msg.Header.Get("Subject"))
	m := sentMail{To: msg.Header.Get("To"), Subject: subject}
	err = walkPart(&m, msg.Header.Get("Content-Type"), msg.Header.Get("Content-Transfer-Encoding"), msg.Body)
	return m, err
}

func walkPart(m *sentMail, ctype, enc string, body io.Reader) error {
	mt, params, err := mime.ParseMediaType(ctype)
	if err != nil {
		return fmt.Errorf("content-type %q: %v", ctype, err)
	}
	if strings.HasPrefix(mt, "multipart/") {
		mr := multipart.NewReader(body, params["boundary"])
		for {
			p, err := mr.NextRawPart()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			if err := walkPart(m, p.Header.Get("Content-Type"), p.Header.Get("Content-Transfer-Encoding"), p); err != nil {
				return err
			}
		}
	}
	m.ContentTypes = append(m.ContentTypes, mt)
	switch strings.ToLower(enc) {
	case "base64":
		body = base64.NewDecoder(base64.StdEncoding, body)
	case "quoted-printable":
		body = quotedprintable.NewReader(body)
	}
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	switch mt {
	case "text/plain":
		m.Plain = string(b)
	case "text/html":
		m.HTML = string(b)
	}
	return nil
}

// editorTag - tagi, które edytor wstawia do treści; nie mogą trafić do
// części tekstowej
var editorTag = regexp.MustCompile(`(?i)</?(p|br|strong|b|em|img|table|tr|td|div|span|a|html|head|style)[\s>/]`)

// checkHTMLMail sprawdza, że mail wyjdzie jako HTML: jest część text/html
// (kompletny dokument ze stylem akapitów), część tekstowa bez tagów, nic nie
// jest podwójnie zakodowane, a treść zawiera podane teksty.
func checkHTMLMail(t *testing.T, what string, m sentMail, want ...string) {
	t.Helper()
	if m.HTML == "" {
		t.Errorf("%s: no text/html part (parts %v), mail client shows raw tags:\n%s", what, m.ContentTypes, m.Plain)
		return
	}
	if !strings.HasPrefix(m.HTML, mailHTMLHead) || !strings.HasSuffix(strings.TrimSpace(m.HTML), "</html>") {
		t.Errorf("%s: HTML part is not a full document with mailHTMLHead:\n%s", what, m.HTML)
	}
	if strings.Count(strings.ToLower(m.HTML), "<html") != 1 {
		t.Errorf("%s: HTML part wrapped twice:\n%s", what, m.HTML)
	}
	if tag := editorTag.FindString(m.Plain); tag != "" {
		t.Errorf("%s: text part contains tag %q:\n%s", what, tag, m.Plain)
	}
	if strings.Contains(m.HTML, "&lt;p&gt;") || strings.Contains(m.HTML, "&lt;strong&gt;") {
		t.Errorf("%s: editor HTML escaped, guest sees tags:\n%s", what, m.HTML)
	}
	for _, w := range want {
		if !strings.Contains(m.HTML, w) {
			t.Errorf("%s: HTML part lacks %q:\n%s", what, w, m.HTML)
		}
		if p := editorTag.ReplaceAllString(w, ""); !strings.Contains(m.Plain, p) {
			t.Errorf("%s: text part lacks %q:\n%s", what, p, m.Plain)
		}
	}
}

// mailsTo - maile wysłane na adres
func mailsTo(sent []sentMail, to string) []sentMail {
	var r []sentMail
	for _, m := range sent {
		if strings.Contains(m.To, to) {
			r = append(r, m)
		}
	}
	return r
}

// adminCookie - ciasteczko zalogowanego admina organizacji o tym adresie
func adminCookie(t *testing.T, cs *sessions.CookieStore, email string) *http.Cookie {
	t.Helper()
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	s, _ := cs.New(r, AUTHCOOKIE)
	s.Values["role"] = "admin"
	s.Values["email"] = email
	if err := s.Save(r, w); err != nil {
		t.Fatal(err)
	}
	return w.Result().Cookies()[0]
}

// editorMail - treść maila tak, jak zapisuje ją edytor (fragment HTML)
const editorMail = `<p>Dzień dobry,</p><p><br></p><p>dziękujemy. Imię: <strong>%s</strong></p><p><br></p><p>Pozdrawiamy</p>`

func TestMailFlowsSendHTML(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	db := newTestDB(t)
	sent := captureMails(t)
	conf := MailConfig{Server: "smtp.test", From: "system@example.com"}
	cs := sessions.NewCookieStore([]byte("0123456789abcdef0123456789abcdef"))

	userID := addUser(t, db, "org@example.com", "org")
	user, err := db.UserGetByEmail("org@example.com")
	if err != nil {
		t.Fatal(err)
	}
	addMail := func(name, related, text string) int64 {
		return must(t)(db.NotificationAdd(&Notification{Name: name, Type: "mail", RelatedTo: related,
			Title: ToNS("Temat " + name), Text: text, CreatedDate: 1, UserID: userID}))
	}

	// formularz z mailem dla gościa i dla administratora
	content := `[
	 {"type":"text","name":"name-REQUIRED","label":"Imię"},
	 {"type":"text","name":"surname-REQUIRED","label":"Nazwisko"},
	 {"type":"text","subtype":"email","name":"email-REQUIRED","label":"E-mail"},
	 {"type":"text","name":"uwagi","label":"Uwagi"}
	]`
	guestMail := addMail("gosc", "forms", fmt.Sprintf(editorMail, `{{.Name}}`)+`<p>Uwagi: <strong>{{.Field "uwagi"}}</strong></p>`)
	adminMail := addMail("admin", "forms", fmt.Sprintf(editorMail, `{{.Name}} {{.Surname}}`))
	templID := must(t)(db.FormTemplateAdd(&FormTemplate{Name: "Deklaracja", URL: "dekl", Content: ToNS(content),
		UserID: userID, CreatedDate: 1, NotificationID: ToNI(guestMail)}))
	if _, err := db.DB.Exec(`UPDATE formtemplates SET admin_notifications_id_fk=$1 WHERE id=$2`, adminMail, templID); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"name-REQUIRED", "surname-REQUIRED", "email-REQUIRED", "uwagi"} {
		must(t)(db.FormFieldAdd(&FormField{Name: f, Display: f, Type: "text", FormTemplateID: templID}))
	}

	// impreza z mailami po zamówieniu i jedną zamówioną rezerwacją
	eventMail := addMail("impreza", "events", fmt.Sprintf(editorMail, `{{.Name}}`)+`<p>Miejsca: <strong>{{.Sits}}</strong></p>`)
	eventAdminMail := addMail("impreza-admin", "events", fmt.Sprintf(editorMail, `{{.Name}} {{.Surname}}`))
	eventID := must(t)(db.EventAdd(&Event{Name: "Bal", Date: 2000, FromDate: 1000, ToDate: 1900,
		DefaultPrice: 100, DefaultCurrency: "Kč", UserID: userID, Language: ToNS("pl"),
		ThankYouNotificationsID: ToNI(eventMail), AdminNotificationsID: ToNI(eventAdminMail)}))
	roomID := addRoom(t, db, "sala", 2)
	if err := db.RoomAddToEventUnsafe(roomID, eventID); err != nil {
		t.Fatal(err)
	}
	custID := must(t)(db.CustomerAdd(&Customer{Email: "kupiec@example.com", Name: ToNS("Ewa"), Surname: ToNS("Nowak")}))
	resID := must(t)(db.ReservationAdd(&Reservation{Status: "ordered", FurnitureID: chair(t, db, roomID, 1),
		EventID: eventID, CustomerID: custID, Price: ToNI(100), Currency: ToNS("Kč")}))

	rtr := mux.NewRouter()
	rtr.HandleFunc("/form/{userurl}/{formurl}/send", GuestFormSendAPI(db, conf))
	rtr.HandleFunc("/api/forms/{id:[0-9]+}/answers/mail", AnswersMailAPI(db, conf, cs))
	rtr.HandleFunc("/api/events/{id:[0-9]+}/reservations/mail", ReservationsMailAPI(db, conf, cs))
	rtr.HandleFunc("/api/mails/{id:[0-9]+}/test", MailTestAPI(db, conf, cs))
	cookie := adminCookie(t, cs, "org@example.com")
	post := func(url string, body interface{}, admin bool) {
		t.Helper()
		b, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", url, bytes.NewReader(b))
		if admin {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		rtr.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("POST %s: %d %s", url, w.Code, w.Body.String())
		}
	}
	// expect bierze maile wysłane od ostatniego wywołania i sprawdza ich liczbę
	seen := 0
	expect := func(what string, n int) []sentMail {
		t.Helper()
		got := (*sent)[seen:]
		seen = len(*sent)
		if len(got) != n {
			t.Fatalf("%s: %d mails sent, want %d", what, len(got), n)
		}
		return got
	}

	t.Run("guest form", func(t *testing.T) {
		post("/form/org/dekl/send", gformSendMsg{Data: []FormAnswersJson{
			{Name: "name-REQUIRED", UserData: []string{"Jan"}}, {Name: "surname-REQUIRED", UserData: []string{"Kowalski"}},
			{Name: "email-REQUIRED", UserData: []string{"jan@example.com"}}, {Name: "uwagi", UserData: []string{"a <script>x</script> & d"}},
		}}, false)
		got := expect("guest form", 2)
		g := mailsTo(got, "jan@example.com")
		if len(g) != 1 {
			t.Fatalf("guest mail recipients: %v", got)
		}
		checkHTMLMail(t, "guest form mail", g[0], "<strong>Jan</strong>", "popraw odpowiedzi")
		if strings.Contains(g[0].HTML, "<script>") || !strings.Contains(g[0].HTML, "&lt;script&gt;") {
			t.Errorf("guest answer inserted into mail unescaped:\n%s", g[0].HTML)
		}
		a := mailsTo(got, "org@example.com")
		if len(a) != 1 {
			t.Fatalf("admin mail recipients: %v", got)
		}
		checkHTMLMail(t, "form admin mail", a[0], "<strong>Jan Kowalski</strong>")
	})

	t.Run("form answers mail from panel", func(t *testing.T) {
		var formID int64
		if err := db.DB.Get(&formID, `SELECT id FROM forms WHERE email='jan@example.com'`); err != nil {
			t.Fatal(err)
		}
		post(fmt.Sprintf("/api/forms/%d/answers/mail", templID), nResIDsMsg{IDs: []int64{formID}, NotificationID: guestMail}, true)
		checkHTMLMail(t, "form answers mail", expect("answers mail", 1)[0], "<strong>Jan</strong>")
	})

	t.Run("reservations mail from panel", func(t *testing.T) {
		post(fmt.Sprintf("/api/events/%d/reservations/mail", eventID), nResIDsMsg{IDs: []int64{resID}, NotificationID: eventMail}, true)
		m := expect("reservations mail", 1)[0]
		if !strings.Contains(m.To, "kupiec@example.com") {
			t.Errorf("reservations mail to %q", m.To)
		}
		checkHTMLMail(t, "reservations mail", m, "<strong>Ewa</strong>")
	})

	t.Run("test mail", func(t *testing.T) {
		post(fmt.Sprintf("/api/mails/%d/test", eventMail), nMailPreviewMsg{Subject: "Temat", Text: fmt.Sprintf(editorMail, "x"), RelatedTo: "events"}, true)
		checkHTMLMail(t, "event test mail", expect("event test mail", 1)[0], "<strong>x</strong>")
		post(fmt.Sprintf("/api/mails/%d/test", guestMail), nMailPreviewMsg{Subject: "Temat", Text: fmt.Sprintf(editorMail, "y"), RelatedTo: "forms", FormID: templID}, true)
		checkHTMLMail(t, "form test mail", expect("form test mail", 1)[0], "<strong>y</strong>")
	})

	t.Run("order mails", func(t *testing.T) {
		event, err := db.EventGetByID(eventID)
		if err != nil {
			t.Fatal(err)
		}
		sendOrderMails(db, conf, event, user, Order{EventID: eventID, Sits: "1", Prices: "100", Rooms: "sala",
			TotalPrice: "100", Email: "kupiec@example.com", Name: "Ewa", Surname: "Nowak"})
		got := expect("order mails", 2)
		checkHTMLMail(t, "order guest mail", mailsTo(got, "kupiec@example.com")[0], "<strong>Ewa</strong>", "<strong>1</strong>")
		checkHTMLMail(t, "order admin mail", mailsTo(got, "org@example.com")[0], "<strong>Ewa Nowak</strong>")
	})
}

// TestMailMessage - sama budowa wiadomości: fragment z edytora idzie jako
// HTML, zwykły tekst jako text/plain.
func TestMailMessage(t *testing.T) {
	sent := captureMails(t)
	conf := MailConfig{Server: "smtp.test"}
	for _, text := range []string{
		fmt.Sprintf(editorMail, "Jan"),
		"<html><body>" + fmt.Sprintf(editorMail, "Jan") + "</body></html>",
	} {
		if err := conf.Send(Mail{From: "a@example.com", To: []string{"b@example.com"}, Subject: "s", Text: text}); err != nil {
			t.Fatal(err)
		}
	}
	checkHTMLMail(t, "editor fragment", (*sent)[0], "<strong>Jan</strong>")
	if h := (*sent)[1].HTML; !strings.Contains(h, "<body><p>Dzień dobry") || strings.Count(h, "<html") != 1 {
		t.Errorf("full document changed:\n%s", h)
	}

	if err := conf.Send(Mail{From: "a@example.com", To: []string{"b@example.com"}, Subject: "s", Text: "Cena < 100 Kč"}); err != nil {
		t.Fatal(err)
	}
	if m := (*sent)[2]; m.HTML != "" || m.Plain != "Cena < 100 Kč" {
		t.Errorf("plain text mail: %+v", m)
	}
}

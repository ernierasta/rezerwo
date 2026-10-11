package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// Wartości z formularza gościa sprawdzane na serwerze (checkValues), tak jak
// robi to strona w JS - żądanie można wysłać z pominięciem przeglądarki.

func TestFormCheckValues(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	defs, err := ParseFormDef(`[
	 {"type":"text","subtype":"email","name":"email-REQUIRED","label":"E-mail"},
	 {"type":"number","name":"ile","label":"Ile","value":"0"},
	 {"type":"number","name":"co50","label":"Co 50","step":50,"value":"0"},
	 {"type":"number","name":"zakres","label":"Zakres","min":"1","max":"5"},
	 {"type":"number","name":"kwota","label":"Kwota"},
	 {"type":"number","name":"dowolna","label":"Dowolna","step":"any"}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	vis := []bool{true, true, true, true, true, true}
	for _, c := range []struct {
		field, value, bad string
	}{
		{"email-REQUIRED", "jan@example.com", ""},
		{"email-REQUIRED", "zly-adres", "E-mail"},
		{"ile", "3", ""},
		{"ile", "abc", "Ile"},
		{"ile", "1e309", "Ile"},
		{"ile", "12.5", "Ile"},
		{"ile", "-5", ""}, // bez min ujemne wolno (jak w przeglądarce)
		{"co50", "100", ""},
		{"co50", "120", "Co 50"},
		{"zakres", "0", "Zakres"},
		{"zakres", "6", "Zakres"},
		{"zakres", "5", ""},
		{"kwota", "-500", "Kwota"}, // pole kwoty do zapłaty: min 0
		{"kwota", "500", ""},
		{"dowolna", "0.25", ""},
		{"ile", "", ""},
	} {
		data := []FormAnswersJson{{Name: c.field, UserData: []string{c.value}}}
		if bad, why := checkValues(defs, vis, data, "kwota", "pl"); bad != c.bad {
			t.Errorf("%s=%q: bad %q (%s), want %q", c.field, c.value, bad, why, c.bad)
		}
	}
	// pole ukryte warunkiem nie jest sprawdzane
	hidden := []bool{true, false, true, true, true, true}
	if bad, _ := checkValues(defs, hidden, []FormAnswersJson{{Name: "ile", UserData: []string{"abc"}}}, "", "pl"); bad != "" {
		t.Errorf("hidden field checked: %q", bad)
	}
}

func TestCSVCell(t *testing.T) {
	for in, want := range map[string]string{
		"Jan": "Jan", "": "", "-5": "-5", "+420 777 123 456": "+420 777 123 456", "12,5": "12,5",
		`=HYPERLINK("http://x")`: `'=HYPERLINK("http://x")`, "@SUM(A1)": "'@SUM(A1)", "-1+cmd": "'-1+cmd", "+x": "'+x",
	} {
		if got := csvCell(in); got != want {
			t.Errorf("csvCell(%q) = %q, want %q", in, got, want)
		}
	}
}

// Wcześniejsze odpowiedzi widzi tylko ten, kto udowodni, że to jego
// zgłoszenie: ciasteczko z przeglądarki albo link z maila, przez EDITDAYS dni.
func TestGuestFormResubmit(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	db := newTestDB(t)
	userID := addUser(t, db, "org@example.com", "org")
	content := `[
	 {"type":"text","name":"name-REQUIRED","label":"Imię"},
	 {"type":"text","name":"surname-REQUIRED","label":"Nazwisko"},
	 {"type":"text","subtype":"email","name":"email-REQUIRED","label":"E-mail"},
	 {"type":"checkbox-group","name":"pomoc","label":"Pomoc","other":true,"values":[{"label":"Bufet","value":"bufet"},{"label":"Nic","value":"nic"}]},
	 {"type":"number","name":"ile","label":"Ile"}
	]`
	templID := must(t)(db.FormTemplateAdd(&FormTemplate{Name: "Deklaracja", URL: "dekl", Content: ToNS(content), UserID: userID, CreatedDate: 1}))
	for _, f := range []string{"name-REQUIRED", "surname-REQUIRED", "email-REQUIRED", "pomoc", "ile"} {
		must(t)(db.FormFieldAdd(&FormField{Name: f, Display: f, Type: "text", FormTemplateID: templID}))
	}
	cs := sessions.NewCookieStore([]byte("0123456789abcdef0123456789abcdef"))
	router := mux.NewRouter()
	router.HandleFunc("/form/{userurl}/{formurl}/send", GuestFormSendAPI(db, MailConfig{}))
	router.HandleFunc("/form/{userurl}/{formurl}", GuestFormPage(db, cs))

	var cookie *http.Cookie // ciasteczko przeglądarki gościa
	type sub struct {
		name, surname, email, pomoc, ile, edit string
		confirm, withCookie                    bool
	}
	send := func(x sub) (int, map[string]interface{}) {
		t.Helper()
		body, _ := json.Marshal(gformSendMsg{Confirm: x.confirm, Edit: x.edit, Data: []FormAnswersJson{
			{Name: "name-REQUIRED", UserData: []string{x.name}}, {Name: "surname-REQUIRED", UserData: []string{x.surname}},
			{Name: "email-REQUIRED", UserData: []string{x.email}}, {Name: "pomoc", UserData: strings.Split(x.pomoc, ",")},
			{Name: "ile", UserData: []string{x.ile}},
		}})
		r := httptest.NewRequest("POST", "/form/org/dekl/send", bytes.NewReader(body))
		if x.withCookie && cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		for _, c := range w.Result().Cookies() {
			cookie = c
		}
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		return w.Code, resp
	}
	count := func() (n int) {
		db.DB.Get(&n, `SELECT count(*) FROM forms`)
		return
	}
	value := func(formID int64, field string) (v string) {
		db.DB.Get(&v, `SELECT fa.value FROM formanswers fa JOIN formfields ff ON ff.id=fa.formfields_id_fk WHERE ff.name=$1 AND fa.forms_id_fk=$2`, field, formID)
		return
	}
	jan := sub{name: "Jan", surname: "Kowalski", email: "jan@example.com", pomoc: "bufet", ile: "2"}

	code, r := send(jan)
	if code != 200 || cookie == nil || !strings.HasPrefix(cookie.Path, "/form/org/dekl") {
		t.Fatalf("first submit: %d %v cookie %v", code, r, cookie)
	}
	id := int64(r["id"].(float64))

	// bez dowodu: odmowa, żadnych odpowiedzi w odpowiedzi serwera
	x := jan
	x.ile = "3"
	if code, r := send(x); code != 409 || r["field"] != ROLEEMAIL || r["changes"] != nil {
		t.Fatalf("same e-mail without proof: %d %v", code, r)
	}
	// ciasteczko: te same odpowiedzi
	x = jan
	x.withCookie = true
	if _, r := send(x); r["same"] != true {
		t.Fatalf("cookie, same answers: %v", r)
	}
	// ciasteczko: zmiany do potwierdzenia, z etykietami opcji i "Inne"
	x.pomoc, x.ile = "nic,Sprzątanie", "3"
	_, r = send(x)
	got, _ := json.Marshal(r["changes"])
	if want := `[{"label":"Pomoc","new":"Nic, Sprzątanie","old":"Bufet"},{"label":"Ile","new":"3","old":"2"}]`; r["confirm"] != true || string(got) != want {
		t.Fatalf("cookie, changes: %v %s", r, got)
	}
	if value(id, "ile") != "2" {
		t.Fatal("saved before confirmation")
	}
	x.confirm = true
	if code, _ := send(x); code != 200 || count() != 1 || value(id, "ile") != "3" {
		t.Fatalf("confirmed: %d forms %d ile %q", code, count(), value(id, "ile"))
	}
	// druga osoba w tej samej przeglądarce, z własnym adresem - nowe zgłoszenie
	anna := sub{name: "Anna", surname: "Nowak", email: "anna@example.com", pomoc: "nic", ile: "1", withCookie: true}
	if code, _ := send(anna); code != 200 || count() != 2 {
		t.Fatalf("second person, own e-mail: %d forms %d", code, count())
	}
	// link z maila (inne urządzenie, bez ciasteczka)
	x = jan
	x.edit, x.ile, x.pomoc, x.confirm = editToken(id), "4", "nic,Sprzątanie", true
	if code, r := send(x); code != 200 || value(id, "ile") != "4" {
		t.Fatalf("edit link: %d %v ile %q", code, r, value(id, "ile"))
	}
	// link z maila, ale adres innego zgłoszenia
	x.email = "anna@example.com"
	if code, _ := send(x); code != 409 {
		t.Fatalf("edit link with other submission's e-mail: %d", code)
	}
	// podrobiony token = brak dowodu
	x = jan
	x.edit = fmt.Sprintf("%d.%s", id, strings.Repeat("0", 24))
	if code, _ := send(x); code != 409 {
		t.Fatalf("forged token: %d", code)
	}
	// strona z linku: zapisane odpowiedzi do wpisania w pola
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/form/org/dekl?edit="+editToken(id), nil))
	if page := w.Body.String(); !strings.Contains(page, `data-edit=`) || !strings.Contains(page, `&#34;ile&#34;:{&#34;v&#34;:[&#34;4&#34;]}`) ||
		!strings.Contains(page, `&#34;o&#34;:&#34;Sprzątanie&#34;`) {
		t.Fatalf("edit page without prefill: %s", page[strings.Index(page, `id="gform"`):][:600])
	}

	// po EDITDAYS dniach: koniec poprawek
	if _, err := db.DB.Exec(`UPDATE forms SET created_date=$1`, nowUnix()-(EDITDAYS+1)*24*3600); err != nil {
		t.Fatal(err)
	}
	x = jan
	x.withCookie, x.confirm = true, true
	if code, r := send(x); code != 409 || !strings.Contains(fmt.Sprint(r["msg"]), "org@example.com") {
		t.Fatalf("late, cookie: %d %v", code, r)
	}
	x.edit = editToken(id)
	if code, _ := send(x); code != 409 || value(id, "ile") != "4" {
		t.Fatalf("late, edit link: %d", code)
	}
}

func TestAddEditLink(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	got := addEditLink("<html><body><p>Dzięki</p></body></html>", "pl", "https://x/form/a/b?edit=1.ab&x=1", 0)
	if !strings.Contains(got, `<a href="https://x/form/a/b?edit=1.ab&amp;x=1">`) || !strings.HasSuffix(got, "</p></body></html>") {
		t.Errorf("addEditLink: %s", got)
	}
}

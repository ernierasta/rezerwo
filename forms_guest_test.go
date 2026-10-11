package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
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

// Ten sam e-mail drugi raz: ta sama osoba widzi zmiany i potwierdza,
// ktoś inny dostaje prośbę o inny adres.
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
	 {"type":"select","name":"pomoc","label":"Pomoc","values":[{"label":"Bufet","value":"bufet"},{"label":"Nic","value":"nic"}]},
	 {"type":"number","name":"ile","label":"Ile"}
	]`
	templID := must(t)(db.FormTemplateAdd(&FormTemplate{Name: "Deklaracja", URL: "dekl", Content: ToNS(content), UserID: userID, CreatedDate: 1}))
	for _, f := range []string{"name-REQUIRED", "surname-REQUIRED", "email-REQUIRED", "pomoc", "ile"} {
		must(t)(db.FormFieldAdd(&FormField{Name: f, Display: f, Type: "text", FormTemplateID: templID}))
	}
	router := mux.NewRouter()
	router.HandleFunc("/form/{userurl}/{formurl}/send", GuestFormSendAPI(db, MailConfig{}))

	send := func(name, surname, email, pomoc, ile string, confirm bool) (int, map[string]interface{}) {
		t.Helper()
		body, _ := json.Marshal(gformSendMsg{Confirm: confirm, Data: []FormAnswersJson{
			{Name: "name-REQUIRED", UserData: []string{name}}, {Name: "surname-REQUIRED", UserData: []string{surname}},
			{Name: "email-REQUIRED", UserData: []string{email}}, {Name: "pomoc", UserData: []string{pomoc}},
			{Name: "ile", UserData: []string{ile}},
		}})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("POST", "/form/org/dekl/send", bytes.NewReader(body)))
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		return w.Code, resp
	}
	count := func() (n int) {
		db.DB.Get(&n, `SELECT count(*) FROM forms`)
		return
	}
	value := func(field string) (v string) {
		db.DB.Get(&v, `SELECT fa.value FROM formanswers fa JOIN formfields ff ON ff.id=fa.formfields_id_fk WHERE ff.name=$1`, field)
		return
	}

	if code, r := send("Jan", "Kowalski", "jan@example.com", "bufet", "2", false); code != 200 || r["id"] == nil {
		t.Fatalf("first submit: %d %v", code, r)
	}
	// te same odpowiedzi: nic nie zapisujemy, link do potwierdzenia
	if _, r := send("Jan", "Kowalski", "jan@example.com", "bufet", "2", false); r["same"] != true || r["done"] == nil {
		t.Fatalf("same answers: %v", r)
	}
	// wielkość liter i spacje nie zmieniają osoby (ale zmianę widać)
	if code, r := send("jan", " Kowalski ", "JAN@example.com", "bufet", "2", false); code != 200 || r["confirm"] != true {
		t.Fatalf("same person, other case: %d %v", code, r)
	}
	// zmiany: najpierw lista zmian z etykietami opcji, bez zapisu
	_, r := send("Jan", "Kowalski", "jan@example.com", "nic", "3", false)
	if r["confirm"] != true {
		t.Fatalf("changed answers: %v", r)
	}
	got, _ := json.Marshal(r["changes"])
	if want := `[{"label":"Pomoc","new":"Nic","old":"Bufet"},{"label":"Ile","new":"3","old":"2"}]`; string(got) != want {
		t.Errorf("changes: %s, want %s", got, want)
	}
	if value("ile") != "2" {
		t.Fatal("saved before confirmation")
	}
	// potwierdzone: to samo zgłoszenie nadpisane
	if code, _ := send("Jan", "Kowalski", "jan@example.com", "nic", "3", true); code != 200 || count() != 1 || value("ile") != "3" {
		t.Fatalf("confirmed resubmit: code %d, forms %d, ile %q", code, count(), value("ile"))
	}
	// ten sam adres, inna osoba
	if code, r := send("Anna", "Nowak", "jan@example.com", "nic", "1", true); code != 409 || r["field"] != ROLEEMAIL || count() != 1 {
		t.Fatalf("other person, same e-mail: %d %v (forms %d)", code, r, count())
	}
	// inny adres - nowe zgłoszenie
	if code, _ := send("Anna", "Nowak", "anna@example.com", "nic", "1", false); code != 200 || count() != 2 {
		t.Fatalf("new e-mail: %d, forms %d", code, count())
	}
}

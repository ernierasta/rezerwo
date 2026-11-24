package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// Formularz zamówienia w designerze pól (orderform.go): dane osoby jako pola
// systemowe, dawny układ składany w locie, a gość wysyła pola pod dawnymi
// nazwami.

func defNames(defs []FormDefField) []string {
	var out []string
	for _, d := range defs {
		if n := d.S("name"); n != "" {
			out = append(out, n)
		} else {
			out = append(out, d.S("type"))
		}
	}
	return out
}

func TestLegacyOrderDef(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	addons := []FormDefField{{"type": "text", "name": "rodziny-multi", "label": "Rodziny"}}
	for after, want := range map[string]string{
		"start": "rodziny-multi email-REQUIRED name-REQUIRED surname-REQUIRED phone-ROLE notes-ROLE",
		"email": "email-REQUIRED rodziny-multi name-REQUIRED surname-REQUIRED phone-ROLE notes-ROLE",
		"name":  "email-REQUIRED name-REQUIRED surname-REQUIRED rodziny-multi phone-ROLE notes-ROLE",
		"phone": "email-REQUIRED name-REQUIRED surname-REQUIRED phone-ROLE rodziny-multi notes-ROLE",
		"":      "email-REQUIRED name-REQUIRED surname-REQUIRED phone-ROLE notes-ROLE rodziny-multi",
		"???":   "email-REQUIRED name-REQUIRED surname-REQUIRED phone-ROLE notes-ROLE rodziny-multi",
	} {
		if got := strings.Join(defNames(LegacyOrderDef(addons, after, false, "pl", "")), " "); got != want {
			t.Errorf("after %q:\n got %s\nwant %s", after, got, want)
		}
	}

	// własna sekcja dodatków = nagłówek nad nimi; bez dodatków bez nagłówka
	defs := LegacyOrderDef(addons, "", true, "cs", "Sem napište přání")
	if got := strings.Join(defNames(defs), " "); got != "email-REQUIRED name-REQUIRED surname-REQUIRED phone-ROLE notes-ROLE header rodziny-multi" {
		t.Errorf("section: %s", got)
	}
	if defs[5].S("label") != T("cs", "addons.guest_title") {
		t.Errorf("section header label %q", defs[5].S("label"))
	}
	// etykiety w języku imprezy, opis uwag z events.order_notes_desc
	if defs[0].S("label") != T("cs", "order.email") || defs[0].S("description") != T("cs", "order.email_help") {
		t.Errorf("email field: %v", defs[0])
	}
	if defs[4].S("description") != "Sem napište přání" || defs[4].S("type") != "textarea" {
		t.Errorf("notes field: %v", defs[4])
	}
	if n := len(LegacyOrderDef(nil, "", true, "pl", "")); n != 5 {
		t.Errorf("no addons: %d fields, want 5", n)
	}
}

func TestNormalizeOrderDef(t *testing.T) {
	if err := LoadLocales(); err != nil {
		t.Fatal(err)
	}
	ev := Event{AddonsAfter: "email"}

	// dawna lista dodatków -> pełny formularz
	old := []FormDefField{{"type": "select", "name": "obiad", "label": "Obiad"}}
	if got := strings.Join(defNames(NormalizeOrderDef(old, ev)), " "); got != "email-REQUIRED obiad name-REQUIRED surname-REQUIRED phone-ROLE notes-ROLE" {
		t.Errorf("legacy: %s", got)
	}

	// zapisany formularz: telefon i uwagi usunięte zostają usunięte, brakujące
	// pola obowiązkowe wracają na początek, rodzaj pól systemowych jest stały
	saved := []FormDefField{
		{"type": "header", "subtype": "h2", "label": "Dane"},
		{"type": "select", "name": ROLEEMAIL, "label": "Mail", "required": false, "show_if": map[string]interface{}{"field": "x"}},
		{"type": "text", "name": "obiad", "label": "Obiad"},
	}
	got := NormalizeOrderDef(saved, ev)
	if s := strings.Join(defNames(got), " "); s != "name-REQUIRED surname-REQUIRED header email-REQUIRED obiad" {
		t.Errorf("saved: %s", s)
	}
	email := got[3]
	if email.S("type") != "text" || email.S("subtype") != "email" || !email.B("required") || email["show_if"] != nil {
		t.Errorf("email not fixed: %v", email)
	}
	if email.S("label") != "Mail" {
		t.Errorf("own label lost: %q", email.S("label"))
	}
}

func TestParseAddonFieldsSkipsSys(t *testing.T) {
	b, _ := json.Marshal(LegacyOrderDef([]FormDefField{{"type": "text", "name": "x", "label": "X", "description": "opis", "placeholder": "np. Y"}}, "", false, "pl", ""))
	ff := ParseAddonFields(string(b))
	if len(ff) != 1 || ff[0].Name != "x" || ff[0].Desc != "opis" || ff[0].Placeholder != "np. Y" {
		t.Fatalf("addon fields: %+v", ff)
	}
}

func TestOrderFormRows(t *testing.T) {
	defs := []FormDefField{
		{"type": "paragraph", "label": "Przeczytaj <b>to</b>"},
		{"type": "text", "subtype": "email", "name": ROLEEMAIL, "label": "E-mail"},
		{"type": "text", "name": ROLENAME, "label": "Imię", "className": "form-control row-1 col-md-6"},
		{"type": "text", "name": ROLESURNAME, "label": "Nazwisko", "className": "form-control row-1 col-md-6"},
		{"type": "select", "name": "kolacja-perseat", "label": "Kolacja", "className": "form-control row-2 col-md-6",
			"values": []interface{}{map[string]interface{}{"label": "ryba", "value": "ryba"}}},
		{"type": "multiplierField", "name": "parking", "label": "Parking", "multipl": "100"},
		{"type": "textarea", "name": ROLENOTES, "label": "Uwagi", "required": true},
	}
	rows, flat := orderFormRows(defs, 2)
	if len(rows) != 6 {
		t.Fatalf("rows: %d, want 6", len(rows))
	}
	if f := rows[0].Items[0].Field; f == nil || f.Type != "paragraph" {
		t.Errorf("row 0: %+v", rows[0].Items[0])
	}
	if f := rows[1].Items[0].Field; f.Name != "email" || f.ID != "email" || !f.Required {
		t.Errorf("email: %+v", f)
	}
	if it := rows[2].Items; len(it) != 2 || it[0].Field.Name != "name" || it[1].Field.Name != "surname" || it[0].Field.Width != 6 {
		t.Errorf("name row: %+v", it)
	}
	if it := rows[3].Items; len(it) != 2 || it[0].Addon.Input != "addon-kolacja-perseat-1" || it[1].Addon.Title != "Kolacja 2" || it[1].Addon.Width != 6 {
		t.Errorf("perseat row: %+v", it)
	}
	if f := rows[5].Items[0].Field; f.Name != "notes" || f.Type != "textarea" || !f.Required {
		t.Errorf("notes: %+v", f)
	}
	if len(flat) != 3 || !flat[2].IsPriced() {
		t.Errorf("flat addons: %+v", flat)
	}
}

// Cała droga gościa z formularzem zmienionym w designerze: nagłówek i tekst
// na stronie, bez telefonu, dodatki tyle razy, ile miejsc; dane osoby idą do
// customers, dodatki do formanswers.
func TestOrderFormGuestFlow(t *testing.T) {
	f := newGuestFixture(t)
	f.router.HandleFunc("/order/status", ReservationOrderStatusHTML(f.db, MailConfig{}))
	user, err := f.db.UserGetByID(f.userID)
	if err != nil {
		t.Fatal(err)
	}

	// impreza bez formularza: dawny stały układ
	body := f.post(t, "/order", url.Values{
		"event-id": {fmt.Sprint(f.eventID)}, "sits": {"3"}, "rooms": {fmt.Sprint(f.roomID)},
	})
	for _, want := range []string{`name="email"`, `name="name"`, `name="surname"`, `name="phone"`, `name="notes"`, T("pl", "order.email_help")} {
		if !strings.Contains(body, want) {
			t.Errorf("default form: missing %s", want)
		}
	}
	if _, err := f.db.DB.Exec(`DELETE FROM reservations`); err != nil {
		t.Fatal(err)
	}

	ev, _ := f.db.EventGetByID(f.eventID)
	defs := []FormDefField{
		{"type": "header", "subtype": "h2", "label": "Ważne informacje"},
		{"type": "paragraph", "label": "Bilety odbieracie przy wejściu."},
		{"type": "text", "subtype": "email", "name": ROLEEMAIL, "label": "Twój mail", "description": "na potwierdzenie"},
		{"type": "text", "name": ROLENAME, "label": "Imię"},
		{"type": "text", "name": ROLESURNAME, "label": "Nazwisko"},
		{"type": "select", "name": "obiad-perseat", "label": "Obiad",
			"values": []interface{}{map[string]interface{}{"label": "ryba", "value": "ryba"}, map[string]interface{}{"label": "mięso", "value": "mięso"}}},
	}
	if _, err := newOrderTemplate(f.db, user, ev, "Formularz", NormalizeOrderDef(defs, ev)); err != nil {
		t.Fatal(err)
	}

	lock := f.mark(t, f.roomID, 1, 2)
	if lock == "" {
		t.Fatal("seats not locked")
	}
	body = f.post(t, "/order", url.Values{
		"event-id": {fmt.Sprint(f.eventID)}, "sits": {"3"}, "rooms": {fmt.Sprint(f.roomID)},
	})
	for _, want := range []string{"Ważne informacje", "Bilety odbieracie przy wejściu.", "Twój mail", "na potwierdzenie", `name="email"`, `name="addon-obiad-perseat-1"`} {
		if !strings.Contains(body, want) {
			t.Errorf("own form: missing %s", want)
		}
	}
	for _, unwanted := range []string{`name="phone"`, `name="notes"`, "email-REQUIRED"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("own form: unexpected %s", unwanted)
		}
	}

	order := url.Values{
		"event-id": {fmt.Sprint(f.eventID)}, "sits": {"1,2"}, "rooms": {fmt.Sprintf("%d,%d", f.roomID, f.roomID)},
		"lock": {lock}, "total-price": {"200"},
		"email": {"nie-mail"}, "name": {"Jan"}, "surname": {"Nowak"},
		"addon-obiad-perseat-1": {"ryba"}, "addon-obiad-perseat-2": {"mięso"},
	}
	// zły e-mail: zamówienie nie przechodzi, miejsca dalej czekają na gościa
	f.post(t, "/order/status", order)
	if st, _ := f.seatStatus(t, 1); st != "marked" {
		t.Fatalf("seat 1 after bad e-mail: %q, want marked", st)
	}
	order.Set("email", " jan@example.com ")
	f.post(t, "/order/status", order)
	if st, _ := f.seatStatus(t, 1); st != "ordered" {
		t.Fatalf("seat 1: %q, want ordered", st)
	}

	var c Customer
	if err := f.db.DB.Get(&c, `SELECT c.* FROM customers c JOIN reservations r ON r.customers_id_fk=c.id WHERE r.events_id_fk=$1 LIMIT 1`, f.eventID); err != nil {
		t.Fatal(err)
	}
	if c.Email != "jan@example.com" || c.Name.String != "Jan" || c.Surname.String != "Nowak" {
		t.Errorf("customer: %+v", c)
	}
	var answers []string
	if err := f.db.DB.Select(&answers, `SELECT ff.name || '=' || fa.value FROM formanswers fa JOIN formfields ff ON ff.id=fa.formfields_id_fk ORDER BY fa.repeat_index`); err != nil {
		t.Fatal(err)
	}
	if strings.Join(answers, " ") != "obiad-perseat=ryba obiad-perseat=mięso" {
		t.Errorf("answers: %v", answers)
	}
}

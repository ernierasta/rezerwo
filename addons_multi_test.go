package main

// Pola dodatków, które gość dokłada sam przyciskiem "+" (końcówka -multi),
// np. nazwiska rodzin, dla których zamawia.

import (
	"strings"
	"testing"
)

const addonsMultiContent = `[
	{"type":"text","label":"Rodziny","name":"rodziny-multi","required":true},
	{"type":"select","label":"Obiad","name":"obiad",
	 "values":[{"label":"Mięsny","value":"miesny"},{"label":"Wege","value":"wege"}]},
	{"type":"multiplierField","label":"Porcja","name":"porcja-multi","multipl":"85"}
]`

func TestParseAddonFieldsMulti(t *testing.T) {
	ff := ParseAddonFields(addonsMultiContent)
	if len(ff) != 3 {
		t.Fatalf("pola: %+v", ff)
	}
	if !ff[0].Multi || ff[0].PerSeat {
		t.Errorf("pole z końcówką %q ma być dokładane przez gościa: %+v", MULTISUFFIX, ff[0])
	}
	if ff[1].Multi {
		t.Errorf("zwykłe pole nie może być dokładane: %+v", ff[1])
	}
	if ff[2].Multi {
		t.Errorf("pole z ceną nie może być dokładane (dopłata liczona raz): %+v", ff[2])
	}
}

// Puste wpisy wypadają, numeracja bez dziur, najwyżej tyle wpisów, ile miejsc
// (więcej nie przyjmujemy, nawet gdy ktoś dopisze pola ręcznie).
func TestCollectAddonAnswersMulti(t *testing.T) {
	ff := ParseAddonFields(addonsMultiContent)
	form := map[string][]string{
		"addon-rodziny-multi-1": {" Kowalscy "},
		"addon-rodziny-multi-2": {""},
		"addon-rodziny-multi-3": {"Rusnokowie"},
		"addon-rodziny-multi-4": {"Ponad limit"},
	}
	aa := CollectAddonAnswers(ff, form, 3)
	var got []string
	for _, a := range aa {
		got = append(got, a.Value+":"+strings.Repeat("I", int(a.Repeat)))
	}
	if strings.Join(got, ",") != "Kowalscy:I,Rusnokowie:II" {
		t.Errorf("odpowiedzi: %v", got)
	}

	page := orderAddonFields(ff, 3)
	if len(page) != 3 || page[0].Input != "addon-rodziny-multi-1" || page[0].Max != 3 {
		t.Errorf("pole na stronie gościa: %+v", page)
	}
}

// Odpowiedzi trafiają do raportów: przy zamówieniu (tabela rezerwacji, CSV)
// i jako lista wpisów z zamawiającym (zamiast liczenia "Kowalscy: 1").
func TestAddonMultiReports(t *testing.T) {
	db := newTestDB(t)
	userID := addUser(t, db, "org@example.com", "org")
	user, err := db.UserGetByID(userID)
	if err != nil {
		t.Fatal(err)
	}
	eventID := addEvent(t, db, userID, "Bal")
	roomID := addRoom(t, db, "sala", 3)
	if err := db.EventAddRoom(eventID, roomID); err != nil {
		t.Fatal(err)
	}
	templID := must(t)(db.FormTemplateAdd(&FormTemplate{Name: "Dodatki", URL: "dodatki",
		Content: ToNS(addonsMultiContent), CreatedDate: 1, UserID: userID, EventID: ToNI(eventID)}))
	if err := db.FormTemplateSetKind(templID, userID, ADDONSKIND); err != nil {
		t.Fatal(err)
	}
	if err := db.EventSetAddonsTemplate(eventID, userID, ToNI(templID)); err != nil {
		t.Fatal(err)
	}
	must(t)(db.FormFieldAdd(&FormField{Name: "rodziny-multi", Display: "Rodziny", Type: "text", FormTemplateID: templID}))
	must(t)(db.FormFieldAdd(&FormField{Name: "obiad", Display: "Obiad", Type: "select", FormTemplateID: templID}))
	event, err := db.EventGetByID(eventID)
	if err != nil {
		t.Fatal(err)
	}

	custID := must(t)(db.CustomerAdd(&Customer{Email: "jan@example.com", Name: ToNS("Jan"), Surname: ToNS("Nowak")}))
	if err := db.CustomerAppendToUser(userID, custID); err != nil {
		t.Fatal(err)
	}
	var resIDs []int64
	for i := int64(1); i <= 2; i++ {
		resIDs = append(resIDs, must(t)(db.ReservationAdd(&Reservation{Status: "ordered", OrderedDate: ToNI(1000),
			FurnitureID: chair(t, db, roomID, i), EventID: eventID, CustomerID: custID})))
	}
	orderID, _ := SaveOrderWithAddons(db, event, user, custID, resIDs, "", map[string][]string{
		"addon-rodziny-multi-1": {"Kowalscy"},
		"addon-rodziny-multi-2": {"Rusnokowie"},
		"addon-obiad":           {"wege"},
	})

	lines := eventAddonAnswersJSON(db, event)[orderID]
	if len(lines) != 2 || lines[0].Label != "Rodziny" || strings.Join(lines[0].Values, ", ") != "Kowalscy, Rusnokowie" ||
		lines[1].Label != "Obiad" || strings.Join(lines[1].Values, ",") != "wege" {
		t.Errorf("odpowiedzi przy zamówieniu: %+v", lines)
	}

	mr := addonMultiRows(db, event)
	if len(mr) != 2 || mr[0].Value != "Kowalscy" || mr[1].Value != "Rusnokowie" ||
		mr[0].Who != "Jan Nowak" || mr[0].Seats != 2 || mr[0].Field != "Rodziny" {
		t.Errorf("lista wpisów: %+v", mr)
	}

	// w liście dla kuchni rodzin nie liczymy, obiad tak
	v := nEventVars{}
	fillAddonSummary(db, event, "pl", &v)
	if len(v.AddonSummary) != 1 || v.AddonSummary[0].Field != "Obiad" {
		t.Errorf("lista dla kuchni: %+v", v.AddonSummary)
	}
	if len(v.MultiRows) != 2 {
		t.Errorf("MultiRows: %+v", v.MultiRows)
	}

	// pastylka {{.Addons}} w mailu i tekście po zamówieniu
	of := orderFuncsFromOrder(Order{EventID: eventID, OrderID: orderID, Name: "Jan"}, db, user)
	got := renderOrderTmpl("<p>{{.Addons}}</p>", of)
	for _, want := range []string{"Rodziny: <strong>Kowalscy, Rusnokowie</strong><br>", "Obiad: <strong>wege</strong><br>"} {
		if !strings.Contains(got, want) {
			t.Errorf(".Addons = %q, brak %q", got, want)
		}
	}
	if renderOrderTmpl("{{.Addons}}", orderFuncsFromOrder(Order{EventID: eventID, Name: "Jan"}, db, user)) != "" {
		t.Error(".Addons bez zamówienia ma być puste")
	}
}

// Dopłaty (pozycje z ceną) też trafiają do .Addons, ze znakami w nazwach
// pól zabezpieczonymi przed HTML.
func TestOrderAddonsHTMLItems(t *testing.T) {
	db := newTestDB(t)
	userID := addUser(t, db, "org@example.com", "org")
	eventID := addEvent(t, db, userID, "Bal")
	event, err := db.EventGetByID(eventID)
	if err != nil {
		t.Fatal(err)
	}
	custID := must(t)(db.CustomerAdd(&Customer{Email: "g@example.com"}))
	orderID := must(t)(db.OrderAdd(&CustomerOrder{CreatedDate: 1, EventID: eventID, CustomerID: custID}))
	must(t)(db.OrderItemAdd(&OrderItem{Kind: "addon", Description: "Parking <b>: 2 × 100", Amount: 2, Price: 100, Status: "ordered", CreatedDate: 1, OrderID: orderID}))
	got := string(orderAddonsHTML(db, event, []int64{orderID}))
	if got != "Parking &lt;b&gt;: <strong>2 × 100</strong><br>" {
		t.Errorf("orderAddonsHTML = %q", got)
	}
}

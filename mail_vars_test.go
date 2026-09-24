package main

// Testy danych zamówienia w mailach imprez (mail_vars.go). Działają na kopii
// bazy w tmp/. Uruchomienie: go test -run 'TestOrderFuncs|TestOrderSubject|TestCheckMailTemplate'

import (
	"strconv"
	"testing"
)

func TestOrderFuncsComplete(t *testing.T) {
	path := copyDBForTest(t)
	db := DBInit(path)
	db.MustConnect()
	defer db.Close()

	var room Room
	if err := db.DB.Get(&room, `SELECT * FROM rooms LIMIT 1`); err != nil {
		t.Skipf("brak sal w bazie: %v", err)
	}
	var event Event
	if err := db.DB.Get(&event, `SELECT * FROM events LIMIT 1`); err != nil {
		t.Skipf("brak imprez w bazie: %v", err)
	}
	roomID := strconv.FormatInt(room.ID, 10)

	// zamówienie gościa: napisy z formularza, sale jako ID
	of := orderFuncsFromOrder(Order{
		EventID: event.ID,
		Sits:    "12,13",
		Rooms:   roomID + "," + roomID,
		Prices:  "100,100",
	}, db, User{})

	if of.Sits != "12, 13" {
		t.Errorf("Sits = %q, chcę %q", of.Sits, "12, 13")
	}
	if of.Rooms != room.Name {
		t.Errorf("Rooms = %q, chcę nazwę sali %q", of.Rooms, room.Name)
	}
	if len(of.SitsL) != 2 || len(of.RoomsL) != 2 || len(of.PricesL) != 2 {
		t.Errorf("listy do PDF niepełne: %v %v %v", of.SitsL, of.RoomsL, of.PricesL)
	}
	if of.EventName != event.Name {
		t.Errorf("EventName = %q, chcę %q", of.EventName, event.Name)
	}

	// wysyłka z panelu: listy, napisy mają powstać z nich
	of = newEventOrderFuncs("Jan", "Kowalski", "", "jan@example.com", "", event.Name,
		[]int{5, 6}, []int{100, 100}, []string{"Sala A", "Sala B"}, event.ID, 200, User{}, db)
	if of.Sits != "5, 6" || of.Rooms != "Sala A, Sala B" {
		t.Errorf("z panelu: Sits = %q, Rooms = %q", of.Sits, of.Rooms)
	}
}

func TestOrderSubject(t *testing.T) {
	of := &OrderFuncs{Name: "Anna & Jan", Surname: "Nowak", TotalPrice: "300"}
	got := OrderSubject("Rezerwacja: {{.Name}} {{.Surname}}, {{.TotalPrice}}", of)
	if got != "Rezerwacja: Anna & Jan Nowak, 300" {
		t.Errorf("temat = %q", got)
	}
	if got := OrderSubject("bez zmiennych", of); got != "bez zmiennych" {
		t.Errorf("temat bez zmiennych = %q", got)
	}
	if got := OrderSubject("{{.Name", of); got != "{{.Name" {
		t.Errorf("błędny temat powinien zostać bez zmian, jest %q", got)
	}
}

func TestCheckMailTemplate(t *testing.T) {
	if err := checkMailTemplate("{{.Name}}", `<p>{{.QRPayMail "Konto \"A\" & B"}}</p>{{.PDF "Dom, 7.3."}}`); err != nil {
		t.Errorf("poprawny szablon odrzucony: %v", err)
	}
	if err := checkMailTemplate("", "<p>{{.Name</p>"); err == nil {
		t.Error("niedomknięta akcja przeszła")
	}
}

// Każdy język ma mieć te same, poprawne szablony (błędny plik jest po cichu
// pomijany w oknie "Nowy mail", więc łapiemy to tutaj).
func TestMailTemplatesParse(t *testing.T) {
	counts := map[string]int{}
	for _, lang := range []string{"pl", "cs", "en"} {
		tt := loadMailTemplates(lang)
		counts[lang] = len(tt)
		for _, tpl := range tt {
			if tpl.Subject == "" || tpl.Text == "" {
				t.Errorf("%s: pusty temat albo treść", tpl.Key)
			}
			if _, ok := findMailTemplate(tpl.Key); !ok {
				t.Errorf("%s: findMailTemplate nie znajduje", tpl.Key)
			}
		}
	}
	if counts["pl"] == 0 || counts["pl"] != counts["cs"] || counts["pl"] != counts["en"] {
		t.Errorf("liczba szablonów różni się między językami: %v", counts)
	}
}

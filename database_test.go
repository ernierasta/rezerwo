package main

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Testy metod bazy (database.go) na pustej bazie ze schematem produkcji.
// StructureCreate() jest nieaktualne, dlatego schemat pochodzi z
// testdata/schema.sql (zrzut `sqlite3 db.sql .schema`) - po każdej migracji
// trzeba go odświeżyć (polecenie w nagłówku testdata/schema.sql).

// newTestDB tworzy świeżą bazę w katalogu tymczasowym testu.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	schema, err := os.ReadFile("testdata/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	db := DBInit(filepath.Join(t.TempDir(), "test.db"))
	db.MustConnect()
	t.Cleanup(db.Close)
	if _, err := db.DB.Exec(string(schema)); err != nil {
		t.Fatalf("loading schema: %v", err)
	}
	return db
}

// must przerywa test przy błędzie metody Add, zwraca nowe ID:
// must(t)(db.UserAdd(u)).
func must(t *testing.T) func(int64, error) int64 {
	return func(id int64, err error) int64 {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
}

func addUser(t *testing.T, db *DB, email, url string) int64 {
	t.Helper()
	return must(t)(db.UserAdd(&User{Email: email, URL: url, Passwd: "hash"}))
}

// addRoom tworzy salę z jednym stołem i podaną liczbą krzeseł (numery od 1).
func addRoom(t *testing.T, db *DB, name string, chairs int) int64 {
	t.Helper()
	roomID := must(t)(db.RoomAdd(&Room{Name: name, Width: 800, Height: 600}))
	must(t)(db.FurnitureAdd(&Furniture{Number: 1, Type: "table", X: 10, Y: 10, RoomID: roomID}))
	for i := 1; i <= chairs; i++ {
		must(t)(db.FurnitureAdd(&Furniture{Number: int64(i), Type: "chair", X: int64(i * 30), Y: 50, RoomID: roomID}))
	}
	return roomID
}

func addEvent(t *testing.T, db *DB, userID int64, name string) int64 {
	t.Helper()
	return must(t)(db.EventAdd(&Event{
		Name: name, Date: 2000, FromDate: 1000, ToDate: 1900,
		DefaultPrice: 100, DefaultCurrency: "Kč", UserID: userID,
	}))
}

func chair(t *testing.T, db *DB, roomID, number int64) int64 {
	t.Helper()
	f, err := db.FurnitureGetByTypeNumberRoom("chair", number, roomID)
	if err != nil {
		t.Fatalf("chair %d in room %d: %v", number, roomID, err)
	}
	return f.ID
}

func TestUser(t *testing.T) {
	db := newTestDB(t)
	id := addUser(t, db, "org@example.com", "org")

	if _, err := db.UserAdd(&User{Email: "org@example.com", URL: "other", Passwd: "x"}); err == nil {
		t.Error("second user with the same email was added")
	}

	u, err := db.UserGetByURL("org")
	if err != nil || u.ID != id || u.Email != "org@example.com" {
		t.Errorf("UserGetByURL = %+v, %v", u, err)
	}
	if _, err := db.UserGetByEmail("none@example.com"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("unknown email: want sql.ErrNoRows, got %v", err)
	}

	u.Organization = ToNS("Koło Testowe")
	u.URL = "kolo"
	if err := db.UserMod(&u); err != nil {
		t.Fatal(err)
	}
	if u2, _ := db.UserGetByID(id); u2.URL != "kolo" || u2.Organization.String != "Koło Testowe" {
		t.Errorf("UserMod not saved: %+v", u2)
	}

	if err := db.UserSetPass("org@example.com", "newhash"); err != nil {
		t.Fatal(err)
	}
	if p, _ := db.UserGetPass("org@example.com"); p != "newhash" {
		t.Errorf("UserGetPass = %q, want newhash", p)
	}

	if err := db.UserDel("org@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := db.UserDel("org@example.com"); err == nil {
		t.Error("deleting a missing user returned no error")
	}
}

func TestAdminLanguage(t *testing.T) {
	db := newTestDB(t)
	userID := addUser(t, db, "org@example.com", "org")
	must(t)(db.AdminAdd(&Admin{Type: "admin", Email: "admin@example.com", UserID: userID}))

	if lang, err := db.AdminGetLanguage("admin@example.com"); err != nil || lang != "" {
		t.Errorf("new admin language = %q, %v; want empty", lang, err)
	}
	if err := db.AdminSetLanguage("admin@example.com", "cs"); err != nil {
		t.Fatal(err)
	}
	if lang, _ := db.AdminGetLanguage("admin@example.com"); lang != "cs" {
		t.Errorf("AdminGetLanguage = %q, want cs", lang)
	}
	if ee, _ := db.AdminGetEmails(userID); len(ee) != 1 || ee[0] != "admin@example.com" {
		t.Errorf("AdminGetEmails = %v", ee)
	}
}

func TestRoomAndFurniture(t *testing.T) {
	db := newTestDB(t)
	u1 := addUser(t, db, "a@example.com", "a")
	u2 := addUser(t, db, "b@example.com", "b")
	roomID := addRoom(t, db, "sala", 4)

	// meble są unikalne w sali po numerze i typie
	if _, err := db.FurnitureAdd(&Furniture{Number: 2, Type: "chair", RoomID: roomID}); err == nil {
		t.Error("duplicate chair number in one room was added")
	}
	if n, _ := db.FurnitureGetAllByRoomIDOfType(roomID, "chair"); len(n) != 4 {
		t.Errorf("chairs in room = %d, want 4", len(n))
	}

	f, _ := db.FurnitureGetByID(chair(t, db, roomID, 3))
	f.Label = ToNS("przy oknie")
	f.X = 500
	if err := db.FurnitureMod(&f); err != nil {
		t.Fatal(err)
	}
	if f2, _ := db.FurnitureGetByID(f.ID); f2.X != 500 || f2.Label.String != "przy oknie" {
		t.Errorf("FurnitureMod not saved: %+v", f2)
	}
	if err := db.FurnitureMod(&Furniture{}); err == nil {
		t.Error("FurnitureMod without ID returned no error")
	}
	if err := db.FurnitureDelByNumberTypeRoom(99, "chair", roomID); err == nil {
		t.Error("deleting a missing chair returned no error")
	}
	// mebel znika razem ze swoimi cenami
	evID := must(t)(db.EventAdd(&Event{Name: "bal", UserID: u1, DefaultCurrency: "Kč"}))
	must(t)(db.PriceAdd(&Price{Price: 100, Currency: "Kč", EventID: evID, FurnitureID: chair(t, db, roomID, 4)}))
	if err := db.FurnitureDelByNumberTypeRoom(4, "chair", roomID); err != nil {
		t.Fatal(err)
	}
	var prices int
	if err := db.DB.Get(&prices, `SELECT COUNT(*) FROM prices WHERE events_id_fk=$1`, evID); err != nil || prices != 0 {
		t.Errorf("prices of deleted chair = %d, %v", prices, err)
	}

	// sala ma najwyżej jednego właściciela
	if err := db.RoomAssignToUser(u1, roomID); err != nil {
		t.Fatal(err)
	}
	if err := db.RoomAssignToUser(u2, roomID); err == nil {
		t.Error("room was assigned to a second user")
	}
	if rr, _ := db.RoomGetAllByUserID(u1); len(rr) != 1 || rr[0].ID != roomID {
		t.Errorf("RoomGetAllByUserID(u1) = %+v", rr)
	}
	if rr, _ := db.RoomGetAllByUserID(u2); len(rr) != 0 {
		t.Errorf("RoomGetAllByUserID(u2) = %+v, want none", rr)
	}

	copyID, err := db.RoomWithFurnituresCopy(roomID, "sala - kopia")
	if err != nil {
		t.Fatal(err)
	}
	if copyID == roomID {
		t.Fatal("room copy has the original ID")
	}
	orig, _ := db.FurnitureGetAllByRoomID(roomID)
	copied, _ := db.FurnitureGetAllByRoomID(copyID)
	if len(copied) != len(orig) {
		t.Errorf("copied furnitures = %d, want %d", len(copied), len(orig))
	}
	if c, _ := db.FurnitureGetByID(chair(t, db, copyID, 3)); c.Label.String != "przy oknie" || c.X != 500 {
		t.Errorf("copied chair lost its attributes: %+v", c)
	}
	if r, _ := db.RoomGetByID(copyID); r.Name != "sala - kopia" || r.Width != 800 {
		t.Errorf("copied room = %+v", r)
	}
}

func TestEvent(t *testing.T) {
	db := newTestDB(t)
	u1 := addUser(t, db, "a@example.com", "a")
	u2 := addUser(t, db, "b@example.com", "b")
	eventID := addEvent(t, db, u1, "Bal")

	// nazwa imprezy jest unikalna tylko w obrębie organizacji
	if _, err := db.EventAdd(&Event{Name: "Bal", UserID: u1, DefaultCurrency: "Kč"}); err == nil {
		t.Error("second event with the same name was added for one user")
	}
	addEvent(t, db, u2, "Bal")
	if ok, _ := db.EventNameExists("Bal", u1); !ok {
		t.Error("EventNameExists = false for existing event")
	}
	if ok, _ := db.EventNameExists("Festyn", u1); ok {
		t.Error("EventNameExists = true for missing event")
	}

	e, err := db.EventGetByID(eventID)
	if err != nil {
		t.Fatal(err)
	}
	if e.Status.String != "published" || e.Theme.String != "" {
		t.Errorf("new event status/theme = %q/%q, want published/empty", e.Status.String, e.Theme.String)
	}

	// kolumny ustawiane osobnymi metodami zmienia tylko właściciel
	if err := db.EventSetTheme(eventID, u2, "klasyczny"); err != nil {
		t.Fatal(err)
	}
	if err := db.EventSetStatus(eventID, u1, "draft"); err != nil {
		t.Fatal(err)
	}
	if err := db.EventSetHideCosts(eventID, u1, true); err != nil {
		t.Fatal(err)
	}
	e, _ = db.EventGetByID(eventID)
	if e.Theme.String != "" {
		t.Errorf("other user changed theme to %q", e.Theme.String)
	}
	if e.Status.String != "draft" || !e.HideCosts.Bool {
		t.Errorf("status/hidecosts = %q/%v, want draft/true", e.Status.String, e.HideCosts.Bool)
	}

	// EventModByID nie kasuje kolumn, których nie zna
	e.MainDesc = ToNS("<p>opis</p>")
	if err := db.EventModByID(&e, u1); err != nil {
		t.Fatal(err)
	}
	if err := db.EventModByID(&e, u2); err == nil {
		t.Error("EventModByID accepted a different user")
	}
	e, _ = db.EventGetByID(eventID)
	if e.MainDesc.String != "<p>opis</p>" || e.Status.String != "draft" || !e.HideCosts.Bool {
		t.Errorf("after EventModByID: desc %q, status %q, hidecosts %v", e.MainDesc.String, e.Status.String, e.HideCosts.Bool)
	}

	if err := db.EventSetPreviousEdition(eventID, u1, ToNI(eventID)); err == nil {
		t.Error("event was set as its own previous edition")
	}
}

func TestEventRooms(t *testing.T) {
	db := newTestDB(t)
	userID := addUser(t, db, "a@example.com", "a")
	eventID := addEvent(t, db, userID, "Bal")
	big := addRoom(t, db, "duża", 1)
	small := addRoom(t, db, "mała", 1)

	// kolejność sal (po ID) decyduje, który opis room1desc/room2desc do której należy
	for _, id := range []int64{small, big} {
		if err := db.EventAddRoom(eventID, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.EventAddRoom(eventID, big); err == nil {
		t.Error("room was added to event twice")
	}
	rr, err := db.EventGetRooms(eventID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rr) != 2 || rr[0].ID != big || rr[1].ID != small {
		t.Errorf("EventGetRooms = %+v, want [duża mała]", rr)
	}

	if err := db.EventClearRooms(eventID); err != nil {
		t.Fatal(err)
	}
	if rr, _ := db.EventGetRooms(eventID); len(rr) != 0 {
		t.Errorf("rooms after EventClearRooms = %d", len(rr))
	}
}

func TestPriceCopyForRoomCopy(t *testing.T) {
	db := newTestDB(t)
	userID := addUser(t, db, "a@example.com", "a")
	oldEvent := addEvent(t, db, userID, "Bal 2025")
	newEvent := addEvent(t, db, userID, "Bal 2026")
	roomID := addRoom(t, db, "sala", 3)

	must(t)(db.PriceAdd(&Price{Price: 150, Currency: "Kč", EventID: oldEvent, FurnitureID: chair(t, db, roomID, 1)}))
	must(t)(db.PriceAdd(&Price{Price: 0, Currency: "Kč", Disabled: 1, EventID: oldEvent, FurnitureID: chair(t, db, roomID, 2)}))

	copyID, err := db.RoomWithFurnituresCopy(roomID, "")
	if err != nil {
		t.Fatal(err)
	}
	n, err := db.PriceCopyForEventRoom(oldEvent, roomID, newEvent, copyID)
	if err != nil || n != 2 {
		t.Fatalf("PriceCopyForEventRoom = %d, %v; want 2", n, err)
	}

	// ceny trafiają na krzesła kopii o tych samych numerach
	var got []Price
	if err := db.DB.Select(&got, `SELECT * FROM prices WHERE events_id_fk=$1 ORDER BY price`, newEvent); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 ||
		got[0].FurnitureID != chair(t, db, copyID, 2) || got[0].Disabled != 1 ||
		got[1].FurnitureID != chair(t, db, copyID, 1) || got[1].Price != 150 {
		t.Errorf("copied prices = %+v", got)
	}
}

func TestReservation(t *testing.T) {
	db := newTestDB(t)
	owner := addUser(t, db, "a@example.com", "a")
	other := addUser(t, db, "b@example.com", "b")
	eventID := addEvent(t, db, owner, "Bal")
	roomID := addRoom(t, db, "sala", 3)
	if err := db.EventAddRoom(eventID, roomID); err != nil {
		t.Fatal(err)
	}
	custID := must(t)(db.CustomerAdd(&Customer{Email: "guest@example.com", Name: ToNS("Jan")}))
	if err := db.CustomerAppendToUser(owner, custID); err != nil {
		t.Fatal(err)
	}

	reserve := func(number, ordered int64, status string) int64 {
		return must(t)(db.ReservationAdd(&Reservation{
			OrderedDate: ToNI(ordered), Price: ToNI(100), Currency: ToNS("Kč"), Status: status,
			FurnitureID: chair(t, db, roomID, number), EventID: eventID, CustomerID: custID,
		}))
	}
	r1 := reserve(1, 1100, "ordered")
	reserve(2, 1500, "ordered")
	reserve(3, 1200, "marked")

	// jedno krzesło można zarezerwować na imprezę tylko raz
	if _, err := db.ReservationAdd(&Reservation{Status: "ordered", FurnitureID: chair(t, db, roomID, 1), EventID: eventID, CustomerID: custID}); err == nil {
		t.Error("the same chair was reserved twice")
	}

	if err := db.ReservationSetStatus(r1, eventID, "payed", ToNI(1300)); err != nil {
		t.Fatal(err)
	}
	if err := db.ReservationSetStatus(r1, eventID+1, "payed", ToNI(1300)); err == nil {
		t.Error("ReservationSetStatus with a wrong event returned no error")
	}
	if r, _ := db.ReservationGet(chair(t, db, roomID, 1), eventID); r.Status != "payed" || r.PayedDate.Int64 != 1300 {
		t.Errorf("after ReservationSetStatus: %+v", r)
	}

	// sprzedane = ordered + payed, zamówione do podanej chwili
	if seats, sum, err := db.ReservationsSoldBefore(eventID, 1200); err != nil || seats != 1 || sum != 100 {
		t.Errorf("ReservationsSoldBefore(1200) = %d, %d, %v; want 1, 100", seats, sum, err)
	}
	if seats, _, _ := db.ReservationsSoldBefore(eventID, 9999); seats != 2 {
		t.Errorf("ReservationsSoldBefore(9999) = %d, want 2", seats)
	}

	full, err := db.ReservationFullGetAll(owner, eventID)
	if err != nil {
		t.Fatal(err)
	}
	if len(full) != 3 || full[0].ChairNumber != 1 || full[0].RoomName != "sala" || full[0].CustEmail.String != "guest@example.com" {
		t.Errorf("ReservationFullGetAll = %+v", full)
	}
	if full, _ := db.ReservationFullGetAll(other, eventID); len(full) != 0 {
		t.Errorf("other user sees %d reservations", len(full))
	}
	if cc, _ := db.CustomerGetAll(owner); len(cc) != 1 || cc[0].ID != custID {
		t.Errorf("CustomerGetAll = %+v", cc)
	}

	// zajęte krzesło chroni designer przed skasowaniem
	if n, _ := db.ReservationCountByFurnitureNumber(2, "chair", roomID); n != 1 {
		t.Errorf("ReservationCountByFurnitureNumber = %d, want 1", n)
	}

	// uwagi: pierwszy zapis tworzy notatkę, kolejny ją zmienia
	if err := db.ReservationSetNotes(r1, "wózek"); err != nil {
		t.Fatal(err)
	}
	if err := db.ReservationSetNotes(r1, "wózek inwalidzki"); err != nil {
		t.Fatal(err)
	}
	var notes []string
	if err := db.DB.Select(&notes, `SELECT text FROM notes`); err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || notes[0] != "wózek inwalidzki" {
		t.Errorf("notes = %q, want one updated note", notes)
	}
	if r, _ := db.ReservationFullGetByID(r1, eventID); r.Notes.String != "wózek inwalidzki" {
		t.Errorf("ReservationFullGetByID notes = %q", r.Notes.String)
	}

	for i, s := range []string{"ordered", "payed"} {
		must(t)(db.ReservationStatusLogAdd(&ReservationStatusLog{Date: int64(1000 + i), Status: s, ReservationID: r1}))
	}
	if ll, _ := db.ReservationStatusLogGet([]int64{r1}); len(ll) != 2 || ll[0].Status != "payed" {
		t.Errorf("ReservationStatusLogGet = %+v, want newest first", ll)
	}
}

func TestOrder(t *testing.T) {
	db := newTestDB(t)
	userID := addUser(t, db, "a@example.com", "a")
	eventID := addEvent(t, db, userID, "Bal")
	otherEvent := addEvent(t, db, userID, "Festyn")
	roomID := addRoom(t, db, "sala", 1)
	custID := must(t)(db.CustomerAdd(&Customer{Email: "guest@example.com"}))

	orderID := must(t)(db.OrderAdd(&CustomerOrder{CreatedDate: 1000, EventID: eventID, CustomerID: custID}))
	if _, err := db.OrderGetByID(orderID, otherEvent); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("order read through another event: %v", err)
	}

	resID := must(t)(db.ReservationAdd(&Reservation{Status: "ordered", FurnitureID: chair(t, db, roomID, 1), EventID: eventID, CustomerID: custID}))
	if err := db.ReservationSetOrder(resID, orderID); err != nil {
		t.Fatal(err)
	}
	if r, _ := db.ReservationGetByIDForNote(resID); r.OrderID.Int64 != orderID {
		t.Errorf("reservation order = %v, want %d", r.OrderID, orderID)
	}

	item := must(t)(db.OrderItemAdd(&OrderItem{Kind: "addon", Description: "obiad", Amount: 2, Price: 150, Status: "ordered", CreatedDate: 1000, OrderID: orderID}))
	must(t)(db.OrderItemAdd(&OrderItem{Kind: "surcharge", Description: "parking", Amount: 1, Price: 50, Status: "ordered", CreatedDate: 1001, OrderID: orderID}))

	if err := db.OrderItemSetStatus(item, orderID, "payed", ToNI(1200)); err != nil {
		t.Fatal(err)
	}
	if err := db.OrderItemSetStatus(item, orderID+1, "payed", ToNI(1200)); err == nil {
		t.Error("OrderItemSetStatus with a wrong order returned no error")
	}
	if i, _ := db.OrderItemGetByID(item, orderID); i.Status != "payed" || i.PayedDate.Int64 != 1200 {
		t.Errorf("item after OrderItemSetStatus = %+v", i)
	}

	if err := db.OrderItemDel(item, orderID+1); err == nil {
		t.Error("OrderItemDel with a wrong order returned no error")
	}
	if err := db.OrderItemDel(item, orderID); err != nil {
		t.Fatal(err)
	}
	if ii, _ := db.OrderItemsGetByEvent(eventID); len(ii) != 1 || ii[0].Description != "parking" {
		t.Errorf("OrderItemsGetByEvent = %+v", ii)
	}
	if ii, _ := db.OrderItemsGetByEvent(otherEvent); len(ii) != 0 {
		t.Errorf("other event has %d order items", len(ii))
	}
}

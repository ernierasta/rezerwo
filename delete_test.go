package main

import (
	"errors"
	"testing"
)

// count - ile wierszy zwraca zapytanie COUNT(*) z jednym parametrem.
func count(t *testing.T, db *DB, q string, arg int64) int64 {
	t.Helper()
	var n int64
	if err := db.DB.Get(&n, q, arg); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

func TestEventDelete(t *testing.T) {
	db := newTestDB(t)
	owner := addUser(t, db, "a@example.com", "a")
	other := addUser(t, db, "b@example.com", "b")
	eventID := addEvent(t, db, owner, "Bal")
	nextID := addEvent(t, db, owner, "Bal 2")
	roomID := addRoom(t, db, "sala", 2)
	if err := db.RoomAssignToUser(owner, roomID); err != nil {
		t.Fatal(err)
	}
	if err := db.EventAddRoom(eventID, roomID); err != nil {
		t.Fatal(err)
	}
	custID := must(t)(db.CustomerAdd(&Customer{Email: "guest@example.com"}))
	orderID := must(t)(db.OrderAdd(&CustomerOrder{CreatedDate: 1, EventID: eventID, CustomerID: custID}))
	must(t)(db.ReservationAdd(&Reservation{Status: "ordered", FurnitureID: chair(t, db, roomID, 1), EventID: eventID, CustomerID: custID, OrderID: ToNI(orderID)}))
	must(t)(db.PriceAdd(&Price{Price: 100, Currency: "Kč", EventID: eventID, FurnitureID: chair(t, db, roomID, 2)}))
	if _, err := db.DB.Exec(`UPDATE events SET previous_event_id=$1 WHERE id=$2`, eventID, nextID); err != nil {
		t.Fatal(err)
	}

	// cudzej imprezy nie skasujemy
	if err := db.EventDelete(eventID, other); err == nil {
		t.Error("EventDelete of another organization's event returned no error")
	}
	if err := db.EventDelete(eventID, owner); err != nil {
		t.Fatal(err)
	}
	for q, want := range map[string]int64{
		`SELECT COUNT(*) FROM events WHERE id=$1`:                 0,
		`SELECT COUNT(*) FROM reservations WHERE events_id_fk=$1`: 0,
		`SELECT COUNT(*) FROM orders WHERE events_id_fk=$1`:       0,
		`SELECT COUNT(*) FROM prices WHERE events_id_fk=$1`:       0,
		`SELECT COUNT(*) FROM events_rooms WHERE events_id_fk=$1`: 0,
		`SELECT COUNT(*) FROM events WHERE previous_event_id=$1`:  0,
	} {
		if n := count(t, db, q, eventID); n != want {
			t.Errorf("%s = %d, want %d", q, n, want)
		}
	}
	// sala, druga impreza i klient zostają
	if n := count(t, db, `SELECT COUNT(*) FROM rooms WHERE id=$1`, roomID); n != 1 {
		t.Error("room was deleted with the event")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM events WHERE id=$1`, nextID); n != 1 {
		t.Error("other event was deleted")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM customers WHERE id=$1`, custID); n != 1 {
		t.Error("customer was deleted with the event")
	}
}

func TestRoomDelete(t *testing.T) {
	db := newTestDB(t)
	owner := addUser(t, db, "a@example.com", "a")
	other := addUser(t, db, "b@example.com", "b")
	eventID := addEvent(t, db, owner, "Bal")
	roomID := addRoom(t, db, "sala", 2)
	if err := db.RoomAssignToUser(owner, roomID); err != nil {
		t.Fatal(err)
	}
	if err := db.EventAddRoom(eventID, roomID); err != nil {
		t.Fatal(err)
	}
	custID := must(t)(db.CustomerAdd(&Customer{Email: "guest@example.com"}))
	resID := must(t)(db.ReservationAdd(&Reservation{Status: "ordered", FurnitureID: chair(t, db, roomID, 1), EventID: eventID, CustomerID: custID}))

	if err := db.RoomDelete(roomID, other); err == nil {
		t.Error("RoomDelete of another organization's room returned no error")
	}
	// z rezerwacjami - nie
	if err := db.RoomDelete(roomID, owner); !errors.Is(err, ErrRoomHasReservations) {
		t.Fatalf("RoomDelete with reservations = %v, want ErrRoomHasReservations", err)
	}
	if names, _ := db.RoomEventNames(roomID); len(names) != 1 || names[0] != "Bal" {
		t.Errorf("RoomEventNames = %v", names)
	}
	if _, err := db.DB.Exec(`DELETE FROM reservations WHERE id=$1`, resID); err != nil {
		t.Fatal(err)
	}
	if err := db.RoomDelete(roomID, owner); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`SELECT COUNT(*) FROM rooms WHERE id=$1`,
		`SELECT COUNT(*) FROM furnitures WHERE rooms_id_fk=$1`,
		`SELECT COUNT(*) FROM events_rooms WHERE rooms_id_fk=$1`,
		`SELECT COUNT(*) FROM users_rooms WHERE rooms_id_fk=$1`,
	} {
		if n := count(t, db, q, roomID); n != 0 {
			t.Errorf("%s = %d after RoomDelete", q, n)
		}
	}
}

func TestMailAndAccountDelete(t *testing.T) {
	db := newTestDB(t)
	owner := addUser(t, db, "a@example.com", "a")
	other := addUser(t, db, "b@example.com", "b")
	eventID := addEvent(t, db, owner, "Bal")
	mailID := must(t)(db.NotificationAdd(&Notification{Name: "dzięki", Type: "thankyou", RelatedTo: "events", Text: "x", CreatedDate: 1, UserID: owner}))
	accID := must(t)(db.BankAccountAdd(&BankAccount{Name: "konto", IBAN: "CZ65", Currency: "CZK", UserID: owner}))
	if _, err := db.DB.Exec(`UPDATE events SET thankyou_notifications_id_fk=$1, admin_notifications_id_fk=$1, bankaccounts_id_fk=$2 WHERE id=$3`, mailID, accID, eventID); err != nil {
		t.Fatal(err)
	}

	if err := db.NotificationDelete(mailID, other); err == nil {
		t.Error("NotificationDelete of another organization's mail returned no error")
	}
	if err := db.BankAccountDelete(accID, other); err == nil {
		t.Error("BankAccountDelete of another organization's account returned no error")
	}
	if err := db.NotificationDelete(mailID, owner); err != nil {
		t.Fatal(err)
	}
	if err := db.BankAccountDelete(accID, owner); err != nil {
		t.Fatal(err)
	}
	e, err := db.EventGetByID(eventID)
	if err != nil {
		t.Fatal(err)
	}
	if e.ThankYouNotificationsID.Valid || e.AdminNotificationsID.Valid || e.BankAccountsID.Valid {
		t.Errorf("event still points to deleted mail/account: %v %v %v", e.ThankYouNotificationsID, e.AdminNotificationsID, e.BankAccountsID)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM notifications WHERE id=$1`, mailID); n != 0 {
		t.Error("mail not deleted")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM bankaccounts WHERE id=$1`, accID); n != 0 {
		t.Error("account not deleted")
	}
}

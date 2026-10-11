package main

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// Kroki zamówienia gościa są publiczne: cena z bazy, tylko miejsca z sal
// imprezy, wolne i w okresie sprzedaży, a blokadę dokańcza/zdejmuje tylko
// jej właściciel (order_check.go).

func (f *guestFixture) post(t *testing.T, path string, form url.Values) string {
	t.Helper()
	r := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, r)
	return w.Body.String()
}

func (f *guestFixture) mark(t *testing.T, room int64, seats ...int64) string {
	t.Helper()
	var ss, rr []string
	for _, s := range seats {
		ss = append(ss, fmt.Sprint(s))
		rr = append(rr, fmt.Sprint(room))
	}
	body := f.post(t, "/order", url.Values{
		"event-id": {fmt.Sprint(f.eventID)}, "sits": {strings.Join(ss, ",")}, "rooms": {strings.Join(rr, ",")},
		"prices": {"1"}, "total-price": {"1"}, "default-currency": {"Kč"},
	})
	m := regexp.MustCompile(`name="lock" id="lock" value="([^"]*)"`).FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return m[1]
}

func (f *guestFixture) seatStatus(t *testing.T, seat int64) (string, int64) {
	t.Helper()
	var st string
	var price int64
	err := f.db.DB.QueryRow(`SELECT r.status, r.price FROM reservations r WHERE r.events_id_fk=$1 AND r.furnitures_id_fk=$2`,
		f.eventID, chair(t, f.db, f.roomID, seat)).Scan(&st, &price)
	if err != nil {
		return "", 0
	}
	return st, price
}

func TestGuestOrderChecks(t *testing.T) {
	f := newGuestFixture(t)
	f.router.HandleFunc("/order/status", ReservationOrderStatusHTML(f.db, MailConfig{}))
	f.router.HandleFunc("/api/ordercancel", OrderCancel(f.db))
	status := func(lock string, seat int64) {
		f.post(t, "/order/status", url.Values{
			"event-id": {fmt.Sprint(f.eventID)}, "sits": {fmt.Sprint(seat)}, "rooms": {fmt.Sprint(f.roomID)},
			"prices": {"1"}, "total-price": {"1"}, "lock": {lock},
			"email": {"guest@example.com"}, "name": {"G"}, "surname": {"G"},
		})
	}
	cancel := func(lock string, seat int64) {
		r := httptest.NewRequest("POST", "/api/ordercancel", strings.NewReader(
			fmt.Sprintf(`{"sits":"%d","rooms":"%d","event-id":%d,"lock":%q}`, seat, f.roomID, f.eventID, lock)))
		f.router.ServeHTTP(httptest.NewRecorder(), r)
	}

	// cena z bazy (domyślna 100), nie z formularza
	lock := f.mark(t, f.roomID, 1)
	if st, price := f.seatStatus(t, 1); st != "marked" || price != 100 || lock == "" {
		t.Fatalf("seat 1: status %q price %d lock %q, want marked, 100, lock", st, price, lock)
	}
	// obcy nie zdejmie ani nie dokończy cudzej blokady
	cancel("", 1)
	cancel("1-00", 1)
	status("", 1)
	other := f.mark(t, f.roomID, 2)
	status(other, 1)
	if st, _ := f.seatStatus(t, 1); st != "marked" {
		t.Fatalf("seat 1 after stranger: %q, want marked", st)
	}
	// właściciel dokańcza
	status(lock, 1)
	if st, price := f.seatStatus(t, 1); st != "ordered" || price != 100 {
		t.Fatalf("seat 1 after order: %q %d, want ordered 100", st, price)
	}
	// i może zrezygnować ze swojej blokady
	cancel(other, 2)
	if st, _ := f.seatStatus(t, 2); st != "" {
		t.Fatalf("seat 2 after own cancel: %q, want deleted", st)
	}

	// miejsce wyłączone w designerze
	must(t)(f.db.PriceAdd(&Price{Price: 100, Currency: "Kč", Disabled: 1, EventID: f.eventID, FurnitureID: chair(t, f.db, f.roomID, 3)}))
	if f.mark(t, f.roomID, 3) != "" {
		t.Fatal("disabled seat 3 was locked")
	}

	// sala spoza imprezy (nawet o tej samej nazwie) i nieistniejące miejsce
	otherRoom := addRoom(t, f.db, "Sala", 5)
	if f.mark(t, otherRoom, 4) != "" || f.mark(t, f.roomID, 99) != "" {
		t.Fatal("seat outside event rooms was locked")
	}
	if n := len(eventChairs(f.db, Event{ID: f.eventID})); n != 3 {
		t.Fatalf("event chairs: %d, want 3 (room with the same name must not mix in)", n)
	}

	// szkic i zamknięta sprzedaż
	for _, q := range []string{`UPDATE events SET status='draft' WHERE id=$1`, `UPDATE events SET status='published', to_date=1900 WHERE id=$1`} {
		if _, err := f.db.DB.Exec(q, f.eventID); err != nil {
			t.Fatal(err)
		}
		if f.mark(t, f.roomID, 2) != "" {
			t.Fatalf("seat locked after %q", q)
		}
	}
}

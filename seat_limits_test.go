package main

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSeatCap(t *testing.T) {
	for total, want := range map[int]int{0: 0, 3: 3, 14: 14, 20: 14, 28: 14, 29: 14, 30: 15, 352: 176} {
		if got := seatCap(total); got != want {
			t.Errorf("seatCap(%d) = %d, want %d", total, got, want)
		}
	}
}

func TestClientIP(t *testing.T) {
	for _, c := range []struct{ remote, real, want string }{
		{"203.0.113.5:4000", "", "203.0.113.5"},
		{"203.0.113.5:4000", "198.51.100.7", "203.0.113.5"}, // nagłówek od gościa, nie od proxy
		{"172.17.1.1:4000", "198.51.100.7", "198.51.100.7"},  // nginx
		{"127.0.0.1:4000", " 198.51.100.7 ", "198.51.100.7"},
		{"172.17.1.1:4000", "", ""}, // proxy bez nagłówka: adresu nie znamy
		{"172.17.1.1:4000", "nonsense", ""},
		{"172.17.1.1:4000", "2001:db8:1:2:aaaa::1", "2001:db8:1:2::/64"},
		{"[2001:db8:1:2:bbbb::9]:4000", "", "2001:db8:1:2::/64"},
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = c.remote
		if c.real != "" {
			r.Header.Set("X-Real-IP", c.real)
		}
		if got := clientIP(r); got != c.want {
			t.Errorf("clientIP(%s, X-Real-IP %q) = %q, want %q", c.remote, c.real, got, c.want)
		}
	}
}

// Jeden gość nie zablokuje całej sali: limit na zamówienie i na adres IP.
func TestOrderSeatLimits(t *testing.T) {
	holds = &seatHolds{m: map[string][]markHold{}}
	f := newGuestFixture(t)
	big := addRoom(t, f.db, "Duża", 30)
	if err := f.db.EventAddRoom(f.eventID, big); err != nil {
		t.Fatal(err)
	}
	// 3 + 30 miejsc: limit 16
	order := func(ip string, seats ...int) string {
		t.Helper()
		var ss, rr []string
		for _, n := range seats {
			ss = append(ss, fmt.Sprint(n))
			rr = append(rr, fmt.Sprint(big))
		}
		form := url.Values{"event-id": {fmt.Sprint(f.eventID)}, "sits": {strings.Join(ss, ",")}, "rooms": {strings.Join(rr, ",")}}
		r := httptest.NewRequest("POST", "/order", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.RemoteAddr = "172.17.1.1:5000"
		r.Header.Set("X-Real-IP", ip)
		w := httptest.NewRecorder()
		f.router.ServeHTTP(w, r)
		return w.Body.String()
	}
	seats := func(from, to int) []int {
		var out []int
		for i := from; i <= to; i++ {
			out = append(out, i)
		}
		return out
	}
	marked := func() (n int) {
		f.db.DB.Get(&n, `SELECT count(*) FROM reservations WHERE status='marked'`)
		return
	}
	tooMany := T("pl", "order.too_many_title")
	held := T("pl", "order.held_title")

	// dwa adresy z tej samej sieci IPv6 /64 liczą się razem
	if body := order("2001:db8::1", seats(1, 10)...); strings.Contains(body, tooMany) {
		t.Fatalf("first IPv6 order refused")
	}
	if body := order("2001:db8::ffff", seats(11, 17)...); !strings.Contains(body, held) {
		t.Errorf("second address of the same /64 not counted together")
	}
	if _, err := f.db.DB.Exec(`DELETE FROM reservations`); err != nil {
		t.Fatal(err)
	}

	if body := order("198.51.100.1", seats(1, 17)...); !strings.Contains(body, tooMany) || !strings.Contains(body, "16 miejsc") {
		t.Fatalf("17 seats in one order accepted")
	}
	if marked() != 0 {
		t.Fatalf("refused order left %d seats marked", marked())
	}
	if body := order("198.51.100.1", seats(1, 16)...); strings.Contains(body, tooMany) || marked() != 16 {
		t.Fatalf("16 seats refused (marked %d)", marked())
	}
	// druga karta z tego samego adresu
	if body := order("198.51.100.1", 17); !strings.Contains(body, held) || !strings.Contains(body, "16 miejsc") {
		t.Errorf("same address got more than the limit")
	}
	// inny gość może zamawiać dalej
	if body := order("198.51.100.2", 18, 19); strings.Contains(body, held) {
		t.Errorf("other address refused")
	}
	// zamówienie dokończone (już nie "marked") zwalnia limit adresu
	if _, err := f.db.DB.Exec(`UPDATE reservations SET status='ordered' WHERE status='marked'`); err != nil {
		t.Fatal(err)
	}
	if body := order("198.51.100.1", 20, 21); strings.Contains(body, held) {
		t.Errorf("address still limited after its order was completed")
	}

	// strona sali zna limit, żeby nie dało się wybrać więcej
	page := f.do(t, httptest.NewRequest("GET", fmt.Sprintf("/res/org?event-id=%d", f.eventID), nil), "pl")
	if !strings.Contains(page, `data-maxseats="16"`) {
		t.Error("reservation page without data-maxseats")
	}
}

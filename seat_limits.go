package main

// seat_limits.go: ochrona przed zablokowaniem całej sali przez jednego gościa.
//
// Krok /order od razu blokuje wybrane miejsca (status "marked" na
// MARKEDMINUTES minut), więc bez ograniczeń jeden gość - albo skrypt
// powtarzający zamówienie - mógłby trzymać wszystkie miejsca imprezy.
//
//  1. Jedno zamówienie: najwyżej połowa wszystkich miejsc imprezy, ale nie
//     mniej niż SEATCAPMIN (na małej imprezie da się zamówić cały stół).
//     Prawdziwe zamówienia to w danych do 18 % sali.
//  2. Jeden adres IP: łącznie tyle samo miejsc zablokowanych naraz w
//     niedokończonych zamówieniach (druga karta, skrypt). Blokada trwa tylko
//     kilka minut, więc wiele osób za jednym IP (szkoła, firma) sobie nie
//     przeszkadza. Adres IPv6 liczymy po prefiksie /64 (jeden przyłącz).
//
// Admin zalogowany w panelu (sprzedaż przy wejściu) limitów nie ma, a
// rezerwacje z panelu w ogóle nie przechodzą przez /order.

import (
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

// SEATCAPMIN - limit miejsc na zamówienie nigdy nie jest mniejszy (o ile
// impreza w ogóle ma tyle miejsc).
const SEATCAPMIN = 14

// seatCap - ile miejsc może naraz zamówić/zablokować jeden gość, gdy
// impreza ma total miejsc.
func seatCap(total int) int {
	return min(total, max(total/2, SEATCAPMIN))
}

// clientIP - adres gościa. Za nginx (połączenie z sieci prywatnej albo
// lokalnej) bierzemy X-Real-IP, które ustawia proxy; bez tego nagłówka
// adresu nie znamy i zwracamy "" (limit na IP wtedy nie działa, zamiast
// wrzucać wszystkich gości do jednego worka z adresem proxy). Nagłówkowi z
// połączenia z publicznego adresu nie wierzymy - mógłby go wpisać sam gość.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return ""
	}
	if ip.IsLoopback() || ip.IsPrivate() {
		ip = net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP")))
		if ip == nil {
			return ""
		}
	}
	if ip.To4() == nil {
		return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
	}
	return ip.String()
}

// markHold - miejsca zablokowane w jednym kroku /order
type markHold struct {
	ids []int64
	at  time.Time
}

// seatHolds pamięta, z jakiego adresu założono blokady "marked". Tylko w
// pamięci: blokada i tak żyje kilka minut, a po restarcie serwera najwyżej
// przez chwilę limit liczy mniej.
type seatHolds struct {
	mu sync.Mutex
	m  map[string][]markHold
}

var holds = &seatHolds{m: map[string][]markHold{}}

// lock zakłada blokadę na sprawdzenie i zapis - inaczej dwa równoczesne
// zamówienia z jednego adresu przeszłyby oba.
func (h *seatHolds) lock()   { h.mu.Lock() }
func (h *seatHolds) unlock() { h.mu.Unlock() }

// held - ile miejsc adres ip trzyma teraz w niedokończonych zamówieniach
// imprezy (rezerwacje nadal "marked"). Wołać pod lock().
func (h *seatHolds) held(db *DB, ip string, eventID int64) int {
	cutoff := time.Now().Add(-(MARKEDMINUTES + 2) * time.Minute)
	var keep []markHold
	var ids []int64
	for _, x := range h.m[ip] {
		if x.at.Before(cutoff) {
			continue // ticker.go już je skasował
		}
		keep = append(keep, x)
		ids = append(ids, x.ids...)
	}
	if len(keep) == 0 {
		delete(h.m, ip)
		return 0
	}
	h.m[ip] = keep
	n, err := db.ReservationCountMarked(ids, eventID)
	if err != nil {
		log.Printf("seatHolds: count marked of %s: %v", ip, err)
	}
	return n
}

// add zapisuje nowe blokady adresu. Wołać pod lock().
func (h *seatHolds) add(ip string, ids []int64) {
	h.m[ip] = append(h.m[ip], markHold{ids: ids, at: time.Now()})
}

// ReservationCountMarked - ile z podanych rezerwacji imprezy jest nadal
// zablokowanych (status "marked").
func (db *DB) ReservationCountMarked(ids []int64, eventID int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	q, args, err := sqlx.In(`SELECT count(*) FROM reservations
		WHERE status='marked' AND events_id_fk=? AND id IN (?)`, eventID, ids)
	if err != nil {
		return 0, err
	}
	var n int
	err = db.DB.Get(&n, db.DB.Rebind(q), args...)
	return n, err
}

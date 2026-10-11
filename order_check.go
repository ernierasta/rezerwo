package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"
)

// Kroki zamówienia gościa (/order, /order/status, /api/ordercancel) są
// publiczne, więc niczego z formularza nie bierzemy na wiarę: cena miejsca
// pochodzi z bazy, miejsce musi być w sali imprezy i wolne, a blokadę
// miejsc może dokończyć albo zdjąć tylko ten, kto ją założył (orderLock).

// eventOnSale - impreza opublikowana i w okresie sprzedaży (jak EventGetCurrent)
func eventOnSale(e Event, now int64) bool {
	return e.Status.String != "draft" && e.FromDate < now && e.ToDate > now
}

// chairPrice - cena miejsca na imprezie: własna z designera albo domyślna imprezy
func chairPrice(c FurnitureFull, event Event) (int64, string) {
	price, currency := c.AdminPrice.Int64, c.AdminCurrency.String
	if price == 0 {
		price = event.DefaultPrice
	}
	if currency == "" {
		currency = event.DefaultCurrency
	}
	return price, currency
}

type seatKey struct{ room, number int64 }

// eventChairs - krzesła wszystkich sal imprezy z cenami i stanem rezerwacji
func eventChairs(db *DB, event Event) map[seatKey]FurnitureFull {
	out := map[seatKey]FurnitureFull{}
	rooms, err := db.EventGetRooms(event.ID)
	if err != nil {
		log.Printf("eventChairs: rooms of event %d: %v", event.ID, err)
	}
	for _, room := range rooms {
		chairs, err := db.FurnitureFullGetChairs(event.ID, room.ID)
		if err != nil {
			log.Printf("eventChairs: chairs of room %d: %v", room.ID, err)
		}
		for _, c := range chairs {
			out[seatKey{room.ID, c.Number}] = c
		}
	}
	return out
}

// orderLock podpisuje blokadę miejsc (numery rezerwacji "marked" z kroku
// /order). Klucz żyje do restartu serwera, tak jak blokada (ticker.go).
func orderLock(eventID int64, resIDs []int64) string {
	ids := make([]string, len(resIDs))
	for i, id := range resIDs {
		ids[i] = strconv.FormatInt(id, 10)
	}
	s := strings.Join(ids, ".")
	return s + "-" + orderLockSig(eventID, s)
}

func orderLockSig(eventID int64, ids string) string {
	m := hmac.New(sha256.New, doneKey)
	fmt.Fprintf(m, "order:%d:%s", eventID, ids)
	return hex.EncodeToString(m.Sum(nil))[:32]
}

// parseOrderLock zwraca numery rezerwacji z podpisanej blokady; ok=false dla
// podrobionej albo z innej imprezy.
func parseOrderLock(eventID int64, lock string) (map[int64]bool, bool) {
	i := strings.LastIndexByte(lock, '-')
	if i <= 0 {
		return nil, false
	}
	ids, sig := lock[:i], lock[i+1:]
	if !hmac.Equal([]byte(sig), []byte(orderLockSig(eventID, ids))) {
		return nil, false
	}
	out := map[int64]bool{}
	for _, s := range strings.Split(ids, ".") {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, false
		}
		out[id] = true
	}
	return out, true
}

package main

import (
	"log"
	"time"
)

// StartMarkedCleanup co minutę zdejmuje blokady miejsc z kroku /order
// (status "marked"), których gość nie potwierdził w ciągu MARKEDMINUTES.
func StartMarkedCleanup(db *DB) {
	go func() {
		for range time.Tick(time.Minute) {
			RemoveStalledOrders(db)
		}
	}()
}

func RemoveStalledOrders(db *DB) {
	rr, err := db.ReservationGetAllInStatus("marked")
	if err != nil {
		log.Printf("RemoveStalledOrders: %v", err)
		return
	}
	limit := time.Now().Add(-MARKEDMINUTES * time.Minute).Unix()
	for _, r := range rr {
		if r.CustomerID == -1 && r.OrderedDate.Int64 < limit {
			if err := db.ReservationDel(r.ID); err != nil {
				log.Printf("RemoveStalledOrders: reservation %d: %v", r.ID, err)
			}
		}
	}
}

package main

import (
	"log"
	"time"
)

func Ticker5min(db *DB, stop chan bool) {
	ticker := time.NewTicker(1 * time.Minute)

	go func() {
		for {
			select {
			case <-stop:
				return
			case t := <-ticker.C:
				_ = t
				RemoveStalledOrders(db)
			}
		}
	}()
}

func TickerStop(stop chan bool) {
	stop <- true
}

func RemoveStalledOrders(db *DB) {
	rr, err := db.ReservationGetInStatus("marked")
	if err != nil {
		log.Println(err)
	}

	now := time.Now().Unix()
	fiveM := int64((5 * time.Minute).Seconds())
	for i := range rr {
		if rr[i].CustomerID == -1 && now > rr[i].OrderedDate.Int64+fiveM {
			db.ReservationDel(rr[i].ID)
		}
	}
}

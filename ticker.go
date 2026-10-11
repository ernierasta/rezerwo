package main

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
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

// GENERATEDMAXAGE - po tym czasie kody QR i bilety PDF znikają z media/
const GENERATEDMAXAGE = 7 * 24 * time.Hour

// StartGeneratedCleanup raz na dobę (i zaraz po starcie) usuwa stare kody QR
// płatności (media/<org>/qrcode) i bilety PDF (media/<org>/pdf/...). To tylko
// pliki pośrednie: strona albo mail tworzy je na nowo przy każdym użyciu
// (qrPayImage, GenerateTicketsPDF), a ich nazwy zawierają nazwiska gości.
func StartGeneratedCleanup(mediaRoot string) {
	go func() {
		for {
			RemoveGeneratedFiles(mediaRoot, time.Now().Add(-GENERATEDMAXAGE))
			time.Sleep(24 * time.Hour)
		}
	}()
}

// RemoveGeneratedFiles kasuje pliki starsze niż before z katalogów qrcode/
// i pdf/ organizacji; zwraca liczbę usuniętych.
func RemoveGeneratedFiles(mediaRoot string, before time.Time) int {
	orgs, err := os.ReadDir(mediaRoot)
	if err != nil {
		log.Printf("RemoveGeneratedFiles: %v", err)
		return 0
	}
	n := 0
	for _, org := range orgs {
		if !org.IsDir() {
			continue
		}
		for _, sub := range []string{MEDIAQRCODESUBDIR, "pdf"} {
			dir := filepath.Join(mediaRoot, org.Name(), sub)
			filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				if info, err := d.Info(); err == nil && info.ModTime().Before(before) {
					if err := os.Remove(p); err != nil {
						log.Printf("RemoveGeneratedFiles: %v", err)
					} else {
						n++
					}
				}
				return nil
			})
		}
	}
	if n > 0 {
		log.Printf("RemoveGeneratedFiles: removed %d old QR codes/tickets", n)
	}
	return n
}

package main

import (
	"log"
	"sort"
)

// roomPlan - meble sali; krzesła z ceną i stanem rezerwacji dla imprezy
type roomPlan struct {
	Tables  []Furniture
	Chairs  []FurnitureFull
	Objects []Furniture
	Labels  []Furniture
}

func roomFurnitures(db *DB, roomName string, eventID int64) roomPlan {
	var p roomPlan
	var err error
	if p.Chairs, err = db.FurnitureFullGetChairs(eventID, roomName); err != nil {
		log.Printf("roomFurnitures: chairs of room %q, event %d: %v", roomName, eventID, err)
	}
	for typ, dst := range map[string]*[]Furniture{"table": &p.Tables, "object": &p.Objects, "label": &p.Labels} {
		if *dst, err = db.FurnitureGetAllByRoomNameOfType(roomName, typ); err != nil {
			log.Printf("roomFurnitures: %ss of room %q: %v", typ, roomName, err)
		}
	}
	return p
}

func (f Furniture) nr() int64 { return f.Number }

// maxNr - najwyższy numer mebla na liście (nowy mebel dostaje kolejny)
func maxNr[F interface{ nr() int64 }](ff []F) int64 {
	m := int64(0)
	for _, f := range ff {
		m = max(m, f.nr())
	}
	return m
}

// FurnitureRenumber numeruje meble od 1 bez dziur, zachowując kolejność.
// Lista musi zawierać meble jednego typu z jednej sali.
func FurnitureRenumber(ff []Furniture) []Furniture {
	sort.Slice(ff, func(i, j int) bool {
		return ff[i].Number < ff[j].Number
	})
	for i := range ff {
		ff[i].Number = int64(i + 1)
	}
	return ff
}

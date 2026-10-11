package main

// designer_ui.go: designer sali w nowym wyglądzie (rozdz. 8 HARMONIJKA.md).
//
// Adresy: /admin/events/{id}/rooms/{roomid} (sala imprezy - stan krzeseł
// i ceny tej imprezy) i /admin/rooms/{roomid} z katalogów (wtedy stan
// krzeseł z najbliższej imprezy organizacji). Zapis mebli dalej przez
// /api/furnit, /api/furdel, /api/room i /api/renumber (js/n_designer.js).

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

type nDesignerVars struct {
	NPage
	Room    Room
	EventID int64
	roomPlan

	// kolejne wolne numery dla nowych mebli
	TableNr, ChairNr, ObjectNr, LabelNr int64

	Seats     int
	Back      string
	BackLabel string
}

// RoomDesignerPage - /admin/rooms/{roomid} i /admin/events/{id}/rooms/{roomid}
func RoomDesignerPage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		lang := PanelLang(r)
		vars := mux.Vars(r)

		roomID, err := strconv.ParseInt(vars["roomid"], 10, 64)
		if err != nil || !userOwnsRoom(db, user.ID, roomID) {
			log.Printf("RoomDesignerPage: user %q, room %q: not found or not owned", user.Email, vars["roomid"])
			http.NotFound(w, r)
			return
		}

		back, backLabel := "/admin/catalogs", T(lang, "catalog.title")
		var eventID int64
		if ids, isEvent := vars["id"]; isEvent {
			eventID, err = strconv.ParseInt(ids, 10, 64)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			event, err := db.EventGetByID(eventID)
			if err != nil || event.UserID != user.ID {
				log.Printf("RoomDesignerPage: event %d not for user %d: %v", eventID, user.ID, err)
				http.NotFound(w, r)
				return
			}
			back, backLabel = "/admin/events/"+ids+"#sala", event.Name
		} else {
			eventID = nearestEventID(db, user.ID)
		}

		room, err := db.RoomGetByID(roomID)
		if err != nil {
			log.Printf("RoomDesignerPage: room %d: %v", roomID, err)
		}
		p := roomFurnitures(db, roomID, eventID)
		v := nDesignerVars{
			NPage:     NPage{Title: room.Name, Crumb: room.Name, OrgName: orgName(user)},
			Room:      room,
			EventID:   eventID,
			roomPlan:  p,
			TableNr:   maxNr(p.Tables) + 1,
			ChairNr:   maxNr(p.Chairs) + 1,
			ObjectNr:  maxNr(p.Objects) + 1,
			LabelNr:   maxNr(p.Labels) + 1,
			Seats:     len(p.Chairs),
			Back:      back,
			BackLabel: backLabel,
		}
		nRender(w, lang, v, "tmpl/n_designer.html")
	}
}

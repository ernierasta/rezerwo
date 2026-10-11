package main

// designer_api.go: zapis zmian z designera sali (js/n_designer.js) -
// /api/room, /api/furnit, /api/furdel, /api/renumber.

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/sessions"
)

// userOwnsRoom checks if room belongs to organization
func userOwnsRoom(db *DB, userID, roomID int64) bool {
	rooms, err := db.RoomGetAllByUserID(userID)
	if err != nil {
		log.Printf("userOwnsRoom: can not get rooms for user %d, err: %v", userID, err)
		return false
	}
	for _, room := range rooms {
		if room.ID == roomID {
			return true
		}
	}
	return false
}

// designerMsg reads JSON body of POST request into m and returns true if
// request comes from admin owning the room, otherwise it writes error response
func designerMsg[M interface{ room() int64 }](w http.ResponseWriter, r *http.Request, db *DB, cs *sessions.CookieStore, m *M) bool {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	if err := json.NewDecoder(r.Body).Decode(m); err != nil {
		log.Printf("designer API %s: bad JSON: %v", r.URL.Path, err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return false
	}
	user, ok := adminUser(w, r, db, cs)
	if !ok {
		return false
	}
	if !userOwnsRoom(db, user.ID, (*m).room()) {
		log.Printf("designer API %s: user %q does not own room %d", r.URL.Path, user.Email, (*m).room())
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

type RoomMsg struct {
	RoomID int64 `json:"room_id"`
	Width  int64 `json:"width"`
	Height int64 `json:"height"`
}

func (m RoomMsg) room() int64 { return m.RoomID }

func DesignerSetRoomSize(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var m RoomMsg
		if !designerMsg(w, r, db, cs, &m) {
			return
		}
		if err := db.RoomModSizeByID(&Room{ID: m.RoomID, Height: m.Height, Width: m.Width}); err != nil {
			log.Printf("DesignerSetRoomSize: %v", err)
		}
	}
}

type MoveMsg struct {
	EventID     int64  `json:"event_id"`
	RoomID      int64  `json:"room_id"`
	Number      int64  `json:"name"`
	Type        string `json:"type"`
	Orientation string `json:"orientation"`
	Disabled    bool   `json:"disabled"`
	X           int64  `json:"x"`
	Y           int64  `json:"y"`
	Width       int64  `json:"width"`
	Height      int64  `json:"height"`
	Color       string `json:"color"`
	Price       int64  `json:"price"`
	Currency    string `json:"currency"`
	Capacity    int64  `json:"capacity"`
	Label       string `json:"label"`
}

func (m MoveMsg) room() int64 { return m.RoomID }

// DesignerMoveObject zapisuje mebel (nowy albo zmieniony) i jego cenę w imprezie
func DesignerMoveObject(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var m MoveMsg
		if !designerMsg(w, r, db, cs, &m) {
			return
		}
		f := Furniture{
			Number:      m.Number,
			X:           m.X,
			Y:           m.Y,
			Type:        m.Type,
			Orientation: ToNS(m.Orientation),
			Width:       ToNI(m.Width),
			Height:      ToNI(m.Height),
			Color:       ToNS(m.Color),
			Label:       ToNS(m.Label),
			Capacity:    ToNI(m.Capacity),
			RoomID:      m.RoomID,
		}
		// insert nie przejdzie dla istniejącego mebla - wtedy update
		if _, err := db.FurnitureAdd(&f); err != nil {
			if err := db.FurnitureModByNumberTypeRoom(&f); err != nil {
				log.Printf("DesignerMoveObject: furniture insert and update failed, f: %+v, err: %v", f, err)
			}
		}

		furn, err := db.FurnitureGetByTypeNumberRoom(m.Type, m.Number, m.RoomID)
		if err != nil {
			log.Printf("DesignerMoveObject: furniture get failed %+v, err: %v", f, err)
		}

		p := Price{
			Price:       m.Price,
			Currency:    m.Currency,
			EventID:     m.EventID,
			FurnitureID: furn.ID,
		}
		if m.Disabled {
			p.Disabled = 1
		}
		if _, err := db.PriceAdd(&p); err != nil {
			if err := db.PriceModByEventIDFurnID(&p); err != nil {
				log.Printf("DesignerMoveObject: price insert and update failed, p: %+v, err: %v", p, err)
			}
		}
	}
}

type DeleteMsg struct {
	EventID int64  `json:"event_id"`
	RoomID  int64  `json:"room_id"`
	Number  int64  `json:"name"`
	Type    string `json:"type"`
}

func (m DeleteMsg) room() int64 { return m.RoomID }

func DesignerDeleteObject(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var m DeleteMsg
		if !designerMsg(w, r, db, cs, &m) {
			return
		}
		// Krzesła z rezerwacjami są chronione (rozdz. 5.3 HARMONIJKA.md):
		// nie da się skasować ani takiego krzesła, ani stołu, przy którym stoi.
		if n, what, err := reservedUnder(db, m); err != nil {
			log.Printf("DesignerDeleteObject: reservation check: %v", err)
		} else if n > 0 {
			key := "designer.chair_reserved"
			if what == "table" {
				key = "designer.table_reserved"
			}
			jsonMsg(w, http.StatusConflict, map[string]interface{}{
				"msg":          TN(PanelLang(r), key, n),
				"reservations": n,
			})
			return
		}

		if err := db.FurnitureDelByNumberTypeRoom(m.Number, m.Type, m.RoomID); err != nil {
			log.Printf("DesignerDeleteObject: %+v: %v", m, err)
			jsonMsg(w, http.StatusTeapot, map[string]interface{}{"msg": err.Error()})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "deleted"})
	}
}

type RenumberMsg struct {
	RoomID int64  `json:"room_id"`
	Type   string `json:"type"`
}

func (m RenumberMsg) room() int64 { return m.RoomID }

func DesignerRenumberType(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var m RenumberMsg
		if !designerMsg(w, r, db, cs, &m) {
			return
		}
		ff, err := db.FurnitureGetAllByRoomIDOfType(m.RoomID, m.Type)
		if err != nil {
			log.Printf("DesignerRenumberType: %q of room %d: %v", m.Type, m.RoomID, err)
		}
		for _, f := range FurnitureRenumber(ff) {
			if err := db.FurnitureMod(&f); err != nil {
				log.Printf("DesignerRenumberType: %v", err)
			}
		}
	}
}

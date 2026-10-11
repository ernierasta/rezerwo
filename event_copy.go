package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/sessions"
)

type EventCopyRoom struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Mine bool   `json:"mine"`
}

type EventCopyNotif struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type EventCopyInfo struct {
	EventID  int64           `json:"event_id"`
	Name     string          `json:"name"`
	Date     string          `json:"date"`
	FromDate string          `json:"from_date"`
	RoomIDs  []int64         `json:"room_ids"`
	AllRooms []EventCopyRoom `json:"all_rooms"`
	ThankYou *EventCopyNotif `json:"thankyou"`
	Admin    *EventCopyNotif `json:"admin"`
}

// eventCopyGetSource returns event which user is allowed to copy (own or sharable)
func eventCopyGetSource(db *DB, eventID int64, user User) (Event, error) {
	ev, err := db.EventGetByID(eventID)
	if err != nil {
		return ev, fmt.Errorf("nie znaleziono imprezy %d: %v", eventID, err)
	}
	if ev.UserID != user.ID && !(ev.Sharable.Valid && ev.Sharable.Bool) {
		return ev, fmt.Errorf("impreza %d nie należy do użytkownika", eventID)
	}
	return ev, nil
}

// EventCopyInfoAPI returns data needed by event copy dialog
func EventCopyInfoAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		eventID, err := strconv.ParseInt(r.FormValue("event_id"), 10, 64)
		if err != nil || eventID == 0 {
			jsonMsg(w, http.StatusTeapot, map[string]interface{}{"msg": "Wybierz imprezę do skopiowania."})
			return
		}
		ev, err := eventCopyGetSource(db, eventID, user)
		if err != nil {
			log.Printf("EventCopyInfoAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]interface{}{"msg": err.Error()})
			return
		}

		info := EventCopyInfo{EventID: ev.ID, Name: ev.Name, Date: ToDate(ev.Date), FromDate: ToDate(ev.FromDate), RoomIDs: []int64{}, AllRooms: []EventCopyRoom{}}

		evRooms, err := db.EventGetRooms(ev.ID)
		if err != nil {
			log.Printf("EventCopyInfoAPI: error getting rooms for event %d, %v", ev.ID, err)
		}
		for _, room := range evRooms {
			info.RoomIDs = append(info.RoomIDs, room.ID)
		}

		userRooms, err := db.RoomGetAllByUserID(user.ID)
		if err != nil {
			log.Printf("EventCopyInfoAPI: error getting rooms for user %d, %v", user.ID, err)
		}
		mine := map[int64]bool{}
		for _, room := range userRooms {
			mine[room.ID] = true
		}
		allRooms, err := db.RoomGetAll()
		if err != nil {
			log.Printf("EventCopyInfoAPI: error getting all rooms, %v", err)
		}
		for _, room := range allRooms {
			info.AllRooms = append(info.AllRooms, EventCopyRoom{ID: room.ID, Name: room.Name, Mine: mine[room.ID]})
		}

		if ev.ThankYouNotificationsID.Valid && ev.ThankYouNotificationsID.Int64 != 0 {
			if n, err := db.NotificationGetByIDUnsafe(ev.ThankYouNotificationsID.Int64); err == nil {
				info.ThankYou = &EventCopyNotif{ID: n.ID, Name: n.Name}
			}
		}
		if ev.AdminNotificationsID.Valid && ev.AdminNotificationsID.Int64 != 0 {
			if n, err := db.NotificationGetByIDUnsafe(ev.AdminNotificationsID.Int64); err == nil {
				info.Admin = &EventCopyNotif{ID: n.ID, Name: n.Name}
			}
		}

		jsonMsg(w, http.StatusOK, info)
	}
}

type EventCopyMsg struct {
	EventID  int64  `json:"event_id"`
	Name     string `json:"name"`
	Date     string `json:"date"`
	FromDate string `json:"from_date"`
	Rooms    []struct {
		RoomID int64  `json:"room_id"`
		Copy   bool   `json:"copy"`
		Name   string `json:"name"` // name of room copy, empty = original name
	} `json:"rooms"`
	CopyNotifications bool   `json:"copy_notifications"`
	ThankYouName      string `json:"thankyou_name"`
	AdminName         string `json:"admin_name"`
}

// EventCopyAPI creates copy of event, with rooms (linked or copied), prices,
// addons and optionally copies of notifications. Reservation ends day after event date.
func EventCopyAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		fail := func(format string, a ...interface{}) {
			msg := fmt.Sprintf(format, a...)
			log.Printf("EventCopyAPI: %s", msg)
			jsonMsg(w, http.StatusTeapot, map[string]interface{}{"msg": msg})
		}

		var m EventCopyMsg
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			fail("Błędne dane: %v", err)
			return
		}
		m.Name = strings.TrimSpace(m.Name)
		m.ThankYouName = strings.TrimSpace(m.ThankYouName)
		m.AdminName = strings.TrimSpace(m.AdminName)

		src, err := eventCopyGetSource(db, m.EventID, user)
		if err != nil {
			fail("%v", err)
			return
		}
		if m.Name == "" {
			fail("Podaj nazwę nowej imprezy.")
			return
		}
		date, err := ToUnix(m.Date)
		if err != nil {
			fail("Podaj poprawną datę imprezy.")
			return
		}
		fromDate, err := ToUnix(m.FromDate)
		if err != nil {
			fail("Podaj poprawną datę początku rezerwacji.")
			return
		}
		if fromDate > date {
			fail("Początek rezerwacji musi być przed datą imprezy.")
			return
		}
		exists, err := db.EventNameExists(m.Name, user.ID)
		if err != nil {
			fail("Błąd bazy danych: %v", err)
			return
		}
		if exists {
			fail("Impreza o nazwie %q już istnieje, wybierz inną nazwę.", m.Name)
			return
		}
		srcRooms, err := db.EventGetRooms(src.ID)
		if err != nil {
			fail("Nie udało się pobrać pomieszczeń imprezy: %v", err)
			return
		}
		srcRoomIDs := map[int64]bool{}
		for _, room := range srcRooms {
			srcRoomIDs[room.ID] = true
		}
		seen := map[int64]bool{}
		for _, rm := range m.Rooms {
			if _, err := db.RoomGetByID(rm.RoomID); err != nil {
				fail("Nie znaleziono pomieszczenia %d.", rm.RoomID)
				return
			}
			if !rm.Copy {
				if seen[rm.RoomID] {
					fail("Pomieszczenie %d jest przypisane dwa razy.", rm.RoomID)
					return
				}
				seen[rm.RoomID] = true
			}
		}

		// notifications
		copyNotif := func(id sql.NullInt64, name string) (sql.NullInt64, error) {
			n, err := db.NotificationGetByIDUnsafe(id.Int64)
			if err != nil {
				return id, fmt.Errorf("nie znaleziono notyfikacji %d: %v", id.Int64, err)
			}
			if n.UserID != user.ID && !n.Sharable {
				return id, fmt.Errorf("notyfikacja %d nie należy do użytkownika", n.ID)
			}
			if name == "" {
				return id, fmt.Errorf("podaj nazwę kopii notyfikacji %q", n.Name)
			}
			n.ID = 0
			n.Name = name
			n.UserID = user.ID
			n.Sharable = false
			n.CreatedDate = time.Now().Unix()
			n.UpdatedDate = sql.NullInt64{}
			newID, err := db.NotificationAdd(&n)
			if err != nil {
				return id, fmt.Errorf("nie udało się skopiować notyfikacji %q: %v", name, err)
			}
			return ToNI(newID), nil
		}

		ev := src
		ev.ID = 0
		ev.Name = m.Name
		ev.Date = date
		ev.FromDate = fromDate
		ev.ToDate = time.Unix(date, 0).UTC().AddDate(0, 0, 1).Unix()
		if src.UserID != user.ID {
			ev.UserID = user.ID
			ev.Sharable = ToNB(false)
		}

		hasThankYou := src.ThankYouNotificationsID.Valid && src.ThankYouNotificationsID.Int64 != 0
		hasAdmin := src.AdminNotificationsID.Valid && src.AdminNotificationsID.Int64 != 0
		if m.CopyNotifications {
			if hasThankYou && m.ThankYouName == "" || hasAdmin && m.AdminName == "" {
				fail("Podaj nazwy kopiowanych notyfikacji.")
				return
			}
			if hasThankYou {
				ev.ThankYouNotificationsID, err = copyNotif(src.ThankYouNotificationsID, m.ThankYouName)
				if err != nil {
					fail("%v", err)
					return
				}
			}
			if hasAdmin {
				if hasThankYou && src.AdminNotificationsID.Int64 == src.ThankYouNotificationsID.Int64 {
					ev.AdminNotificationsID = ev.ThankYouNotificationsID // same notification used for both
				} else {
					ev.AdminNotificationsID, err = copyNotif(src.AdminNotificationsID, m.AdminName)
					if err != nil {
						fail("%v", err)
						return
					}
				}
			}
		}

		newEventID, err := db.EventAdd(&ev)
		if err == nil {
			// kopia wie, z czego powstała - stąd porównanie sprzedaży edycji
			if perr := db.EventSetPreviousEdition(newEventID, user.ID, ToNI(src.ID)); perr != nil {
				log.Printf("EventCopyAPI: previous edition: %v", perr)
			}
			if terr := db.EventSetTheme(newEventID, user.ID, ThemeKey(src.Theme.String)); terr != nil {
				log.Printf("EventCopyAPI: theme: %v", terr)
			}
		}
		if err != nil {
			fail("Nie udało się zapisać imprezy: %v", err)
			return
		}

		// rooms + prices
		var problems []string
		for _, rm := range m.Rooms {
			roomID := rm.RoomID
			if rm.Copy {
				roomID, err = db.RoomWithFurnituresCopy(rm.RoomID, strings.TrimSpace(rm.Name))
				if err != nil {
					problems = append(problems, fmt.Sprintf("kopia pomieszczenia %d: %v", rm.RoomID, err))
					continue
				}
				if err := db.RoomAssignToUser(user.ID, roomID); err != nil {
					problems = append(problems, fmt.Sprintf("przypisanie pomieszczenia %d: %v", roomID, err))
				}
			}
			if err := db.EventAddRoom(newEventID, roomID); err != nil {
				problems = append(problems, fmt.Sprintf("przypisanie pomieszczenia %d do imprezy: %v", roomID, err))
				continue
			}
			if srcRoomIDs[rm.RoomID] {
				if _, err := db.PriceCopyForEventRoom(src.ID, rm.RoomID, newEventID, roomID); err != nil {
					problems = append(problems, fmt.Sprintf("ceny pomieszczenia %d: %v", roomID, err))
				}
			}
		}
		if _, err := db.EventAddonCopy(src.ID, newEventID, user.ID); err != nil {
			problems = append(problems, fmt.Sprintf("dodatki imprezy: %v", err))
		}

		if len(problems) > 0 {
			fail("Impreza %q została utworzona, ale wystąpiły błędy:\n%s", m.Name, strings.Join(problems, "\n"))
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": fmt.Sprintf("copied event %d to %d", src.ID, newEventID), "event_id": newEventID})
	}
}

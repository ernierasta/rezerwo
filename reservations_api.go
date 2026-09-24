package main

// reservations_api.go: nowe API tabeli rezerwacji (etap 2 redesignu).
//
// Wszystkie endpointy sprawdzają sesję admina i to, że impreza należy do jego
// organizacji - stare /api/resstatus i /api/resdelete tego nie robiły.
// Statusy ustawiane są jawnie (nie "toggle"), data zapłaty liczona na serwerze,
// a cofnięcie statusu czyści ją (NULL).

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// nResJSON to jeden wiersz tabeli (jedno krzesło).
type nResJSON struct {
	ID          int64  `json:"id"`
	Chair       int64  `json:"chair"`
	Room        string `json:"room"`
	RoomID      int64  `json:"room_id"`
	Name        string `json:"name"`
	Surname     string `json:"surname"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Status      string `json:"status"`
	Price       int64  `json:"price"`
	Currency    string `json:"currency"`
	OrderedDate int64  `json:"ordered_date"`
	PayedDate   int64  `json:"payed_date"`
	LastMail    int64  `json:"last_mail"`
	MailCount   int64  `json:"mail_count"`
	Notes       string `json:"notes"`
	OrderID     int64  `json:"order_id"`
}

// nAddonItemJSON to dodatek albo dopłata jednego zamówienia.
type nAddonItemJSON struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Amount      int64  `json:"amount"`
	Price       int64  `json:"price"`
	Currency    string `json:"currency"`
	Status      string `json:"status"`
}

type nResListResp struct {
	EventID   int64      `json:"event_id"`
	EventName string     `json:"event_name"`
	Currency  string     `json:"currency"`
	Rows      []nResJSON `json:"rows"`
	// Addons - pozycje zamówień (dodatki i dopłaty) po id zamówienia
	Addons        map[int64][]nAddonItemJSON `json:"addons"`
	Notifications []nResNotifOpt             `json:"notifications"`
}

// eventAddonsByOrder zwraca pozycje zamówień imprezy pogrupowane po zamówieniu.
func eventAddonsByOrder(db *DB, eventID int64) map[int64][]nAddonItemJSON {
	items, err := db.OrderItemsGetByEvent(eventID)
	if err != nil {
		log.Printf("eventAddonsByOrder: %v", err)
		return nil
	}
	out := map[int64][]nAddonItemJSON{}
	for _, it := range items {
		out[it.OrderID] = append(out[it.OrderID], nAddonItemJSON{
			ID:          it.ID,
			Kind:        it.Kind,
			Description: it.Description,
			Amount:      it.Amount,
			Price:       it.Price,
			Currency:    it.Currency.String,
			Status:      it.Status,
		})
	}
	return out
}

type nResNotifOpt struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// adminEvent sprawdza sesję i to, że impreza {id} należy do organizacji admina.
func adminEvent(w http.ResponseWriter, r *http.Request, db *DB, cs *sessions.CookieStore) (User, Event, bool) {
	user, ok := adminUser(w, r, db, cs)
	if !ok {
		return User{}, Event{}, false
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "bad event id"})
		return User{}, Event{}, false
	}
	event, err := db.EventGetByID(id)
	if err != nil || event.UserID != user.ID {
		jsonMsg(w, http.StatusNotFound, map[string]string{"msg": "event not found"})
		return User{}, Event{}, false
	}
	return user, event, true
}

func reservationRows(db *DB, user User, event Event) []nResJSON {
	rr, err := db.ReservationFullGetAll(user.ID, event.ID)
	if err != nil {
		log.Printf("reservationRows: %v", err)
	}
	rows := make([]nResJSON, 0, len(rr))
	for _, r := range rr {
		rows = append(rows, nResJSON{
			ID:          r.ID,
			Chair:       r.ChairNumber,
			Room:        r.RoomName,
			RoomID:      r.RoomID,
			Name:        r.CustName.String,
			Surname:     r.CustSurname.String,
			Email:       r.CustEmail.String,
			Phone:       r.CustPhone.String,
			Status:      r.Status,
			Price:       r.Price.Int64,
			Currency:    r.Currency.String,
			OrderedDate: r.OrderedDate.Int64,
			PayedDate:   r.PayedDate.Int64,
			LastMail:    r.LastNotification.Int64,
			MailCount:   r.NotifAmmount.Int64,
			Notes:       r.Notes.String,
			OrderID:     r.OrderID.Int64,
		})
	}
	return rows
}

// ReservationsListAPI: GET /api/events/{id}/reservations
func ReservationsListAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, event, ok := adminEvent(w, r, db, cs)
		if !ok {
			return
		}
		notifs, err := db.NotificationGetAllRelatedToEventsForUser(user.ID)
		if err != nil {
			log.Printf("ReservationsListAPI: notifications: %v", err)
		}
		resp := nResListResp{
			EventID:   event.ID,
			EventName: event.Name,
			Currency:  event.DefaultCurrency,
			Rows:      reservationRows(db, user, event),
			Addons:    eventAddonsByOrder(db, event.ID),
		}
		for _, n := range notifs {
			resp.Notifications = append(resp.Notifications, nResNotifOpt{ID: n.ID, Name: n.Name})
		}
		jsonMsg(w, http.StatusOK, resp)
	}
}

type nResIDsMsg struct {
	IDs            []int64 `json:"ids"`
	Status         string  `json:"status"`
	NotificationID int64   `json:"notification_id"`
	Notes          string  `json:"notes"`
}

func decodeIDs(w http.ResponseWriter, r *http.Request) (nResIDsMsg, bool) {
	var m nResIDsMsg
	body, err := io.ReadAll(r.Body)
	if err != nil {
		jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
		return m, false
	}
	if err := json.Unmarshal(body, &m); err != nil {
		jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
		return m, false
	}
	if len(m.IDs) == 0 {
		jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "no ids"})
		return m, false
	}
	return m, true
}

// ReservationsStatusAPI: POST /api/events/{id}/reservations/status
// {"ids": [...], "status": "payed"|"ordered"|"disabled"}
func ReservationsStatusAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, event, ok := adminEvent(w, r, db, cs)
		if !ok {
			return
		}
		m, ok := decodeIDs(w, r)
		if !ok {
			return
		}
		switch m.Status {
		case "payed", "ordered", "marked", "disabled":
		default:
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "unknown status " + m.Status})
			return
		}

		payed := sql.NullInt64{}
		if m.Status == "payed" {
			payed = sql.NullInt64{Int64: time.Now().Unix(), Valid: true}
		}

		var failed []int64
		for _, id := range m.IDs {
			if err := db.ReservationSetStatus(id, event.ID, m.Status, payed); err != nil {
				log.Printf("ReservationsStatusAPI: %d: %v", id, err)
				failed = append(failed, id)
				continue
			}
			// historia zmian: kto i kiedy zmienił status
			if _, err := db.ReservationStatusLogAdd(&ReservationStatusLog{
				Date:          time.Now().Unix(),
				Status:        m.Status,
				AdminEmail:    ToNS(user.Email),
				ReservationID: id,
			}); err != nil {
				log.Printf("ReservationsStatusAPI: log %d: %v", id, err)
			}
		}
		if len(failed) > 0 {
			jsonMsg(w, http.StatusTeapot, map[string]interface{}{"msg": "some reservations not updated", "failed": failed})
			return
		}
		// zwracamy zaktualizowane wiersze - UI zmienia się dopiero po odpowiedzi serwera
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "ok", "rows": reservationRows(db, user, event)})
	}
}

// ReservationsDeleteAPI: DELETE /api/events/{id}/reservations {"ids": [...]}
func ReservationsDeleteAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, event, ok := adminEvent(w, r, db, cs)
		if !ok {
			return
		}
		m, ok := decodeIDs(w, r)
		if !ok {
			return
		}

		var failed []int64
		for _, id := range m.IDs {
			evID, err := db.ReservationEventID(id)
			if err != nil || evID != event.ID {
				log.Printf("ReservationsDeleteAPI: reservation %d not in event %d: %v", id, event.ID, err)
				failed = append(failed, id)
				continue
			}
			res, err := db.ReservationGetByIDForNote(id)
			if err != nil {
				log.Printf("ReservationsDeleteAPI: get %d: %v", id, err)
			}
			if err := db.ReservationDel(id); err != nil {
				log.Printf("ReservationsDeleteAPI: delete %d: %v", id, err)
				failed = append(failed, id)
				continue
			}
			// uwagi zamówienia kasujemy dopiero, gdy nie ma już rezerwacji, które ich używają
			if res.NoteID.Valid {
				rest, err := db.ReservationGetAllByNoteID(res.NoteID.Int64)
				if err != nil {
					log.Printf("ReservationsDeleteAPI: notes check: %v", err)
				} else if len(rest) == 0 {
					if err := db.NoteDel(res.NoteID.Int64); err != nil {
						log.Printf("ReservationsDeleteAPI: note delete: %v", err)
					}
				}
			}
		}
		if len(failed) > 0 {
			jsonMsg(w, http.StatusTeapot, map[string]interface{}{"msg": "some reservations not deleted", "failed": failed})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "ok", "rows": reservationRows(db, user, event)})
	}
}

// ReservationNotesAPI: PATCH /api/events/{id}/reservations/notes {"ids":[id], "notes":"..."}
func ReservationNotesAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, event, ok := adminEvent(w, r, db, cs)
		if !ok {
			return
		}
		m, ok := decodeIDs(w, r)
		if !ok {
			return
		}
		for _, id := range m.IDs {
			evID, err := db.ReservationEventID(id)
			if err != nil || evID != event.ID {
				jsonMsg(w, http.StatusNotFound, map[string]string{"msg": "reservation not in this event"})
				return
			}
			if err := db.ReservationSetNotes(id, m.Notes); err != nil {
				log.Printf("ReservationNotesAPI: %d: %v", id, err)
				jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
				return
			}
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "ok", "rows": reservationRows(db, user, event)})
	}
}

// ReservationsMailAPI: POST /api/events/{id}/reservations/mail
// {"ids": [...], "notification_id": 8} - grupowanie po adresie e-mail robi serwer,
// jeden mail na adres, tak jak dotąd robił to JS w przeglądarce.
func ReservationsMailAPI(db *DB, mailConf MailConfig, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, event, ok := adminEvent(w, r, db, cs)
		if !ok {
			return
		}
		m, ok := decodeIDs(w, r)
		if !ok {
			return
		}
		if m.NotificationID == 0 {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "no mail template selected"})
			return
		}
		mail, err := db.NotificationGetByID(m.NotificationID, user.ID)
		if err != nil {
			jsonMsg(w, http.StatusNotFound, map[string]string{"msg": "mail template not found"})
			return
		}

		// pogrupuj rezerwacje po adresie e-mail klienta
		byEmail := map[string][]ReservationFull{}
		order := []string{}
		for _, id := range m.IDs {
			rf, err := db.ReservationFullGetByID(id, event.ID)
			if err != nil {
				log.Printf("ReservationsMailAPI: reservation %d: %v", id, err)
				continue
			}
			email := rf.CustEmail.String
			if email == "" {
				continue
			}
			if _, seen := byEmail[email]; !seen {
				order = append(order, email)
			}
			byEmail[email] = append(byEmail[email], rf)
		}

		sent, failed := 0, []string{}
		for _, email := range order {
			frs := byEmail[email]
			if err := sendEventMailToOrder(db, mailConf, user, event, mail, frs); err != nil {
				log.Printf("ReservationsMailAPI: sending to %s: %v", email, err)
				failed = append(failed, email)
				continue
			}
			sent++
		}

		status := http.StatusOK
		if len(failed) > 0 {
			status = http.StatusTeapot
		}
		jsonMsg(w, status, map[string]interface{}{
			"msg":    fmt.Sprintf("sent %d of %d", sent, len(order)),
			"sent":   sent,
			"failed": failed,
			"rows":   reservationRows(db, user, event),
		})
	}
}

// sendEventMailToOrder wysyła jeden mail dla zamówienia (grupy rezerwacji jednego
// adresu) i zapisuje wysyłkę w logu. Działa jak /api/eventanssendmail, tylko
// grupowanie i obsługa błędów są po stronie serwera.
func sendEventMailToOrder(db *DB, mailConf MailConfig, user User, event Event, mail Notification, frs []ReservationFull) error {
	if len(frs) == 0 {
		return fmt.Errorf("no reservations")
	}

	var sits, prices []int
	var rooms []string
	var total int64
	for _, rf := range frs {
		sits = append(sits, int(rf.ChairNumber))
		rooms = append(rooms, rf.RoomName)
		prices = append(prices, int(rf.Price.Int64))
		total += rf.Price.Int64
	}

	mailConf.EmbededHTMLImgs = getEmbeddedImgs(mail.EmbeddedImgsDelimited.String, user.URL, MEDIAEMAILSUBDIR, MEDIAROOT)

	first := frs[0]
	parsedText, embImgs, attachedFiles := parseEventMailTemplate(
		first.CustName.String, first.CustSurname.String, mail.Text,
		first.CustPhone.String, first.CustEmail.String, first.Notes.String,
		event.Name, sits, prices, rooms, event.ID, total, user, db)

	mailConf.Files = attachedFiles
	subject := OrderSubject(mail.Title.String, newEventOrderFuncs(
		first.CustName.String, first.CustSurname.String, first.CustPhone.String,
		first.CustEmail.String, first.Notes.String, event.Name, sits, prices, rooms,
		event.ID, total, user, db))

	if err := prepMailConfSendMail(first.CustEmail.String, subject, parsedText, user, embImgs, mailConf); err != nil {
		return err
	}

	for _, rf := range frs {
		if _, err := db.EventNotificationLogAdd(&EventNotificationLog{
			Date:           time.Now().Unix(),
			NotificationID: mail.ID,
			ReservationID:  rf.ID,
		}); err != nil {
			log.Printf("sendEventMailToOrder: log: %v", err)
		}
	}
	return nil
}

// ReservationsCSVAPI: GET /api/events/{id}/reservations.csv
func ReservationsCSVAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, event, ok := adminEvent(w, r, db, cs)
		if !ok {
			return
		}
		lang := PanelLang(r)
		rows := reservationRows(db, user, event)

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q",
			strings.ReplaceAll(event.Name, "\"", "")+".csv"))

		cw := csv.NewWriter(w)
		defer cw.Flush()

		cw.Write([]string{
			T(lang, "res.person"), T(lang, "field.rooms"), T(lang, "res.seats"),
			"e-mail", "telefon", T(lang, "res.status"), T(lang, "res.amount"),
			T(lang, "field.currency"), T(lang, "field.date"), T(lang, "status.payed"),
			"notes",
		})
		for _, row := range rows {
			cw.Write([]string{
				strings.TrimSpace(row.Name + " " + row.Surname),
				row.Room,
				strconv.FormatInt(row.Chair, 10),
				row.Email,
				row.Phone,
				T(lang, "status."+row.Status),
				strconv.FormatInt(row.Price, 10),
				row.Currency,
				FormatDate(lang, row.OrderedDate),
				FormatDate(lang, row.PayedDate),
				row.Notes,
			})
		}
	}
}

// nHistoryRow to jedna linia historii zmian statusu.
type nHistoryRow struct {
	ReservationID int64  `json:"reservation_id"`
	Date          int64  `json:"date"`
	Status        string `json:"status"`
	Admin         string `json:"admin"`
}

// ReservationsHistoryAPI: GET /api/events/{id}/reservations/history?ids=1,2,3
// Historia zmian statusu dla wskazanych rezerwacji (panel szczegółów).
func ReservationsHistoryAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		_, event, ok := adminEvent(w, r, db, cs)
		if !ok {
			return
		}

		var ids []int64
		for _, s := range strings.Split(r.URL.Query().Get("ids"), ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			id, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				continue
			}
			// tylko rezerwacje tej imprezy
			if evID, err := db.ReservationEventID(id); err == nil && evID == event.ID {
				ids = append(ids, id)
			}
		}

		ll, err := db.ReservationStatusLogGet(ids)
		if err != nil {
			log.Printf("ReservationsHistoryAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
			return
		}

		rows := make([]nHistoryRow, 0, len(ll))
		for _, l := range ll {
			rows = append(rows, nHistoryRow{
				ReservationID: l.ReservationID,
				Date:          l.Date,
				Status:        l.Status,
				Admin:         l.AdminEmail.String,
			})
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"rows": rows})
	}
}

// SURCHARGEKIND - pozycja dopisana przez admina (dopłata), w odróżnieniu od
// dodatku zamówionego przez gościa.
const SURCHARGEKIND = "surcharge"

// adminOrder sprawdza sesję, imprezę i to, że zamówienie należy do tej imprezy.
func adminOrder(w http.ResponseWriter, r *http.Request, db *DB, cs *sessions.CookieStore) (Event, CustomerOrder, bool) {
	_, event, ok := adminEvent(w, r, db, cs)
	if !ok {
		return Event{}, CustomerOrder{}, false
	}
	orderID, err := strconv.ParseInt(mux.Vars(r)["oid"], 10, 64)
	if err != nil {
		jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "bad order id"})
		return Event{}, CustomerOrder{}, false
	}
	order, err := db.OrderGetByID(orderID, event.ID)
	if err != nil {
		jsonMsg(w, http.StatusNotFound, map[string]string{"msg": "order not found"})
		return Event{}, CustomerOrder{}, false
	}
	return event, order, true
}

type nSurchargeMsg struct {
	Description string `json:"description"`
	Amount      int64  `json:"amount"`
	Price       int64  `json:"price"`
	Status      string `json:"status"`
}

// OrderItemAddAPI: POST /api/events/{id}/orders/{oid}/items
// Dopłata dopisywana przez admina (rozdz. 9.4: osobna pozycja, a nie zmiana
// statusu całego zamówienia).
func OrderItemAddAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		event, order, ok := adminOrder(w, r, db, cs)
		if !ok {
			return
		}

		body, _ := io.ReadAll(r.Body)
		var m nSurchargeMsg
		if err := json.Unmarshal(body, &m); err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		m.Description = strings.TrimSpace(m.Description)
		if m.Description == "" {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(PanelLang(r), "addons.need_description")})
			return
		}
		if m.Amount <= 0 {
			m.Amount = 1
		}

		id, err := db.OrderItemAdd(&OrderItem{
			Kind:        SURCHARGEKIND,
			Description: m.Description,
			Amount:      m.Amount,
			Price:       m.Price,
			Currency:    ToNS(event.DefaultCurrency),
			Status:      "ordered",
			CreatedDate: time.Now().Unix(),
			OrderID:     order.ID,
		})
		if err != nil {
			log.Printf("OrderItemAddAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{
			"msg": "created", "id": id, "addons": eventAddonsByOrder(db, event.ID),
		})
	}
}

// OrderItemStatusAPI: POST /api/events/{id}/orders/{oid}/items/{iid}/status
func OrderItemStatusAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		event, order, ok := adminOrder(w, r, db, cs)
		if !ok {
			return
		}
		itemID, err := strconv.ParseInt(mux.Vars(r)["iid"], 10, 64)
		if err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "bad item id"})
			return
		}

		body, _ := io.ReadAll(r.Body)
		var m nSurchargeMsg
		if err := json.Unmarshal(body, &m); err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": err.Error()})
			return
		}
		if m.Status != "payed" && m.Status != "ordered" {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "unknown status " + m.Status})
			return
		}

		payed := sql.NullInt64{}
		if m.Status == "payed" {
			payed = sql.NullInt64{Int64: time.Now().Unix(), Valid: true}
		}
		if err := db.OrderItemSetStatus(itemID, order.ID, m.Status, payed); err != nil {
			log.Printf("OrderItemStatusAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "ok", "addons": eventAddonsByOrder(db, event.ID)})
	}
}

// OrderItemDeleteAPI: DELETE /api/events/{id}/orders/{oid}/items/{iid}
// Kasować można tylko dopłaty dopisane przez admina - dodatki zamówione przez
// gościa są częścią zamówienia.
func OrderItemDeleteAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		event, order, ok := adminOrder(w, r, db, cs)
		if !ok {
			return
		}
		itemID, err := strconv.ParseInt(mux.Vars(r)["iid"], 10, 64)
		if err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "bad item id"})
			return
		}

		item, err := db.OrderItemGetByID(itemID, order.ID)
		if err != nil {
			jsonMsg(w, http.StatusNotFound, map[string]string{"msg": "item not found"})
			return
		}
		if item.Kind != SURCHARGEKIND {
			jsonMsg(w, http.StatusConflict, map[string]string{"msg": T(PanelLang(r), "addons.guest_item")})
			return
		}
		if err := db.OrderItemDel(itemID, order.ID); err != nil {
			log.Printf("OrderItemDeleteAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
			return
		}
		jsonMsg(w, http.StatusOK, map[string]interface{}{"msg": "deleted", "addons": eventAddonsByOrder(db, event.ID)})
	}
}

// EventAddonsCSVAPI: GET /api/events/{id}/addons.csv
// Lista dla kuchni: ile razy wybrano którą odpowiedź, plus dopłaty.
func EventAddonsCSVAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		_, event, ok := adminEvent(w, r, db, cs)
		if !ok {
			return
		}
		lang := PanelLang(r)

		counts, err := db.AddonAnswerSummary(event.ID)
		if err != nil {
			log.Printf("EventAddonsCSVAPI: %v", err)
		}
		items, err := db.OrderItemsGetByEvent(event.ID)
		if err != nil {
			log.Printf("EventAddonsCSVAPI: items: %v", err)
		}

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q",
			strings.ReplaceAll(event.Name, "\"", "")+"-dodatki.csv"))

		cw := csv.NewWriter(w)
		defer cw.Flush()

		priced := pricedAddonLabels(db, event)
		cw.Write([]string{T(lang, "addons.field"), T(lang, "addons.answer"), T(lang, "addons.count")})
		for _, c := range counts {
			label := plainLabel(c.Field)
			if priced[label] || strings.TrimSpace(c.Value) == "" {
				continue // pola z ceną są niżej, w dopłatach
			}
			cw.Write([]string{label, c.Value, strconv.FormatInt(c.Count, 10)})
		}

		// dopłaty osobno: ile sztuk i za ile
		byDesc := map[string][2]int64{}
		var order []string
		for _, it := range items {
			v, seen := byDesc[it.Description]
			if !seen {
				order = append(order, it.Description)
			}
			byDesc[it.Description] = [2]int64{v[0] + it.Amount, v[1] + it.Amount*it.Price}
		}
		if len(order) > 0 {
			cw.Write([]string{})
			cw.Write([]string{T(lang, "addons.surcharges"), T(lang, "addons.count"), T(lang, "report.sum")})
			for _, d := range order {
				v := byDesc[d]
				cw.Write([]string{d, strconv.FormatInt(v[0], 10), strconv.FormatInt(v[1], 10)})
			}
		}
	}
}

// EventReportCSVAPI: GET /api/events/{id}/report.csv - rozliczenie po cenach.
func EventReportCSVAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, event, ok := adminEvent(w, r, db, cs)
		if !ok {
			return
		}
		lang := PanelLang(r)

		v := nEventVars{}
		fillReservationSummary(db, user, event, lang, &v)
		fillEventReport(db, user, event, lang, &v)

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q",
			strings.ReplaceAll(event.Name, "\"", "")+"-rozliczenie.csv"))

		cw := csv.NewWriter(w)
		defer cw.Flush()

		cw.Write([]string{T(lang, "report.price"), T(lang, "report.seats"),
			T(lang, "report.sum"), T(lang, "report.payed"), T(lang, "report.waiting")})
		for _, row := range v.Report {
			cw.Write([]string{row.Price, strconv.FormatInt(row.Seats, 10), row.Sum, row.Payed, row.Waiting})
		}
		cw.Write([]string{T(lang, "report.total"), strconv.FormatInt(v.ReportTotal.Seats, 10),
			v.ReportTotal.Sum, v.ReportTotal.Payed, v.ReportTotal.Waiting})
	}
}

// --- strona tabeli ---------------------------------------------------------

type nReservationsVars struct {
	NPage
	ID        int64
	EventName string
	Currency  string
}

// ReservationsPage: /admin/events/{id}/reservations
func ReservationsPage(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		_, event, ok := adminEventHTML(w, r, db, cs)
		if !ok {
			return
		}
		lang := PanelLang(r)
		user, _ := db.UserGetByID(event.UserID)

		vars := nReservationsVars{
			NPage: NPage{
				Title:   event.Name + " — " + T(lang, "event.sec.reservations"),
				Crumb:   event.Name,
				OrgName: orgName(user),
			},
			ID:        event.ID,
			EventName: event.Name,
			Currency:  event.DefaultCurrency,
		}
		nRender(w, lang, vars, "tmpl/n_reservations.html")
	}
}

// adminEventHTML to adminEvent dla stron HTML (błędy jako strona, nie JSON).
func adminEventHTML(w http.ResponseWriter, r *http.Request, db *DB, cs *sessions.CookieStore) (User, Event, bool) {
	user, ok := adminUser(w, r, db, cs)
	if !ok {
		return User{}, Event{}, false
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return User{}, Event{}, false
	}
	event, err := db.EventGetByID(id)
	if err != nil || event.UserID != user.ID {
		http.NotFound(w, r)
		return User{}, Event{}, false
	}
	return user, event, true
}

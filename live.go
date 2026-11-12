package main

// live.go: stan stron gościa "na żywo".
//
// Strona sali (/res/{user}) i strona formularza (/form/{userurl}/{formurl})
// co kilkadziesiąt sekund pytają o aktualne liczby (js/n_ui.js RZ.live):
// sala o stan miejsc, formularz o panel informacyjny i liczby przy polach.
// Zmienione liczby strona "roluje" (RZ.roll), zajęte w międzyczasie miejsca
// przestają być klikalne. To tylko podgląd - o wolnym miejscu i tak decyduje
// /order (order_check.go), a cele pól nigdy niczego nie blokują.
//
// Odpowiedź ma ETag, więc przeglądarka przy braku zmian dostaje 304 bez
// treści. Endpointy są publiczne i pokazują dokładnie to, co strona gościa
// (te same funkcje budujące), nic więcej.

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// writeLive wysyła JSON z ETagiem (304, gdy przeglądarka ma już ten stan).
func writeLive(w http.ResponseWriter, r *http.Request, v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("writeLive: %v", err)
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	sum := sha1.Sum(b)
	etag := `"` + hex.EncodeToString(sum[:10]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

// liveSeat - stan jednego krzesła: numer, stan dla gościa (free/taken/
// disabled), dokładny stan zajętego (temat klasyczny) i opis z dymka.
type liveSeat struct {
	N  int64  `json:"n"`
	S  string `json:"s"`
	St string `json:"st,omitempty"`
	T  string `json:"t"`
}

type liveRoom struct {
	ID    int64      `json:"id"`
	Free  int        `json:"free"`
	Total int        `json:"total"`
	Seats []liveSeat `json:"seats"`
}

// GuestRoomLiveAPI - GET /res/{user}/live?event-id=N: stan miejsc imprezy.
func GuestRoomLiveAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		gone := func(why string, args ...interface{}) {
			log.Printf("GuestRoomLiveAPI: "+why, args...)
			jsonMsg(w, http.StatusNotFound, map[string]string{"msg": "not found"})
		}
		user, err := db.UserGetByURL(mux.Vars(r)["user"])
		if err != nil {
			gone("unknown organization %q", mux.Vars(r)["user"])
			return
		}
		id, err := strconv.ParseInt(r.URL.Query().Get("event-id"), 10, 64)
		if err != nil {
			gone("bad event-id %q", r.URL.Query().Get("event-id"))
			return
		}
		event, err := db.EventGetByID(id)
		if err != nil || event.UserID != user.ID {
			gone("event %d not of user %d: %v", id, user.ID, err)
			return
		}
		// szkic widzi tylko admin, tak jak samą stronę
		if event.Status.String == "draft" && !isAdminSession(r, db, cs) {
			gone("draft event %d", id)
			return
		}
		rooms, err := db.EventGetRooms(event.ID)
		if err != nil {
			gone("rooms of event %d: %v", id, err)
			return
		}
		lng := KnownLang(event.Language.String)

		out := struct {
			Rooms []liveRoom `json:"rooms"`
		}{Rooms: []liveRoom{}}
		for _, room := range rooms {
			gr := guestRoom(db, room, event, lng)
			lr := liveRoom{ID: gr.RoomID, Free: gr.SeatsFree, Total: gr.SeatsTotal, Seats: []liveSeat{}}
			for _, c := range gr.Chairs {
				lr.Seats = append(lr.Seats, liveSeat{N: c.Number, S: c.Status, St: c.State, T: c.Title})
			}
			out.Rooms = append(out.Rooms, lr)
		}
		writeLive(w, r, out)
	}
}

// guestFormInfo składa panel informacyjny formularza i stany pól
// pokazywane przy polach (panel_place) - dla strony i dla /live.
func guestFormInfo(db *DB, user User, templ FormTemplate, lang string) (template.HTML, map[string]PanelItem) {
	if mode := panelMode(templ); mode == PANELAUTO || mode == PANELFIELDS {
		pv := BuildPanel(db, user, templ, mode)
		return renderPanelVars(lang, pv), pv.Fields
	}
	return RenderInfoPanel(db, user, templ, panelMode(templ)), nil
}

// liveOpt - liczba przy opcji ("3 / 10"), pusta = bez liczby.
type liveOpt struct {
	T string `json:"t"`
	D bool   `json:"d,omitempty"`
}

// liveField - stan pola: wiersz pod polem (HTML szablonu "ffstat") albo
// liczby przy opcjach; klucz opcji to data-when + "|" + wartość.
type liveField struct {
	Stat string             `json:"stat,omitempty"`
	Opts map[string]liveOpt `json:"opts,omitempty"`
}

func optStatText(s *OptionStat) liveOpt {
	if s == nil {
		return liveOpt{}
	}
	t := strconv.FormatInt(s.Count, 10)
	if s.Goal > 0 {
		t += fmt.Sprintf(" / %d", s.Goal)
	}
	return liveOpt{T: t, D: s.Done()}
}

// GuestFormLiveAPI - GET /form/{userurl}/{formurl}/live: panel i liczby pól.
func GuestFormLiveAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, templ, ok := guestFormAPI(w, r, db)
		if !ok {
			return
		}
		// zamknięty formularz gość widzi bez panelu; podgląd ma tylko admin
		if !templ.IsOpen(nowUnix()) && !isFormOwnerSession(r, db, cs, user) {
			jsonMsg(w, http.StatusGone, map[string]string{"msg": "closed"})
			return
		}
		lang := formLang(templ)
		defs, err := ParseFormDef(templ.Content.String)
		if err != nil {
			log.Printf("GuestFormLiveAPI: form %d: %v", templ.ID, err)
		}
		info, items := guestFormInfo(db, user, templ, lang)
		rows := BuildRenderRows(defs, templ.MoneyAmountFieldName.String)
		AttachFieldStats(rows, items)

		out := struct {
			Info   string               `json:"info"`
			Fields map[string]liveField `json:"fields"`
		}{Info: string(info), Fields: map[string]liveField{}}

		var stat *template.Template
		for _, row := range rows {
			for _, f := range row.Fields {
				if f.Stat != nil {
					if stat == nil {
						if stat, err = template.New("n_formfield.html").Funcs(GuestFuncs(lang, templ.Theme)).ParseFiles("tmpl/n_formfield.html"); err != nil {
							log.Printf("GuestFormLiveAPI: %v", err)
							http.Error(w, "template error", http.StatusInternalServerError)
							return
						}
					}
					var buf bytes.Buffer
					if err := stat.ExecuteTemplate(&buf, "ffstat", f.Stat); err != nil {
						log.Printf("GuestFormLiveAPI: ffstat %s: %v", f.Name, err)
					}
					out.Fields[f.Name] = liveField{Stat: buf.String()}
					continue
				}
				if _, ok := items[f.Name]; !ok || len(f.Options) == 0 {
					continue
				}
				lf := liveField{Opts: map[string]liveOpt{}}
				for _, o := range f.Options {
					lf.Opts[o.When+"|"+o.Value] = optStatText(o.Stat)
				}
				out.Fields[f.Name] = lf
			}
		}
		writeLive(w, r, out)
	}
}

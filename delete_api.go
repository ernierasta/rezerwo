package main

// delete_api.go: kasowanie imprez, sal, maili i kont z panelu admina.
// Strony pokazują przycisk z data-delete (okno potwierdzenia w js/n_ui.js),
// formularze mają własne okno (FormDeleteAPI, js/n_form.js).

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// EventDeleteAPI - DELETE /api/events/{id}
func EventDeleteAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, event, ok := adminEvent(w, r, db, cs)
		if !ok {
			return
		}
		if err := db.EventDelete(event.ID, user.ID); err != nil {
			log.Printf("EventDeleteAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(PanelLang(r), "del.failed")})
			return
		}
		log.Printf("EventDeleteAPI: user %d deleted event %d %q", user.ID, event.ID, event.Name)
		jsonMsg(w, http.StatusOK, map[string]string{"msg": "deleted", "next": "/admin"})
	}
}

// RoomDeleteAPI - DELETE /api/rooms/{id}
func RoomDeleteAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		lang := PanelLang(r)
		roomID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
		if err != nil || !userOwnsRoom(db, user.ID, roomID) {
			jsonMsg(w, http.StatusNotFound, map[string]string{"msg": "room not found"})
			return
		}
		if err := db.RoomDelete(roomID, user.ID); err != nil {
			if errors.Is(err, ErrRoomHasReservations) {
				jsonMsg(w, http.StatusConflict, map[string]string{"msg": T(lang, "del.room_has_res")})
				return
			}
			log.Printf("RoomDeleteAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(lang, "del.failed")})
			return
		}
		log.Printf("RoomDeleteAPI: user %d deleted room %d", user.ID, roomID)
		jsonMsg(w, http.StatusOK, map[string]string{"msg": "deleted", "next": "/admin/catalogs"})
	}
}

// MailDeleteAPI - DELETE /api/mails/{id}
func MailDeleteAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, mail, ok := adminMail(w, r, db, cs, false)
		if !ok {
			return
		}
		if err := db.NotificationDelete(mail.ID, user.ID); err != nil {
			log.Printf("MailDeleteAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(PanelLang(r), "del.failed")})
			return
		}
		log.Printf("MailDeleteAPI: user %d deleted mail %d %q", user.ID, mail.ID, mail.Name)
		jsonMsg(w, http.StatusOK, map[string]string{"msg": "deleted", "next": "/admin/catalogs"})
	}
}

// AccountDeleteAPI - DELETE /api/accounts/{id}
func AccountDeleteAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, acc, ok := adminAccount(w, r, db, cs, false)
		if !ok {
			return
		}
		if err := db.BankAccountDelete(acc.ID, user.ID); err != nil {
			log.Printf("AccountDeleteAPI: %v", err)
			jsonMsg(w, http.StatusTeapot, map[string]string{"msg": T(PanelLang(r), "del.failed")})
			return
		}
		log.Printf("AccountDeleteAPI: user %d deleted bank account %d %q", user.ID, acc.ID, acc.Name)
		jsonMsg(w, http.StatusOK, map[string]string{"msg": "deleted", "next": "/admin/catalogs"})
	}
}

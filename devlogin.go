package main

import (
	"log"
	"net"
	"net/http"
	"os"

	"github.com/gorilla/sessions"
)

// DEVLOGINENV holds email of the user logged in by /admin/devlogin. Development
// only: when it is not set (production), the endpoint does not exist.
const DEVLOGINENV = "REZERWO_DEV_LOGIN"

// DevLogin logs in admin set in REZERWO_DEV_LOGIN without password, so the
// app can be tested in browser by automation. Only requests from localhost
// without proxy headers are accepted.
func DevLogin(db *DB, cookieStore *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		email := os.Getenv(DEVLOGINENV)
		if email == "" || !isLocalRequest(r) {
			http.NotFound(w, r)
			return
		}
		if _, err := db.UserGetPass(email); err != nil {
			log.Printf("DevLogin: user %q not found, err: %v", email, err)
			http.Error(w, "dev login user not found", http.StatusInternalServerError)
			return
		}
		session, err := cookieStore.Get(r, AUTHCOOKIE)
		if err != nil {
			// invalid cookie, new session is returned, we overwrite the cookie
			log.Printf("DevLogin: session error: %v", err)
		}
		session.Values["email"] = email
		session.Values["role"] = "admin"
		if err := session.Save(r, w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("DevLogin: %s logged in as admin", email)
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}

// isLocalRequest reports whether request comes directly from loopback address
func isLocalRequest(r *http.Request) bool {
	if r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Real-IP") != "" || r.Header.Get("Forwarded") != "" {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

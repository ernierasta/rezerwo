package main

// auth.go: hasła adminów (users.passwd) i logowanie.
//
// Hasła trzymamy jako hash bcrypt. Starsze bazy miały je jawnym tekstem -
// przy starcie serwera (HashPlainPasswords) i przy logowaniu (checkPassword)
// takie hasło zamieniamy na hash, więc ręcznie wpisane w bazie hasło
// dalej działa i od razu przestaje być jawne.

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

// isBcryptHash - hash bcrypt zaczyna się od $2a$, $2b$ albo $2y$.
func isBcryptHash(s string) bool {
	return len(s) == 60 && strings.HasPrefix(s, "$2")
}

func hashPassword(pass string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	return string(h), err
}

// dummyHash - porównanie z nim przy nieznanym e-mailu, żeby czas odpowiedzi
// nie zdradzał, czy konto istnieje.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("rezerwo-dummy-password"), bcrypt.DefaultCost)

// checkPassword sprawdza hasło admina organizacji. Jawne hasło z bazy
// po udanym logowaniu zamienia na hash.
func checkPassword(db *DB, email, pass string) bool {
	stored, err := db.UserGetPass(email)
	if err != nil || stored == "" || pass == "" {
		bcrypt.CompareHashAndPassword(dummyHash, []byte(pass))
		return false
	}
	if isBcryptHash(stored) {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(pass)) == nil
	}
	if subtle.ConstantTimeCompare([]byte(stored), []byte(pass)) != 1 {
		return false
	}
	if h, err := hashPassword(pass); err == nil {
		if err := db.UserSetPass(email, h); err != nil {
			log.Printf("checkPassword: can not store hash for %q: %v", email, err)
		}
	}
	return true
}

// HashPlainPasswords zamienia wszystkie jawne hasła w users na hashe bcrypt.
func HashPlainPasswords(db *DB) {
	users, err := db.UserGetAll()
	if err != nil {
		log.Printf("HashPlainPasswords: %v", err)
		return
	}
	n := 0
	for _, u := range users {
		if u.Passwd == "" || isBcryptHash(u.Passwd) {
			continue
		}
		h, err := hashPassword(u.Passwd)
		if err != nil {
			log.Printf("HashPlainPasswords: %q: %v", u.Email, err)
			continue
		}
		if err := db.UserSetPass(u.Email, h); err != nil {
			log.Printf("HashPlainPasswords: %q: %v", u.Email, err)
			continue
		}
		n++
	}
	if n > 0 {
		log.Printf("HashPlainPasswords: %d plain text password(s) replaced by bcrypt hash", n)
	}
}

type LoginAPIMsg struct {
	Email    string `json:"email"`
	Pass     string `json:"password"`
	Remember string `json:"remember-me"`
}

// sessionOptions - ważność ciasteczka sesji: 2 godziny albo 2 tygodnie
// przy "Zapamiętaj mnie". Ustawiane na konkretnej sesji, nie na całym
// cookieStore, żeby wybór jednej osoby nie zmieniał sesji innych.
func sessionOptions(remember bool) *sessions.Options {
	maxAge := 60 * 60 * 2
	if remember {
		maxAge = 86400 * 14
	}
	return &sessions.Options{
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

// LoginAPI - POST /api/login {email, password, remember-me}
func LoginAPI(db *DB, cookieStore *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonMsg(w, http.StatusMethodNotAllowed, map[string]string{"msg": "POST only"})
			return
		}
		var m LoginAPIMsg
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": "bad request"})
			return
		}
		m.Email = strings.TrimSpace(m.Email)

		if !checkPassword(db, m.Email, m.Pass) {
			log.Printf("LoginAPI: failed login for %q", m.Email)
			jsonMsg(w, http.StatusUnauthorized, map[string]string{"msg": "wrong e-mail or password"})
			return
		}

		// nowa sesja zamiast istniejącej (ochrona przed podrzuceniem sesji)
		session, err := cookieStore.New(r, AUTHCOOKIE)
		if err != nil && session == nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		session.Options = sessionOptions(m.Remember == "on")
		session.Values["email"] = m.Email
		session.Values["role"] = "admin"
		if err := session.Save(r, w); err != nil {
			log.Printf("LoginAPI: can not save session: %v", err)
			http.Error(w, "session error", http.StatusInternalServerError)
			return
		}
		log.Printf("LoginAPI: %q logged in", m.Email)
		// panel w języku wybranym przez tego admina (admins.language)
		ApplyAdminLanguage(w, db, m.Email)
		jsonMsg(w, http.StatusOK, map[string]string{"resp": "OK"})
	}
}

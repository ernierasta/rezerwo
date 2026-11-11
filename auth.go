package main

// auth.go: hasła adminów (users.passwd) i logowanie.
//
// Logują się dwa rodzaje kont: konto organizacji (users.email/passwd) i jej
// menedżerowie (admins, type=MANAGER, managers_ui.go). Menedżer ma dostęp do
// panelu swojej organizacji, ale nie zarządza loginami menedżerów. Sesję
// menedżera sprawdzamy w bazie przy każdym żądaniu (sessionLogin): usunięcie
// konta, zmiana e-maila albo hasła od razu ją unieważnia.
//
// Hasła trzymamy jako hash bcrypt. Starsze bazy miały je jawnym tekstem -
// przy starcie serwera (HashPlainPasswords) i przy logowaniu (checkPassword)
// takie hasło zamieniamy na hash, więc ręcznie wpisane w bazie hasło
// dalej działa i od razu przestaje być jawne.

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

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

// checkManagerPassword sprawdza hasło menedżera (zawsze hash - ustawiane
// tylko z panelu).
func checkManagerPassword(db *DB, email, pass string) (Admin, bool) {
	a, err := db.ManagerGetByEmail(email)
	if err != nil || !isBcryptHash(a.Passwd.String) || pass == "" {
		bcrypt.CompareHashAndPassword(dummyHash, []byte(pass))
		return Admin{}, false
	}
	if bcrypt.CompareHashAndPassword([]byte(a.Passwd.String), []byte(pass)) != nil {
		return Admin{}, false
	}
	return a, true
}

// checkLogin - konto organizacji albo menedżer. Zwraca menedżera (ID 0 dla
// konta organizacji).
func checkLogin(db *DB, email, pass string) (Admin, bool) {
	if _, err := db.UserGetPass(email); err == nil {
		return Admin{}, checkPassword(db, email, pass)
	}
	return checkManagerPassword(db, email, pass)
}

// passFingerprint - skrót hasha hasła menedżera w sesji: nowe hasło
// unieważnia wcześniejsze sesje.
func passFingerprint(hash string) string {
	sum := sha256.Sum256([]byte(hash))
	return hex.EncodeToString(sum[:8])
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

		mgr, ok := checkLogin(db, m.Email, m.Pass)
		if !ok {
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
		if mgr.ID != 0 {
			session.Values["email"] = mgr.Email
			session.Values["manager"] = mgr.ID
			session.Values["pv"] = passFingerprint(mgr.Passwd.String)
		}
		if err := session.Save(r, w); err != nil {
			log.Printf("LoginAPI: can not save session: %v", err)
			http.Error(w, "session error", http.StatusInternalServerError)
			return
		}
		if mgr.ID != 0 {
			m.Email = mgr.Email
			if err := db.ManagerSetLastLogin(mgr.ID, time.Now().Unix()); err != nil {
				log.Printf("LoginAPI: last login of %q: %v", mgr.Email, err)
			}
			log.Printf("LoginAPI: manager %q (org %d) logged in", mgr.Email, mgr.UserID)
		} else {
			log.Printf("LoginAPI: %q logged in", m.Email)
		}
		// panel w języku wybranym przez tego admina (admins.language)
		ApplyAdminLanguage(w, db, m.Email)
		jsonMsg(w, http.StatusOK, map[string]string{"resp": "OK"})
	}
}

func AdminLoginHTML() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		lng := PanelLang(r)
		guestRender(w, lng, struct{ Title string }{T(lng, "login.title")}, "tmpl/n_login.html")
	}
}

// AdminLogout removes admin session cookie and redirects to login page
func AdminLogout(cookieStore *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := cookieStore.Get(r, AUTHCOOKIE)
		if err != nil {
			// invalid cookie, new session is returned, we still overwrite the cookie
			log.Printf("AdminLogout: session error: %v", err)
		}
		session.Values = map[interface{}]interface{}{}
		session.Options.MaxAge = -1
		if err := session.Save(r, w); err != nil {
			log.Printf("AdminLogout: can not remove session, err: %v", err)
		}
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	}
}

// sessionAdmin zwraca e-mail admina z ważnej sesji albo "" - bez
// przekierowań (stare albo zepsute ciasteczko to po prostu brak sesji).
func sessionAdmin(r *http.Request, cs *sessions.CookieStore) string {
	if cs == nil {
		return ""
	}
	session, err := cs.Get(r, AUTHCOOKIE)
	if err != nil {
		return ""
	}
	if role, _ := session.Values["role"].(string); role != "admin" {
		return ""
	}
	email, _ := session.Values["email"].(string)
	return email
}

// Login - kto jest zalogowany: organizacja, e-mail logowania i czy to
// menedżer (a nie konto organizacji).
type Login struct {
	User    User
	Email   string
	Manager bool
}

// sessionLogin sprawdza sesję w bazie - bez przekierowań. Menedżer musi
// dalej istnieć z tym samym e-mailem i hasłem co przy logowaniu.
func sessionLogin(r *http.Request, db *DB, cs *sessions.CookieStore) (Login, bool) {
	if cs == nil || db == nil {
		return Login{}, false
	}
	session, err := cs.Get(r, AUTHCOOKIE)
	if err != nil {
		return Login{}, false
	}
	if role, _ := session.Values["role"].(string); role != "admin" {
		return Login{}, false
	}
	email, _ := session.Values["email"].(string)
	if email == "" {
		return Login{}, false
	}
	if mid, ok := session.Values["manager"].(int64); ok {
		a, err := db.ManagerGetByEmail(email)
		pv, _ := session.Values["pv"].(string)
		if err != nil || a.ID != mid || a.Email != email ||
			subtle.ConstantTimeCompare([]byte(passFingerprint(a.Passwd.String)), []byte(pv)) != 1 {
			return Login{}, false
		}
		u, err := db.UserGetByID(a.UserID)
		if err != nil {
			return Login{}, false
		}
		return Login{User: u, Email: a.Email, Manager: true}, true
	}
	u, err := db.UserGetByEmail(email)
	if err != nil {
		return Login{}, false
	}
	return Login{User: u, Email: email}, true
}

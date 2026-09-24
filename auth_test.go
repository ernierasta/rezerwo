package main

// Testy haseł i logowania na KOPII bazy w tmp/ (copyDBForTest).

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
)

const testUser = "admin@zori.cz"

func TestPasswordHashing(t *testing.T) {
	path := copyDBForTest(t)
	db := DBInit(path)
	db.MustConnect()
	defer db.Close()

	// jak w starej bazie: jawne hasło
	if err := db.UserSetPass(testUser, "tajne123"); err != nil {
		t.Fatal(err)
	}
	if checkPassword(db, testUser, "zle") {
		t.Fatal("wrong password accepted")
	}
	if !checkPassword(db, testUser, "tajne123") {
		t.Fatal("plain text password not accepted")
	}
	stored, _ := db.UserGetPass(testUser)
	if !isBcryptHash(stored) {
		t.Fatalf("password not rehashed after login: %q", stored)
	}
	if !checkPassword(db, testUser, "tajne123") {
		t.Fatal("hashed password not accepted")
	}

	// nieznany e-mail i puste hasło nie przechodzą
	if checkPassword(db, "nikt@example.com", "") || checkPassword(db, testUser, "") {
		t.Fatal("empty password or unknown user accepted")
	}

	// hashowanie przy starcie
	db.UserSetPass(testUser, "inne456")
	HashPlainPasswords(db)
	users, _ := db.UserGetAll()
	for _, u := range users {
		if u.Passwd != "" && !isBcryptHash(u.Passwd) {
			t.Fatalf("user %q still has plain text password", u.Email)
		}
	}
	if !checkPassword(db, testUser, "inne456") {
		t.Fatal("password hashed at startup not accepted")
	}
}

func TestLoginRememberDoesNotLeak(t *testing.T) {
	path := copyDBForTest(t)
	db := DBInit(path)
	db.MustConnect()
	defer db.Close()
	db.UserSetPass(testUser, "tajne123")

	cs := sessions.NewCookieStore([]byte("0123456789abcdef0123456789abcdef"))
	cs.Options = sessionOptions(false)
	h := LoginAPI(db, cs)

	login := func(body string) *http.Response {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest("POST", "/api/login", bytes.NewBufferString(body)))
		return rec.Result()
	}

	r := login(`{"email":"admin@zori.cz","password":"tajne123","remember-me":"on"}`)
	if r.StatusCode != 200 || len(r.Cookies()) == 0 {
		t.Fatalf("login failed: %d", r.StatusCode)
	}
	for _, c := range r.Cookies() {
		if c.Name == AUTHCOOKIE && c.MaxAge != 86400*14 {
			t.Fatalf("remember me: MaxAge %d", c.MaxAge)
		}
	}
	if cs.Options.MaxAge != 60*60*2 {
		t.Fatalf("remember me changed store defaults: %d", cs.Options.MaxAge)
	}

	r = login(`{"email":"admin@zori.cz","password":"tajne123"}`)
	for _, c := range r.Cookies() {
		if c.Name == AUTHCOOKIE && c.MaxAge != 60*60*2 {
			t.Fatalf("next login got MaxAge %d", c.MaxAge)
		}
	}

	if r := login(`{"email":"admin@zori.cz","password":"zle"}`); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password: %d", r.StatusCode)
	}
	// dawniej rola inna niż "admin" omijała sprawdzanie hasła
	if r := login(`{"email":"admin@zori.cz","password":"x","role":"customer"}`); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("role bypass: %d", r.StatusCode)
	}
}

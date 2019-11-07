package main

import (
	"database/sql"
	"os"
	"testing"
)

var (
	filename = "test_db_sqllite.db"
	user1    = &User{
		Email:        "mail@testing.com",
		Passwd:       "somehashhere",
		Name:         sql.NullString{"Tester", true},
		Surname:      sql.NullString{"Overtestosteron", true},
		Organization: sql.NullString{"Golang Corp", true},
	}
	user1b = &User{
		Email:        "mail@testing.com",
		Passwd:       "otherhashhere",
		Name:         sql.NullString{"Tester", true},
		Surname:      sql.NullString{"Overtestosteron", true},
		Organization: sql.NullString{"ErnieRasta Corp", true},
	}
	room1 = &Room{
		Name:   "room1",
		Width:  1000,
		Height: 1000,
	}
	room1b = &Room{
		Name:   "room1b",
		Width:  1005,
		Height: 1010,
	}
	furniture1 = &Furniture{
		Number:      1,
		Type:        "table",
		Orientation: sql.NullString{"horizontal", true},
		X:           100,
		Y:           50,
		Label:       sql.NullString{"fajny stół", true},
		Capacity:    sql.NullInt64{6, true},
	}
	furniture1b = &Furniture{
		Number:      1,
		Type:        "table",
		Orientation: sql.NullString{"horizontal", true},
		X:           150,
		Y:           100,
		Label:       sql.NullString{"super fajny stół", true},
		Capacity:    sql.NullInt64{9, true},
	}
	event1 = &Event{
		Name:            "Balik malik",
		FromDate:        12345,
		ToDate:          23456,
		DefaultPrice:    300,
		DefaultCurrency: "Kč",
		HowTo:           "here is description",
	}
)

func Setup() *DB {
	os.Remove(filename)
	db := DBInit(filename)
	db.MustConnect()
	db.StructureCreate()
	return db
}

func Raport(note string, db *DB, t *testing.T) {
	u, err := db.UserGetAll()
	if err != nil {
		t.Error(err)
	}
	r, err := db.RoomGetAll()
	if err != nil {
		t.Error(err)
	}
	t.Log(note, u, r)
}

func CleanUp(db *DB, t *testing.T) {
	db.Close()
	if err := os.Remove(filename); err != nil {
		t.Error(err)
	}
}

func TestUser(t *testing.T) {
	db := Setup()
	if id, err := db.UserAdd(user1); err != nil {
		t.Errorf("problem adding user: %v, err: %v", user1, err)
	} else {
		user1.ID = id
	}
	if _, err := db.UserAdd(user1); err == nil {
		t.Errorf("expected error adding the same user: %v, err: %v", user1, err)
	} else {
		t.Log(err)
	}
	if u, err := db.UserGetByEmail(user1.Email); err != nil {
		t.Errorf("problem retrieving user by email: %q, User: %v, err: %v", user1.Email, u, err)
	}
	if u, err := db.UserGetByEmail("nonexisting@none.no"); err == nil {
		t.Errorf("expected error when retrieving nonexisting user, User: %v", u)
	} else {
		if err == sql.ErrNoRows {
			t.Logf("ErrNoRows detected as expected: %v", err)
		} else {
			t.Errorf("expected sql.ErrNoRows error, got different: %v", err)
		}
	}
	if err := db.UserMod(user1b); err != nil {
		t.Errorf("problem updating user, new data: %v, err: %v", user1b, err)
	}
	Raport("before deleting user:", db, t)
	if err := db.UserDel(user1.Email); err != nil {
		t.Errorf("problem deleting user by email: %q, err: %v", user1.Email, err)
	}
	Raport("users test end:", db, t)
	CleanUp(db, t)
}

func TestRoom(t *testing.T) {
	db := Setup()
	if id, err := db.RoomAdd(room1); err != nil {
		t.Errorf("problem adding room: %v, err: %v", room1, err)
	} else {
		if id != 1 {
			t.Errorf("first room ID should be 1, is: %d", id)
		}
		room1.ID = id
	}
	if id, err := db.RoomAdd(room1); err == nil {
		t.Errorf("expected error adding the same room: %v, id:%d err: %v", room1, id, err)
	} else {
		t.Log(err)
	}
	if r, err := db.RoomGetByName(room1.Name); err != nil {
		t.Errorf("problem retrieving room by name: %q, Room: %v, err: %v", room1.Name, r, err)
	} else {
		t.Log(r)
	}
	if r, err := db.RoomGetByName("nonexisting"); err == nil {
		t.Errorf("expected error when retrieving nonexisting room, Room: %v", r)
	} else {
		if err == sql.ErrNoRows {
			t.Logf("ErrNoRows detected as expected: %v", err)
		} else {
			t.Errorf("expected sql.ErrNoRows error, got different: %v", err)
		}
	}
	if err := db.RoomMod(room1b); err != nil {
		t.Errorf("problem updating room, new data: %v, err: %v", room1b, err)
	}
	Raport("before deleting room:", db, t)
	if err := db.RoomDel(room1.ID); err != nil {
		t.Errorf("problem deleting room by ID: %d, err: %v", room1.ID, err)
	}
	Raport("rooms test end:", db, t)
	CleanUp(db, t)
}

func TestFurniture(t *testing.T) {
	db := Setup()
	rID, err := db.RoomAdd(room1)
	room1.ID = rID
	if err != nil {
		t.Fatal(err)
	}
	rID, err = db.RoomAdd(room1b)
	room1b.ID = rID
	if err != nil {
		t.Fatal(err)
	}

	furniture1.RoomID = rID
	fID, err := db.FurnitureAddOrUpdate(furniture1)
	if err != nil {
		t.Errorf("problem adding furniture: %+v, err: %v ", furniture1, err)
	} else {
		furniture1.ID = fID
		t.Log(furniture1)
	}
	// update the same furniture
	fID, err = db.FurnitureAddOrUpdate(furniture1)
	if err != nil {
		t.Errorf("problem re-adding(updating) furniture: %+v, err: %v ", furniture1, err)
	}

	// change room
	err = db.FurnitureChangeRoomByName(furniture1.Number, furniture1.Type, room1b.Name)
	if err != nil {
		t.Errorf("problem changing room by name, err: %v", err)
	}

	// update furniture attribs
	err = db.FurnitureMod(furniture1b)
	if err != nil {
		t.Errorf("problem updating furniture, err: %v", err)
	}

	// remove furniture1
	err = db.FurnitureDelByNumberType(furniture1.Number, furniture1.Type)
	if err != nil {
		t.Errorf("problem deleting furniture, err: %v", err)
	}
	CleanUp(db, t)

}

func TestPrice(t *testing.T) {

}

func TestEventsRoom(t *testing.T) {
	db := Setup()

	uID, err := db.UserAdd(user1)
	if err != nil {
		t.Fatal(err)
	}
	event1.UserID = uID
	eID, err := db.EventAddOrUpdate(event1)
	if err != nil {
		t.Fatal(err)
	}
	rID, err := db.RoomAdd(room1)
	room1.ID = rID
	if err != nil {
		t.Fatal(err)
	}

	rID1b, err := db.RoomAdd(room1b)
	room1b.ID = rID1b
	if err != nil {
		t.Fatal(err)
	}

	err = db.RoomAddToEvent(room1.ID, eID)
	if err != nil {
		t.Fatal(err)
	}
	err = db.RoomAddToEvent(room1b.ID, eID)
	if err != nil {
		t.Fatal(err)
	}

	rr, err := db.EventGetRooms(eID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rr) != 2 {
		t.Errorf("wrong ammount of rooms, expected 2, have %v", len(rr))
	}
	for _, r := range rr {
		if r.Name != room1.Name && r.Name != room1b.Name {
			t.Errorf("unexpected name: %q", r.Name)
		}
	}
	t.Log(rr)
}

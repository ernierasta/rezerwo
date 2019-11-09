package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	// Workaround bug:
	//"github.com/demiurgestudios/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

const (
	ConnOptions = "?cache=shared&mode=rwc&_busy_timeout=999999"
)

type User struct {
	ID           int64          `db:"id"`
	Email        string         `db:"email"`
	URL          string         `db:"url"`
	Passwd       string         `db:"passwd"`
	Name         sql.NullString `db:"name"`
	Surname      sql.NullString `db:"surname"`
	Organization sql.NullString `db:"organization"`
	Phone        sql.NullString `db:"phone"`
}

type Customer struct {
	User
	Notes string `db:"notes"`
}

type Room struct {
	ID          int64          `db:"id"`
	Name        string         `db:"name"`
	Description sql.NullString `db:"description"`
	Width       int64          `db:"width"`
	Height      int64          `db:"height"`
}

type Furniture struct {
	ID          int64          `db:"id"`
	Number      int64          `db:"number"`
	Type        string         `db:"type"`
	Orientation sql.NullString `db:"orientation"`
	X           int64          `db:"x"`
	Y           int64          `db:"y"`
	Width       sql.NullInt64  `db:"width"`
	Height      sql.NullInt64  `db:"height"`
	Color       sql.NullString `db:"color"`
	Label       sql.NullString `db:"label"`
	Capacity    sql.NullInt64  `db:"capacity"`
	RoomID      int64          `db:"rooms_id_fk"`
}

type UsersRooms struct {
	UserID int64 `db:"users_id_fk"`
	RoomID int64 `db:"rooms_id_fk"`
}

type Price struct {
	ID          int64  `db:"id"`
	Price       int64  `db:"price"`
	Currency    string `db:"currency"`
	Disabled    int64  `db:"disabled"`
	EventID     int64  `db:"events_id_fk"`
	FurnitureID int64  `db:"furnitures_id_fk"`
}

type Event struct {
	ID              int64  `db:"id"`
	Name            string `db:"name"`
	Date            int64  `db:"date"`
	FromDate        int64  `db:"from_date"`
	ToDate          int64  `db:"to_date"`
	DefaultPrice    int64  `db:"default_price"`
	DefaultCurrency string `db:"default_currency"`
	OrderedNote     string `db:"ordered_note"`
	MailSubject     string `db:"mail_subject"`
	MailText        string `db:"mail_text"`
	HowTo           string `db:"how_to"`
	UserID          int64  `db:"users_id_fk"`
}

// TODO: would we ever use this type of stucts in go?
type EventsRooms struct {
	EventID int64 `db:"events_id_fk"`
	RoomID  int64 `db:"rooms_id_fk"`
}

type EventFull struct {
	Event
	EventsRooms
}

type Reservation struct {
	ID          int64          `db:"id"`
	OrderedDate sql.NullInt64  `db:"ordered_date"`
	PayedDate   sql.NullInt64  `db:"payed_date"`
	Price       sql.NullInt64  `db:"price"`
	Currency    sql.NullString `db:"currency"`
	// Status: free, marked, ordered, confirmed, disabled
	Status      string `db:"status"`
	FurnitureID int64  `db:"furnitures_id_fk"`
	EventID     int64  `db:"events_id_fk"`
	CustomerID  int64  `db:"customers_id_fk"`
}

type FurnitureFull struct {
	Furniture
	AdminPrice    sql.NullInt64  `db:"admin_price"`
	AdminCurrency sql.NullString `db:"admin_currency"`
	AdminDisabled sql.NullInt64  `db:"admin_disabled"`
	BoughtPrice   sql.NullInt64  `db:"bought_price"`
	Status        sql.NullString `db:"status"`
	OrderedDate   sql.NullInt64  `db:"ordered_date"`
	PayedDate     sql.NullInt64  `db:"payed_date"`
	CustomerID    sql.NullInt64  `db:"reservation_customers_id_fk"`
}

type DB struct {
	FileName string
	DB       *sqlx.DB
}

func DBInit(filename string) *DB {
	return &DB{
		FileName: filename,
	}
}

func (db *DB) MustConnect() {
	db.DB = sqlx.MustConnect("sqlite3", db.FileName+ConnOptions)
	db.DB.SetMaxOpenConns(1)
}

func (db *DB) StructureCreate() {
	structure := `
	CREATE TABLE IF NOT EXISTS users (id INTEGER NOT NULL PRIMARY KEY, email TEXT NOT NULL UNIQUE, url TEXT NOT NULL, passwd TEXT NOT NULL, name TEXT, surname TEXT, organization TEXT, phone TEXT);
	CREATE TABLE IF NOT EXISTS rooms (id INTEGER NOT NULL PRIMARY KEY, name TEXT NOT NULL UNIQUE, description TEXT, width INTEGER NOT NULL, height INTEGER NOT NULL);
	CREATE TABLE IF NOT EXISTS users_rooms (users_id_fk INTEGER NOT NULL, rooms_id_fk INTEGER NOT NULL UNIQUE, FOREIGN KEY(users_id_fk) REFERENCES users(id), FOREIGN KEY(rooms_id_fk) REFERENCES rooms(id));
	CREATE TABLE IF NOT EXISTS furnitures (id INTEGER NOT NULL PRIMARY KEY, number INTEGER NOT NULL, type TEXT NOT NULL, orientation TEXT, x INTEGER NOT NULL, y INTEGER NOT NULL, width INTEGER, height INTEGER, color TEXT, label TEXT, capacity INTEGER, rooms_id_fk INTEGER NOT NULL, UNIQUE(number, type, rooms_id_fk) ON CONFLICT ROLLBACK,FOREIGN KEY(rooms_id_fk) REFERENCES rooms(id));
	CREATE TABLE IF NOT EXISTS prices (id INTEGER NOT NULL PRIMARY KEY, price INTEGER NOT NULL, currency TEXT NOT NULL, disabled INTEGER NOT NULL, events_id_fk INTEGER NOT NULL, furnitures_id_fk INTEGER NOT NULL, FOREIGN KEY(events_id_fk) REFERENCES events(id), FOREIGN KEY(furnitures_id_fk) REFERENCES furnitures(id), UNIQUE(furnitures_id_fk, events_id_fk) ON CONFLICT REPLACE);
	CREATE TABLE IF NOT EXISTS customers (id INTEGER NOT NULL PRIMARY KEY, email TEXT NOT NULL UNIQUE, passwd TEXT NOT NULL, name TEXT, surname TEXT, address TEXT, notes TEXT);
	CREATE TABLE IF NOT EXISTS events (id INTEGER NOT NULL PRIMARY KEY, name TEXT NOT NULL, date INTEGER NOT NULL, from_date INTEGER NOT NULL, to_date INTEGER NOT NULL, default_price INTEGER NOT NULL, default_currency TEXT NOT NULL, how_to TEXT NOT NULL, ordered_note TEXT NOT NULL, mail_subject INTEGER NOT NULL, mail_text INTEGER NOT NULL, users_id_fk INTEGER NOT NULL, FOREIGN KEY(users_id_fk) REFERENCES users(id), UNIQUE(name, users_id_fk) ON CONFLICT ROLLBACK);
	CREATE TABLE IF NOT EXISTS events_rooms (events_id_fk INTEGER NOT NULL, rooms_id_fk INTEGER NOT NULL, FOREIGN KEY(events_id_fk) REFERENCES events(id), FOREIGN KEY(rooms_id_fk) REFERENCES rooms(id), UNIQUE(events_id_fk, rooms_id_fk) ON CONFLICT ROLLBACK);
	CREATE TABLE IF NOT EXISTS reservations (id INTEGER NOT NULL PRIMARY KEY, ordered_date INTEGER, payed_date INTEGER, price INTEGER, currency TEXT, status TEXT NOT NULL, furnitures_id_fk INTEGER NOT NULL, events_id_fk INTEGER NOT NULL, customers_id_fk INTEGER NOT NULL, FOREIGN KEY(furnitures_id_fk) REFERENCES furnitures(id), FOREIGN KEY(events_id_fk) REFERENCES events(id), FOREIGN KEY(customers_id_fk) REFERENCES curstomers(id), UNIQUE(furnitures_id_fk, events_id_fk) ON CONFLICT ROLLBACK);
	`
	if _, err := db.DB.Exec(structure); err != nil {
		log.Fatalf("error creating db structure in %s: %v", db.FileName, err)
	}
}

func (db *DB) UserAdd(user *User) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO users (email, url, passwd, name, surname, organization) VALUES (:email, :url, :passwd, :name, :surname, :organization)`, user)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) UserGetByEmail(email string) (User, error) {
	user := User{}
	err := db.DB.Get(&user, `SELECT * FROM users WHERE email=$1`, email)
	return user, err
}

func (db *DB) UserGetByID(id int64) (User, error) {
	user := User{}
	err := db.DB.Get(&user, `SELECT * FROM users WHERE id=$1`, id)
	return user, err
}

func (db *DB) UserGetByURL(url string) (User, error) {
	user := User{}
	err := db.DB.Get(&user, `SELECT * FROM users WHERE url=$1`, url)
	return user, err
}

func (db *DB) UserGetAll() ([]User, error) {
	users := []User{}
	err := db.DB.Select(&users, `SELECT * FROM users`)
	return users, err
}

func (db *DB) UserMod(user *User) error {
	_, err := db.DB.NamedExec(`UPDATE users SET email=:email, url=:url, passwd=:passwd, name=:name, surname=:surname, organization=:organization WHERE id=:id`, user)
	return err
}

func (db *DB) UserModByEmail(user *User) error {
	_, err := db.DB.NamedExec(`UPDATE users SET url=:url, passwd=:passwd, name=:name, surname=:surname, organization=:organization WHERE email=:email`, user)
	return err
}

func (db *DB) UserDel(email string) error {
	ret, err := db.DB.Exec(`DELETE FROM users WHERE email=$1`, email)
	affected, err := ret.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("user %q not found", email)
	}
	return err
}

func (db *DB) RoomAdd(room *Room) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO rooms (name, description, width, height) VALUES (:name, :description, :width, :height)`, room)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) RoomGetByName(name string) (Room, error) {
	room := Room{}
	err := db.DB.Get(&room, `SELECT * FROM rooms WHERE name=$1`, name)
	return room, err
}

func (db *DB) RoomGetByID(id int64) (Room, error) {
	room := Room{}
	err := db.DB.Get(&room, `SELECT * FROM rooms WHERE id=$1`, id)
	return room, err
}

func (db *DB) RoomGetAllForEvent(event string) ([]Room, error) {
	rooms := []Room{}
	err := db.DB.Select(&rooms, `SELECT r.* FROM rooms r LEFT JOIN events_rooms er ON r.id = er.rooms_id_fk LEFT JOIN events e ON er.events_id_fk = e.id WHERE e.name = $1`, event)
	return rooms, err
}

func (db *DB) RoomGetAllForEventID(eventID int64) ([]Room, error) {
	rooms := []Room{}
	err := db.DB.Select(&rooms, `SELECT r.* FROM rooms r LEFT JOIN events_rooms er ON r.id = er.rooms_id_fk WHERE er.events_id_fk = $1`, eventID)
	return rooms, err
}

func (db *DB) RoomGetAll() ([]Room, error) {
	rooms := []Room{}
	err := db.DB.Select(&rooms, `SELECT * FROM rooms`)
	return rooms, err
}

func (db *DB) RoomMod(room *Room) error {
	_, err := db.DB.NamedExec(`UPDATE rooms SET name=:name, width=:width, height=:height WHERE id=:id`, room)
	return err
}

func (db *DB) RoomModSizeByName(room *Room) error {
	_, err := db.DB.NamedExec(`UPDATE rooms SET width=:width, height=:height WHERE name=:name`, room)
	return err
}

func (db *DB) RoomDel(id int64) error {
	ret, err := db.DB.Exec(`DELETE FROM rooms WHERE id=$1`, id)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("room with ID: %d not found", id)
	}
	return err
}

// rooms_id_fk is uniqu, so only one owner of room can exist
func (db *DB) RoomAssignToUser(userID, roomID int64) error {
	err := NoMinus("userID", userID)
	if err != nil {
		return err
	}
	err = NoMinus("roomID", roomID)
	if err != nil {
		return err
	}
	ret, err := db.DB.Exec(`INSERT INTO users_rooms (users_id_fk, rooms_id_fk) VALUES ($1, $2)`, userID, roomID)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("no users_rooms entry added for userID: %v, roomID: %v", userID, roomID)
	}
	return err
}

func (db *DB) RoomUnassignUser(userID, roomID int64) error {
	ret, err := db.DB.Exec(`DELETE FROM users_rooms WHERE users_id_fk=$1 AND rooms_id_fk=$2`, userID, roomID)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("no users_rooms entry removed for userID: %v, roomID: %v", userID, roomID)
	}
	return err
}

func (db *DB) FurnitureAdd(f *Furniture) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO furnitures (number, type, orientation, x, y, width, height, color, label, capacity, rooms_id_fk) 
VALUES(:number, :type, :orientation, :x, :y, :width, :height, :color, :label, :capacity, :rooms_id_fk)`, f)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) FurnitureAddOrUpdateUnsafe(f *Furniture) (int64, error) {
	log.Println("Do NOT use FurnitureAddOrUpdate - messes with ID's")
	ret, err := db.DB.NamedExec(`INSERT OR REPLACE INTO furnitures (number, type, orientation, x, y, width, height, color, label, capacity, rooms_id_fk) 
VALUES(:number, :type, :orientation, :x, :y, :width, :height, :color, :label, :capacity, :rooms_id_fk)`, f)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) FurnitureGetByTypeNumberRoom(ftype string, number int64, roomID int64) (Furniture, error) {
	furniture := Furniture{}
	err := db.DB.Get(&furniture, `SELECT * FROM furnitures WHERE type=$1 AND number=$2 AND rooms_id_fk=$3`, ftype, number, roomID)
	return furniture, err
}

func (db *DB) FurnitureGetByID(id int64) (Furniture, error) {
	furniture := Furniture{}
	err := db.DB.Get(&furniture, `SELECT * FROM furnitures WHERE id=$1`, id)
	return furniture, err
}

func (db *DB) FurnitureGetAllByRoomID(roomID int64) ([]Furniture, error) {
	furnitures := []Furniture{}
	err := db.DB.Select(&furnitures, `SELECT * FROM furnitures WHERE rooms_id_fk=$1 ORDER BY number`, roomID)
	return furnitures, err
}

func (db *DB) FurnitureGetAllByRoomIDOfType(roomID int64, ftype string) ([]Furniture, error) {
	furnitures := []Furniture{}
	err := db.DB.Select(&furnitures, `SELECT * FROM furnitures WHERE type=$1 AND rooms_id_fk=$2 ORDER BY number`, ftype, roomID)
	return furnitures, err
}

func (db *DB) FurnitureGetAllByRoomName(name string) ([]Furniture, error) {
	furnitures := []Furniture{}
	err := db.DB.Select(&furnitures, `SELECT * FROM furnitures WHERE rooms_id_fk=(SELECT id FROM rooms WHERE name=$1) ORDER BY number`, name)
	return furnitures, err
}

func (db *DB) FurnitureGetAllByRoomNameOfType(name, ftype string) ([]Furniture, error) {
	furnitures := []Furniture{}
	err := db.DB.Select(&furnitures, `SELECT * FROM furnitures WHERE type=$1 AND rooms_id_fk=(SELECT id FROM rooms WHERE name=$2) ORDER BY number`, ftype, name)
	return furnitures, err
}

func (db *DB) FurnitureFullGetAllByEventRoom(eventID int64, name string) ([]FurnitureFull, error) {
	furnitures := []FurnitureFull{}
	err := db.DB.Select(&furnitures, `SELECT f.*, p.price admin_price,p.currency admin_currency, p.disabled admin_disabled, r.price bought_price, r.status status, r.ordered_date ordered_date, r.payed_date payed_date, r.customers_id_fk reservation_customers_id_fk FROM furnitures f LEFT JOIN prices p ON f.id = p.furnitures_id_fk AND p.events_id_fk=$1 LEFT JOIN reservations r ON f.id = r.furnitures_id_fk AND r.events_id_fk=$1 WHERE f.rooms_id_fk=(SELECT id FROM rooms WHERE name=$2) ORDER BY f.type, f.number`, eventID, name)
	return furnitures, err
}

func (db *DB) FurnitureFullGetChairs(eventID int64, name string) ([]FurnitureFull, error) {
	furnitures := []FurnitureFull{}
	err := db.DB.Select(&furnitures, `SELECT f.*, p.price admin_price,p.currency admin_currency, p.disabled admin_disabled, r.price bought_price, r.status status, r.ordered_date ordered_date, r.payed_date payed_date, r.customers_id_fk reservation_customers_id_fk FROM furnitures f LEFT JOIN prices p ON f.id = p.furnitures_id_fk AND p.events_id_fk=$1 LEFT JOIN reservations r ON f.id = r.furnitures_id_fk AND r.events_id_fk=$1 WHERE f.type="chair" AND f.rooms_id_fk=(SELECT id FROM rooms WHERE name=$2) ORDER BY f.number`, eventID, name)
	return furnitures, err
}

func (db *DB) FurnitureGetAll() ([]Furniture, error) {
	furnitures := []Furniture{}
	err := db.DB.Select(&furnitures, `SELECT * FROM furnitures ORDER BY number`)
	return furnitures, err
}

func (db *DB) FurnitureMod(furniture *Furniture) error {
	if furniture.ID == 0 {
		return fmt.Errorf("FurnitureMod: cannot update when ID is 0")
	}
	_, err := db.DB.NamedExec(`UPDATE furnitures SET number=:number, type=:type, orientation=:orientation,
		x=:x, y=:y, width=:width, height=:height, color=:color, label=:label, capacity=:capacity WHERE id=:id`, furniture)
	return err
}

func (db *DB) FurnitureModByNumberTypeRoom(furniture *Furniture) error {
	_, err := db.DB.NamedExec(`UPDATE furnitures SET orientation=:orientation, x=:x, y=:y, width=:width, height=:height, color=:color, label=:label, capacity=:capacity WHERE number=:number AND type=:type AND rooms_id_fk=:rooms_id_fk`, furniture)
	return err
}

func (db *DB) FurnitureChangeRoomByName(number int64, ftype string, roomName string) error {
	_, err := db.DB.Exec(`UPDATE furnitures SET rooms_id_fk=(SELECT id FROM rooms WHERE name=$1) WHERE number=$2 AND type=$3`, roomName, number, ftype)
	return err
}

func (db *DB) FurnitureDel(id int64) error {
	ret, err := db.DB.Exec(`DELETE FROM furnitures WHERE id=$1`, id)
	affected, err := ret.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("furniture with ID: %d not found", id)
	}
	return err
}

func (db *DB) FurnitureDelByNumberType(number int64, ftype string) error {
	ret, err := db.DB.Exec(`DELETE FROM furnitures WHERE number=$1 AND type=$2`, number, ftype)
	affected, err := ret.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("furniture with number and type: %q and %q not found", number, ftype)
	}
	return err
}

func (db *DB) PriceAdd(p *Price) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO prices (price, currency, disabled, events_id_fk, furnitures_id_fk) 
VALUES(:price, :currency, :disabled, :events_id_fk, :furnitures_id_fk)`, p)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) PriceAddOrUpdateUnsafe(p *Price) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT OR REPLACE INTO prices (price, currency, disabled, events_id_fk, furnitures_id_fk) 
VALUES(:price, :currency, :disabled, :events_id_fk, :furnitures_id_fk)`, p)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) PriceGetByEventName(fID int64, event string) (Price, error) {
	price := Price{}
	err := db.DB.Get(&price, `SELECT * FROM prices WHERE furnitures_id_fk=$1 
								and events_id_fk = (SELECT id FROM events WHERE name=$2)`, fID, event)
	return price, err
}

func (db *DB) PriceGetByID(id int64) (Price, error) {
	price := Price{}
	err := db.DB.Get(&price, `SELECT * FROM prices WHERE id=$1`, id)
	return price, err
}

func (db *DB) PriceMod(price *Price) error {
	if price.ID == 0 {
		return fmt.Errorf("can not modify price if ID is 0")
	}

	_, err := db.DB.NamedExec(`UPDATE prices SET price=:price, currency=:currency, disabled=:disabled WHERE id=:id`, price)
	return err
}

func (db *DB) PriceModByEventIDFurnID(price *Price) error {
	if price.EventID == 0 || price.FurnitureID == 0 {
		return fmt.Errorf("can not modify price if EventID or FurnitureID is 0, %+v", price)
	}

	_, err := db.DB.NamedExec(`UPDATE prices SET price=:price, currency=:currency, disabled=:disabled WHERE events_id_fk=:events_id_fk and furnitures_id_fk=:furnitures_id_fk`, price)
	return err
}

func (db *DB) PriceDel(id int64) error {
	ret, err := db.DB.Exec(`DELETE FROM prices WHERE id=$1`, id)
	affected, err := ret.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("price with ID: %d not found", id)
	}
	return err
}

func (db *DB) PriceDelByEventIDFurn(eventID int64, fnumber int64, ftype string) error {
	ret, err := db.DB.Exec(`DELETE FROM prices WHERE events_id_fk=$1 AND number=$2 AND type=$3`, eventID, fnumber, ftype)
	affected, err := ret.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("price with number and type: %q and %q not found", fnumber, ftype)
	}
	return err
}

func (db *DB) PriceDelByEventFurn(event string, fnumber int64, ftype string) error {
	ret, err := db.DB.Exec(`DELETE FROM prices WHERE events_id_fk=(SELECT id FROM events where name=$1) AND number=$2 AND type=$3`, event, fnumber, ftype)
	affected, err := ret.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("price with number and type: %q and %q not found", fnumber, ftype)
	}
	return err
}

func (db *DB) EventAdd(e *Event) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO events (name, date, from_date, to_date, default_price, default_currency, ordered_note, how_to, mail_subject, mail_text, users_id_fk) 
VALUES(:name, :date, :from_date, :to_date, :default_price, :default_currency, :ordered_note, :how_to, :mail_subject, :mail_text, :users_id_fk)`, e)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

// EventAddOrUpdate will increase id every update!!!
func (db *DB) EventAddOrUpdateUnsafe(e *Event) (int64, error) {
	log.Println("EventAddOrUpdate: probably wrong idea to use this func!")
	ret, err := db.DB.NamedExec(`INSERT OR REPLACE INTO events (name, date, from_date, to_date, default_price, default_currency, ordered_note, how_to, mail_subject, mail_text, users_id_fk) 
VALUES(:name, :date, :from_date, :to_date, :default_price, :default_currency, :ordered_note, :how_to, :mail_subject, :mail_text, :users_id_fk)`, e)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) EventGetByName(eventName string) (Event, error) {
	event := Event{}
	err := db.DB.Get(&event, `SELECT * FROM events WHERE name=$1 `, eventName)
	return event, err
}

func (db *DB) EventGetByID(id int64) (Event, error) {
	event := Event{}
	err := db.DB.Get(&event, `SELECT * FROM events WHERE id=$1`, id)
	return event, err
}

func (db *DB) EventGetAllByUserID(userID int64) ([]Event, error) {
	events := []Event{}
	err := db.DB.Select(&events, `SELECT * FROM events WHERE users_id_fk = $1 ORDER BY name`, userID)
	return events, err
}

func (db *DB) EventGetAll() ([]Event, error) {
	events := []Event{}
	err := db.DB.Select(&events, `SELECT * FROM events ORDER BY name`)
	return events, err
}

func (db *DB) RoomAddToEvent(roomID int64, eventID int64) error {
	_, err := db.DB.Exec("INSERT OR REPLACE INTO events_rooms (events_id_fk, rooms_id_fk) VALUES ($1, $2)", eventID, roomID)
	return err
}

func (db *DB) EventAddRoom(eventID, roomID int64) error {
	err := NoMinus("eventID", eventID)
	if err != nil {
		return err
	}
	err = NoMinus("roomID", roomID)
	if err != nil {
		return err
	}
	ret, err := db.DB.Exec("INSERT INTO events_rooms (events_id_fk, rooms_id_fk) VALUES ($1, $2)", eventID, roomID)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("error adding room %d to event %d, err: %v", roomID, eventID, err)
	}
	return err
}

func (db *DB) EventGetRooms(eventID int64) ([]Room, error) {
	rooms := []Room{}
	err := db.DB.Select(&rooms, `SELECT r.* FROM rooms r LEFT JOIN events_rooms er ON r.id = er.rooms_id_fk WHERE er.events_id_fk = $1`, eventID)
	return rooms, err
}

func (db *DB) EventMod(event *Event) error {
	if event.ID == 0 {
		return fmt.Errorf("can not modify event if ID is 0")
	}
	_, err := db.DB.NamedExec(`UPDATE events SET name=:name, date=:date, from_date=:from_date, to_date=:to_date,default_price=:default_price, default_currency=:default_currency, ordered_note=:ordered_note, how_to=:how_to, mail_subject=:mail_subject, mail_text=:mail_text WHERE id=:id`, event)
	return err
}

func (db *DB) EventDel(id int64) error {
	ret, err := db.DB.Exec(`DELETE FROM events WHERE id=$1`, id)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("event with ID: %d not found", id)
	}
	return err
}

func (db *DB) EventDelByEventIDFurn(eventID int64, fnumber int64, ftype string) error {
	ret, err := db.DB.Exec(`DELETE FROM events WHERE events_id_fk=$1 AND number=$2 AND type=$3`, eventID, fnumber, ftype)
	affected, err := ret.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("event with number and type: %q and %q not found", fnumber, ftype)
	}
	return err
}

func (db *DB) EventDelByEventFurn(event string, fnumber int64, ftype string) error {
	ret, err := db.DB.Exec(`DELETE FROM events WHERE events_id_fk=(SELECT id FROM events where name=$1) AND number=$2 AND type=$3`, event, fnumber, ftype)
	affected, err := ret.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("event with number and type: %q and %q not found", fnumber, ftype)
	}
	return err
}

func (db *DB) ReservationGet(furnitureID, eventID int64) (Reservation, error) {
	r := Reservation{}
	err := db.DB.Get(&r, `SELECT * FROM reservations WHERE furnitures_id_fk=$1 AND events_id_fk=$2`, furnitureID, eventID)
	return r, err
}

func (db *DB) ReservationAdd(r *Reservation) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO reservations (ordered_date, payed_date, price, currency, status, furnitures_id_fk, events_id_fk, customers_id_fk) 
VALUES(:ordered_date, :payed_date, :price, :currency, :status, :furnitures_id_fk, :events_id_fk, :customers_id_fk)`, r)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) ReservationMod(r *Reservation) error {
	_, err := db.DB.NamedExec(`UPDATE reservations SET ordered_date=:ordered_date, payed_date=:payed_date, price=:price, currency=:currency, status=:status, customers_id_fk=:customers_id_fk WHERE furnitures_id_fk=:furnitures_id_fk AND events_id_fk=:events_id_fk`, r)
	return err
}

func (db *DB) Close() {
	db.DB.Close()
}

func NoMinus(n string, i int64) error {
	if i < 0 {
		return fmt.Errorf("wrong input param %q: %d", n, i)
	}
	return nil
}

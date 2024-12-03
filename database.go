package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	// Workaround bug:
	//"github.com/demiurgestudios/sqlx"
	//_ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite"
)

// QUERIES which may be helpful in the future:

// Add prices for given room:

// INSERT INTO prices (price, currency, disabled, events_id_fk, furnitures_id_fk)
// SELECT 650, "Kč", 0, 6, furnitures.id
// FROM furnitures
// WHERE furnitures.rooms_id_fk = 4

// Check which prices are defined:

// SELECT * FROM prices p
// JOIN furnitures f ON f.id = p.furnitures_id_fk
// WHERE
// p.events_id_fk = 6
// AND f.rooms_id_fk = 4

// Example of alter table:
// ALTER TABLE users
//   ADD COLUMN alt_email TEXT;

// ALTER TABLE events
//   ADD COLUMN mail_attachments TEXT;

//ALTER TABLE formtemplates
//   ADD COLUMN infopanel TEXT;

// ALTER TABLE formtemplates
//   ADD COLUMN bankaccounts_id_fk INTEGER REFERENCES bankaccounts(id);

//ALTER TABLE formtemplates
//   ADD COLUMN moneyfield TEXT;
//ALTER TABLE formtemplates
//   ADD COLUMN thankyoumailsubject TEXT;
//ALTER TABLE formtemplates
//   ADD COLUMN thankyoumailtext TEXT;

//ALTER TABLE events
//   ADD COLUMN mail_embeded_imgs TEXT;

//ALTER TABLE forms
//   ADD COLUMN status TEXT;

//ALTER TABLE formtemplates
//   ADD COLUMN notifications_id_fk TEXT;

// Mods for notifications integration

// ALTER TABLE events
//   ADD COLUMN thankyou_notifications_id_fk INTEGER REFERENCES notifications(id);

// ALTER TABLE events
//   ADD COLUMN admin_notifications_id_fk INTEGER REFERENCES notifications(id);

// ALTER TABLE events
//   ADD COLUMN "bankaccounts_id_fk" INTEGER REFERENCES bankaccounts(id);

//ALTER TABLE rooms
//	ADD COLUMN sharable INTEGER;

//ALTER TABLE events
//	ADD COLUMN sharable INTEGER;

// copy customers mail to notifications table
// INSERT INTO notifications (name, type, related_to, title, text, embedded_imgs, attached_imgs, sharable, created_date, users_id_fk)
// SELECT name, 'mail', 'events', mail_subject, mail_text, mail_embeded_imgs, mail_attachments, 0, 1711746719, users_id_fk
// FROM events

// copy admin mail to notifications table
// INSERT INTO notifications (name, type, related_to, title, text, embedded_imgs, attached_imgs, sharable, created_date, users_id_fk)
//  SELECT name||' (admin)', 'mail', 'events', admin_mail_subject, admin_mail_text, mail_embeded_imgs, mail_attachments, 0, 1711746719, users_id_fk
// FROM events

// link mails to events
// UPDATE events SET
// thankyou_notifications_id_fk=(SELECT id FROM notifications WHERE events.name = notifications.name),
// admin_notifications_id_fk=(SELECT id FROM notifications WHERE events.name||' (admin)' = notifications.name)

// remove mails from events table
// ALTER TABLE events DROP COLUMN mail_subject;
// ALTER TABLE events DROP COLUMN mail_text;
// ALTER TABLE events DROP COLUMN admin_mail_subject;
// ALTER TABLE events DROP COLUMN admin_mail_text;
// ALTER TABLE events DROP COLUMN mail_attachments;
// ALTER TABLE events DROP COLUMN mail_embeded_imgs;

// copy thankyou subject/text from formtemplates to notifications
// INSERT INTO notifications (name, type, related_to, title, text, embedded_imgs, attached_imgs, sharable, created_date, users_id_fk)
// SELECT name||' (form)', 'mail', 'forms', thankyoumailsubject, thankyoumailtext, '', '', 0, 1711746719, users_id_fk
// FROM formtemplates

// remove subject/text from formtemplates
//
//ALTER TABLE formtemplates DROP COLUMN thankyoumailsubject;
//ALTER TABLE formtemplates DROP COLUMN thankyoumailtext;
//
//
// END

// Get form data:
// SELECT display, value FROM formanswers
// JOIN formfields ON (formanswers.formfields_id_fk=formfields.id)
// WHERE forms_id_fk=4

// FormAnswers:
//SELECT f.name, f.surname, ff.display, fa.value FROM forms f
//JOIN formanswers fa ON fa.forms_id_fk=f.id
//JOIN formfields ff ON fa.formfields_id_fk=ff.id
//WHERE ff.display != "Nazwisko"

//SELECT display, SUM(fa.value) FROM forms f
//JOIN formanswers fa ON fa.forms_id_fk=f.id
//JOIN formfields ff ON fa.formfields_id_fk=ff.id
//WHERE ff.display != "Nazwisko" AND ff.display != "E-mail" AND display != "Imię dziecka"
//AND display != "Klasa/przedszkole"
//GROUP BY display

// Renumbering tables:
// UPDATE furnitures
// SET number = number + 20
// WHERE rooms_id_fk = 4 AND type = "table"

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
	AltEmail     sql.NullString `db:"alt_email"`
}

type Admin struct {
	ID     int64          `db:"id"`
	Type   string         `db:"type"`
	Email  string         `db:"email"`
	Passwd sql.NullString `db:"passwd"`
	Notes  sql.NullString `db:"notes"`
	UserID int64          `db:"users_id_fk"`
}

type Customer struct {
	ID      int64          `db:"id"`
	Email   string         `db:"email"`
	Passwd  sql.NullString `db:"passwd"`
	Name    sql.NullString `db:"name"`
	Surname sql.NullString `db:"surname"`
	Phone   sql.NullString `db:"phone"`
	Notes   string         `db:"notes"`
}

// TODO: add build name
// TODO: remake all queries - make room name non unique(beware! currently impossible, queries depends on name unique)
type Room struct {
	ID          int64          `db:"id"`
	Name        string         `db:"name"`
	Banner      sql.NullString `db:"banner_img"`
	Description sql.NullString `db:"description"`
	Width       int64          `db:"width"`
	Height      int64          `db:"height"`
	Sharable    sql.NullInt64  `db:"sharable"`
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
	ID                      int64         `db:"id"`
	Name                    string        `db:"name"`
	Date                    int64         `db:"date"`
	FromDate                int64         `db:"from_date"`
	ToDate                  int64         `db:"to_date"`
	DefaultPrice            int64         `db:"default_price"`
	DefaultCurrency         string        `db:"default_currency"`
	NoSitsSelectedTitle     string        `db:"no_sits_selected_title"`
	NoSitsSelectedText      string        `db:"no_sits_selected_text"`
	OrderHowto              string        `db:"order_howto"`
	OrderNotesDescription   string        `db:"order_notes_desc"`
	OrderedNoteTitle        string        `db:"ordered_note_title"`
	OrderedNoteText         string        `db:"ordered_note_text"`
	HowTo                   string        `db:"how_to"`
	UserID                  int64         `db:"users_id_fk"`
	ThankYouNotificationsID sql.NullInt64 `db:"thankyou_notifications_id_fk"`
	AdminNotificationsID    sql.NullInt64 `db:"admin_notifications_id_fk"`
	Sharable                sql.NullBool  `db:"sharable"`
	BankAccountsID          sql.NullInt64 `db:"bankaccounts_id_fk"`
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
	Status      string        `db:"status"`
	NoteID      sql.NullInt64 `db:"notes_id_fk"`
	FurnitureID int64         `db:"furnitures_id_fk"`
	EventID     int64         `db:"events_id_fk"`
	CustomerID  int64         `db:"customers_id_fk"`
}

type ReservationFull struct {
	Reservation
	CustEmail   sql.NullString `db:"cust_email"`
	CustName    sql.NullString `db:"cust_name"`
	CustSurname sql.NullString `db:"cust_surname"`
	CustPhone   sql.NullString `db:"cust_phone"`
	ChairNumber int64          `db:"chair_number"`
	RoomName    string         `db:"room_name"`
	Notes       sql.NullString `db:"reservation_notes"`
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

type FormTemplate struct {
	ID                   int64          `db:"id"`
	Name                 string         `db:"name"`
	URL                  string         `db:"url"`
	HowTo                sql.NullString `db:"howto"`
	Banner               sql.NullString `db:"banner"`
	InfoPanel            sql.NullString `db:"infopanel"`
	ThankYou             sql.NullString `db:"thankyou"`
	Content              sql.NullString `db:"content"`
	CreatedDate          int64          `db:"created_date"`
	MoneyAmountFieldName sql.NullString `db:"moneyfield"`
	UserID               int64          `db:"users_id_fk"`
	EventID              sql.NullInt64  `db:"events_id_fk"`
	BankAccountID        sql.NullInt64  `db:"bankaccounts_id_fk"`
	//ThankYouMailSubject  sql.NullString `db:"thankyoumailsubject"` // not used anymore
	//ThankYouMailText     sql.NullString `db:"thankyoumailtext"`    // not used anymore
	NotificationID      sql.NullInt64 `db:"notifications_id_fk"`
	AdminNotificationID sql.NullInt64 `db:"admin_notifications_id_fk"`
}

type FormField struct {
	ID             int64  `db:"id"`
	Name           string `db:"name"`
	Display        string `db:"display"`
	Type           string `db:"type"`
	FormTemplateID int64  `db:"formtemplates_id_fk"`
}

type Form struct {
	ID             int64          `db:"id"`
	Name           sql.NullString `db:"name"`
	Surname        sql.NullString `db:"surname"`
	Email          sql.NullString `db:"email"`
	Notes          sql.NullString `db:"notes"`
	Status         sql.NullString `db:"status"`
	CreatedDate    int64          `db:"created_date"`
	UserID         int64          `db:"users_id_fk"`
	FormTemplateID int64          `db:"formtemplates_id_fk"`
}

type FormAnswer struct {
	ID          int64          `db:"id"`
	Value       sql.NullString `db:"value"`
	FormFieldID int64          `db:"formfields_id_fk"`
	FormID      int64          `db:"forms_id_fk"`
}

type BankAccount struct {
	ID            int64          `db:"id"`
	Name          string         `db:"name"`
	IBAN          string         `db:"iban"`
	RecipientName sql.NullString `db:"recipientname"`
	BankID        sql.NullString `db:"bank"`
	Currency      string         `db:"currency"`
	Message       sql.NullString `db:"message"`
	VarSymbol     sql.NullInt64  `db:"varsymbol"`
	AmountField   sql.NullString `db:"amountfield"`
	UserID        int64          `db:"users_id_fk"`
}

type Notification struct {
	ID                     int64          `db:"id"`
	Name                   string         `db:"name"`
	Type                   string         `db:"type"`
	RelatedTo              string         `db:"related_to"`
	Title                  sql.NullString `db:"title"`
	Text                   string         `db:"text"`
	EmbeddedImgsDelimited  sql.NullString `db:"embedded_imgs"`
	AttachedFilesDelimited sql.NullString `db:"attached_imgs"`
	Sharable               bool           `db:"sharable"`
	CreatedDate            int64          `db:"created_date"`
	UpdatedDate            sql.NullInt64  `db:"updated_date"`
	UserID                 int64          `db:"users_id_fk"`
}

type FormNotificationLog struct {
	ID             int64 `db:"id"`
	Date           int64 `db:"date"`
	NotificationID int64 `db:"notifications_id_fk"`
	FormID         int64 `db:"forms_id_fk"`
}

type EventAddon struct {
	ID       int64          `db:"id"`
	Name     string         `db:"name"`
	Price    sql.NullInt64  `db:"price"`
	Currency sql.NullString `db:"currency"`
	EventID  int64          `db:"events_id_fk"`
	UserID   int64          `db:"users_id_fk"`
}

// GetType is needed for generics
func (f FormField) GetType() string {
	return f.Type
}
func (f FormField) GetName() string {
	return f.Name
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
	db.DB = sqlx.MustConnect("sqlite", db.FileName+ConnOptions)
	db.DB.SetMaxOpenConns(1)
}

func (db *DB) StructureCreate() {
	structure := `
	CREATE TABLE IF NOT EXISTS users (id INTEGER NOT NULL PRIMARY KEY, email TEXT NOT NULL UNIQUE, url TEXT NOT NULL, passwd TEXT NOT NULL, name TEXT, surname TEXT, organization TEXT, phone TEXT);
	CREATE TABLE IF NOT EXISTS admins (id INTEGER NOT NULL PRIMARY KEY, type TEXT NOT NULL, email TEXT NOT NULL, passwd TEXT, notes TEXT, users_id_fk INTEGER NOT NULL, FOREIGN KEY(users_id_fk) REFERENCES users(id));
	CREATE TABLE IF NOT EXISTS rooms (id INTEGER NOT NULL PRIMARY KEY, name TEXT NOT NULL UNIQUE, banner_img TEXT, description TEXT, width INTEGER NOT NULL, height INTEGER NOT NULL);
	CREATE TABLE IF NOT EXISTS users_rooms (users_id_fk INTEGER NOT NULL, rooms_id_fk INTEGER NOT NULL UNIQUE, FOREIGN KEY(users_id_fk) REFERENCES users(id), FOREIGN KEY(rooms_id_fk) REFERENCES rooms(id));
	CREATE TABLE IF NOT EXISTS furnitures (id INTEGER NOT NULL PRIMARY KEY, number INTEGER NOT NULL, type TEXT NOT NULL, orientation TEXT, x INTEGER NOT NULL, y INTEGER NOT NULL, width INTEGER, height INTEGER, color TEXT, label TEXT, capacity INTEGER, rooms_id_fk INTEGER NOT NULL, UNIQUE(number, type, rooms_id_fk) ON CONFLICT ROLLBACK, FOREIGN KEY(rooms_id_fk) REFERENCES rooms(id));
	CREATE TABLE IF NOT EXISTS prices (id INTEGER NOT NULL PRIMARY KEY, price INTEGER NOT NULL, currency TEXT NOT NULL, disabled INTEGER NOT NULL, events_id_fk INTEGER NOT NULL, furnitures_id_fk INTEGER NOT NULL, FOREIGN KEY(events_id_fk) REFERENCES events(id), FOREIGN KEY(furnitures_id_fk) REFERENCES furnitures(id), UNIQUE(furnitures_id_fk, events_id_fk) ON CONFLICT ROLLBACK);
	CREATE TABLE IF NOT EXISTS customers (id INTEGER NOT NULL PRIMARY KEY, email TEXT NOT NULL UNIQUE, passwd TEXT NOT NULL, name TEXT, surname TEXT, phone TEXT, notes TEXT);
	CREATE TABLE IF NOT EXISTS users_customers (users_id_fk INTEGER NOT NULL, customers_id_fk INTEGER NOT NULL, FOREIGN KEY(users_id_fk) REFERENCES users(id), FOREIGN KEY(customers_id_fk) REFERENCES customers(id), UNIQUE(users_id_fk, customers_id_fk) ON CONFLICT ROLLBACK);
	CREATE TABLE IF NOT EXISTS events (id INTEGER NOT NULL PRIMARY KEY, name TEXT NOT NULL, date INTEGER NOT NULL, from_date INTEGER NOT NULL, to_date INTEGER NOT NULL, default_price INTEGER NOT NULL, default_currency TEXT NOT NULL, no_sits_selected_title TEXT NOT NULL, no_sits_selected_text TEXT NOT NULL, how_to TEXT NOT NULL, order_howto TEXT NOT NULL, order_notes_desc TEXT NOT NULL, ordered_note_title TEXT NOT NULL, ordered_note_text TEXT NOT NULL, thankyou_notifications_id_fk INTEGER REFERENCES notifications(id), admin_notifications_id_fk INTEGER REFERENCES notifications(id),users_id_fk INTEGER NOT NULL, FOREIGN KEY(users_id_fk) REFERENCES users(id), UNIQUE(name, users_id_fk) ON CONFLICT ROLLBACK);
	CREATE TABLE IF NOT EXISTS events_rooms (events_id_fk INTEGER NOT NULL, rooms_id_fk INTEGER NOT NULL, FOREIGN KEY(events_id_fk) REFERENCES events(id), FOREIGN KEY(rooms_id_fk) REFERENCES rooms(id), UNIQUE(events_id_fk, rooms_id_fk) ON CONFLICT ROLLBACK);
	CREATE TABLE IF NOT EXISTS reservations (id INTEGER NOT NULL PRIMARY KEY, ordered_date INTEGER, payed_date INTEGER, price INTEGER, currency TEXT, status TEXT NOT NULL, notes_id_fk INTEGER, furnitures_id_fk INTEGER NOT NULL, events_id_fk INTEGER NOT NULL, customers_id_fk INTEGER NOT NULL, FOREIGN KEY(notes_id_fk) REFERENCES notes(id), FOREIGN KEY(furnitures_id_fk) REFERENCES furnitures(id), FOREIGN KEY(events_id_fk) REFERENCES events(id), FOREIGN KEY(customers_id_fk) REFERENCES customers(id), UNIQUE(furnitures_id_fk, events_id_fk) ON CONFLICT ROLLBACK);
	CREATE TABLE IF NOT EXISTS notes (id INTEGER NOT NULL PRIMARY KEY, text TEXT NOT NULL);
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

func (db *DB) UserGetPass(email string) (string, error) {
	passwd := ""
	err := db.DB.Get(&passwd, `SELECT passwd FROM users WHERE email=$1`, email)
	return passwd, err
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
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("user %q not found", email)
	}
	return err
}

func (db *DB) RoomAdd(room *Room) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO rooms (name, banner_img, description, width, height, sharable) VALUES (:name, :banner_img, :description, :width, :height, :sharable)`, room)
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

func (db *DB) RoomEventAdd(eventID, roomID int64) error {
	_, err := db.DB.NamedExec(`INSERT INTO events_rooms (events_id_fk, rooms_id_fk) VALUES (:events_id_fk, :rooms_id_fk)`, map[string]interface{}{"events_id_fk": eventID, "rooms_id_fk": roomID})
	return err
}

func (db *DB) RoomGetAll() ([]Room, error) {
	rooms := []Room{}
	err := db.DB.Select(&rooms, `SELECT * FROM rooms`)
	return rooms, err
}

func (db *DB) RoomGetAllByUserID(userID int64) ([]Room, error) {
	rooms := []Room{}
	err := db.DB.Select(&rooms, `SELECT r.* FROM rooms r LEFT JOIN users_rooms ur ON r.id = ur.rooms_id_fk WHERE ur.users_id_fk = $1`, userID)
	return rooms, err
}

func (db *DB) RoomGetUsersAndSharedByUserID(userID int64) ([]Room, error) {
	rooms := []Room{}
	err := db.DB.Select(&rooms, `SELECT r.* FROM rooms r LEFT JOIN users_rooms ur ON r.id = ur.rooms_id_fk WHERE ur.users_id_fk = $1 OR r.`, userID)
	return rooms, err
}

// TODO: do we need banner_img here? probably separate function for setting banner
// would be better idea.
func (db *DB) RoomMod(room *Room) error {
	_, err := db.DB.NamedExec(`UPDATE rooms SET name=:name, width=:width, height=:height, sharable=:sharable WHERE id=:id`, room)
	return err
}

func (db *DB) RoomModSizeByName(room *Room) error {
	_, err := db.DB.NamedExec(`UPDATE rooms SET width=:width, height=:height WHERE name=:name`, room)
	return err
}

func (db *DB) RoomModSizeByID(room *Room) error {
	_, err := db.DB.NamedExec(`UPDATE rooms SET width=:width, height=:height WHERE id=:id`, room)
	return err
}

func (db *DB) RoomDel(id int64) error {
	ret, err := db.DB.Exec(`DELETE FROM rooms WHERE id=$1`, id)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
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
	if err != nil {
		return err
	}
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
	if err != nil {
		return err
	}
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

func (db *DB) FurnitureCopyRoom(fromRoomID int64, toRoomID int64) (int64, error) {
	ret, err := db.DB.Exec(`INSERT INTO furnitures (number, type, orientation, x, y, width, height, color, label, capacity, rooms_id_fk) SELECT number, type, orientation, x, y, width, height, color, label, capacity, $1 FROM furnitures WHERE rooms_id_fk = $2`, toRoomID, fromRoomID)
	if err != nil {
		return -1, err
	}
	rows, err := ret.RowsAffected()
	if err != nil {
		return -1, err
	}
	return rows, nil
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
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("furniture with ID: %d not found", id)
	}
	return err
}

func (db *DB) FurnitureDelByNumberTypeRoom(number int64, ftype string, roomID int64) error {
	ret, err := db.DB.Exec(`DELETE FROM furnitures WHERE number=$1 AND type=$2 AND rooms_id_fk=$3`, number, ftype, roomID)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("furniture with number: %d and type: %q not found", number, ftype)
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
	err := NoMinus("eventID", price.EventID)
	if err != nil {
		return err
	}
	err = NoMinus("furnitureID", price.FurnitureID)
	if err != nil {
		return err
	}

	_, err = db.DB.NamedExec(`UPDATE prices SET price=:price, currency=:currency, disabled=:disabled WHERE events_id_fk=:events_id_fk and furnitures_id_fk=:furnitures_id_fk`, price)
	return err
}

func (db *DB) PriceDel(id int64) error {
	ret, err := db.DB.Exec(`DELETE FROM prices WHERE id=$1`, id)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("price with ID: %d not found", id)
	}
	return err
}

func (db *DB) PriceDelByEventIDFurn(eventID int64, fnumber int64, ftype string) error {
	ret, err := db.DB.Exec(`DELETE FROM prices WHERE events_id_fk=$1 AND number=$2 AND type=$3`, eventID, fnumber, ftype)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("price with number and type: %q and %q not found", fnumber, ftype)
	}
	return err
}

func (db *DB) PriceDelByEventFurn(event string, fnumber int64, ftype string) error {
	ret, err := db.DB.Exec(`DELETE FROM prices WHERE events_id_fk=(SELECT id FROM events where name=$1) AND number=$2 AND type=$3`, event, fnumber, ftype)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("price with number and type: %q and %q not found", fnumber, ftype)
	}
	return err
}

func (db *DB) EventAdd(e *Event) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO events (name, date, from_date, to_date, default_price, default_currency, no_sits_selected_title, no_sits_selected_text, order_howto, order_notes_desc, ordered_note_title, ordered_note_text, how_to, users_id_fk, thankyou_notifications_id_fk, admin_notifications_id_fk, sharable, bankaccounts_id_fk) 
	VALUES(:name, :date, :from_date, :to_date, :default_price, :default_currency, :no_sits_selected_title, :no_sits_selected_text, :order_howto, :order_notes_desc, :ordered_note_title, :ordered_note_text, :how_to, :users_id_fk, :thankyou_notifications_id_fk, :admin_notifications_id_fk, :sharable, :bankaccounts_id_fk)`, e)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

// EventAddOrUpdate will increase id every update!!!
//func (db *DB) EventAddOrUpdateUnsafe(e *Event) (int64, error) {
//	log.Println("EventAddOrUpdate: probably wrong idea to use this func!")
//	ret, err := db.DB.NamedExec(`INSERT OR REPLACE INTO events (name, date, from_date, to_date, default_price, default_currency, ordered_note, how_to, users_id_fk)
//VALUES(:name, :date, :from_date, :to_date, :default_price, :default_currency, :ordered_note, :how_to, :users_id_fk)`, e)
//	if err != nil {
//		return -1, err
//	}
//	return ret.LastInsertId()
//}

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

func (db *DB) RoomAddToEventUnsafe(roomID int64, eventID int64) error {
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
	if err != nil {
		return err
	}
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

func (db *DB) EventModByID(event *Event, UserID int64) error {
	if event.ID == 0 {
		return fmt.Errorf("can not modify event if ID is 0")
	}
	if event.UserID != UserID {
		return fmt.Errorf("event.UserID %d do not match UserID %d, can't update event", event.UserID, UserID)
	}
	_, err := db.DB.NamedExec(`UPDATE events SET name=:name, date=:date, from_date=:from_date, to_date=:to_date, default_price=:default_price, default_currency=:default_currency, no_sits_selected_title=:no_sits_selected_title, no_sits_selected_text=:no_sits_selected_text, order_howto=:order_howto, order_notes_desc=:order_notes_desc, ordered_note_title=:ordered_note_title, ordered_note_text=:ordered_note_text, how_to=:how_to, thankyou_notifications_id_fk=:thankyou_notifications_id_fk, admin_notifications_id_fk=:admin_notifications_id_fk, sharable=:sharable, bankaccounts_id_fk=:bankaccounts_id_fk WHERE id=:id AND users_id_fk=:users_id_fk`, event)
	return err
}

func (db *DB) EventDel(id int64) error {
	ret, err := db.DB.Exec(`DELETE FROM events WHERE id=$1`, id)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("event with ID: %d not found", id)
	}
	return err
}

func (db *DB) EventDelByEventIDFurn(eventID int64, fnumber int64, ftype string) error {
	ret, err := db.DB.Exec(`DELETE FROM events WHERE events_id_fk=$1 AND number=$2 AND type=$3`, eventID, fnumber, ftype)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("event with number and type: %q and %q not found", fnumber, ftype)
	}
	return err
}

func (db *DB) EventDelByEventFurn(event string, fnumber int64, ftype string) error {
	ret, err := db.DB.Exec(`DELETE FROM events WHERE events_id_fk=(SELECT id FROM events where name=$1) AND number=$2 AND type=$3`, event, fnumber, ftype)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
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

func (db *DB) ReservationGetAllInStatus(status string) ([]Reservation, error) {
	reservations := []Reservation{}
	err := db.DB.Select(&reservations, `SELECT * FROM reservations WHERE status=$1`, status)
	return reservations, err
}

func (db *DB) ReservationGetAllByNoteID(noteID int64) ([]Reservation, error) {
	reservations := []Reservation{}
	err := db.DB.Select(&reservations, `SELECT * FROM reservations WHERE notes_id_fk=$1`, noteID)
	return reservations, err
}

func (db *DB) ReservationFullGetAll(userID, eventID int64) ([]ReservationFull, error) {
	rr := []ReservationFull{}
	err := NoMinus("userID", userID)
	if err != nil {
		return rr, err
	}
	err = NoMinus("eventID", eventID)
	if err != nil {
		return rr, err
	}
	err = db.DB.Select(&rr, `SELECT r.*, c.email cust_email, c.name cust_name, c.surname cust_surname, c.phone cust_phone, f.number chair_number, rm.name room_name, n.text reservation_notes FROM reservations r LEFT JOIN furnitures f ON r.furnitures_id_fk = f.id LEFT JOIN rooms rm ON f.rooms_id_fk = rm.id LEFT JOIN customers c ON r.customers_id_fk = c.id LEFT JOIN events e ON r.events_id_fk = e.id LEFT JOIN notes n ON r.notes_id_fk = n.id WHERE e.users_id_fk = $1 AND r.events_id_fk=$2 ORDER BY f.number`, userID, eventID)
	return rr, err

}

func (db *DB) ReservationAdd(r *Reservation) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO reservations (ordered_date, payed_date, price, currency, status, notes_id_fk, furnitures_id_fk, events_id_fk, customers_id_fk) 
VALUES(:ordered_date, :payed_date, :price, :currency, :status, :notes_id_fk, :furnitures_id_fk, :events_id_fk, :customers_id_fk)`, r)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) ReservationMod(r *Reservation) error {
	_, err := db.DB.NamedExec(`UPDATE reservations SET ordered_date=:ordered_date, payed_date=:payed_date, price=:price, currency=:currency, status=:status, notes_id_fk=:notes_id_fk, customers_id_fk=:customers_id_fk WHERE furnitures_id_fk=:furnitures_id_fk AND events_id_fk=:events_id_fk`, r)
	return err
}

func (db *DB) ReservationModStatus(status string, payedDate int64, eventID, furnID int64) error {
	if err := NoMinus("eventID", eventID); err != nil {
		return fmt.Errorf("error: ReservationModStatus: %v", err)
	}
	if err := NoMinus("furnID", furnID); err != nil {
		return fmt.Errorf("error: ReservationModStatus: %v", err)
	}
	_, err := db.DB.Exec(`UPDATE reservations SET status=$1, payed_date=$2 WHERE events_id_fk=$3 AND furnitures_id_fk=$4`, status, payedDate, eventID, furnID)
	return err
}

// TODO: does affected check make any sense? there is error if no row affected IMO
func (db *DB) ReservationDel(id int64) error {
	ret, err := db.DB.Exec(`DELETE FROM reservations WHERE id=$1`, id)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("reservation with: %d", id)
	}
	return err
}

func (db *DB) CustomerAdd(c *Customer) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO customers (email, passwd, name, surname, phone, notes) 
VALUES(:email, :passwd, :name, :surname, :phone, :notes)`, c)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) CustomerGetByEmail(email string) (Customer, error) {
	c := Customer{}
	err := db.DB.Get(&c, `SELECT * FROM customers WHERE email=$1 ORDER BY id DESC LIMIT 1`, email)
	return c, err
}

func (db *DB) CustomerGetAll(userID int64) ([]Customer, error) {
	cc := []Customer{}

	err := NoMinus("userID", userID)
	if err != nil {
		return cc, err
	}
	err = db.DB.Select(&cc, `SELECT c.* FROM customers c LEFT JOIN users_customers uc ON c.id = uc.customers_id_fk WHERE uc.users_id_fk=$1`, userID)
	return cc, err
}

func (db *DB) CustomerAppendToUser(userID, customerID int64) error {
	err := NoMinus("userID", userID)
	if err != nil {
		return err
	}
	err = NoMinus("customerID", customerID)
	if err != nil {
		return err
	}
	ret, err := db.DB.Exec(`INSERT INTO users_customers (users_id_fk, customers_id_fk) 
VALUES($1, $2)`, userID, customerID)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("no users_customers entry added for userID: %v, customerID: %v", userID, customerID)
	}
	return err
}

func (db *DB) AdminAdd(a *Admin) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO admins (type, email, passwd, notes, users_id_fk) 
VALUES(:type, :email, :passwd, :notes, :users_id_fk)`, a)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) AdminGetAll(UserID int64) ([]Admin, error) {
	aa := []Admin{}
	err := db.DB.Select(&aa, `SELECT * FROM admins WHERE users_id_fk=$1`, UserID)
	return aa, err
}

// AdminGetEmails retrieves all mails, TODO: maybe filter by type?
func (db *DB) AdminGetEmails(UserID int64) ([]string, error) {
	aa := []string{}
	err := db.DB.Select(&aa, `SELECT email FROM admins WHERE users_id_fk=$1`, UserID)
	return aa, err
}

func (db *DB) NoteAdd(note string) (int64, error) {
	ret, err := db.DB.Exec(`INSERT INTO notes (text) 
VALUES($1)`, note)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) NoteDel(noteID int64) error {
	_, err := db.DB.Exec(`DELETE FROM notes WHERE id=$1`, noteID)
	return err
}

func (db *DB) FormTemplateAdd(t *FormTemplate) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO formtemplates (name, url, howto, banner, thankyou, infopanel, content, created_date, moneyfield, users_id_fk, events_id_fk, bankaccounts_id_fk, notifications_id_fk)
	VALUES(:name, :url, :howto, :banner, :thankyou, :infopanel, :content, :created_date, :moneyfield, :users_id_fk, :events_id_fk, :bankaccounts_id_fk, :notifications_id_fk)`, t)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) FormTemplateGetAll(UserID int64) ([]FormTemplate, error) {
	tt := []FormTemplate{}
	err := db.DB.Select(&tt, `SELECT * FROM formtemplates WHERE users_id_fk=$1`, UserID)
	return tt, err
}

func (db *DB) FormTemplateGetByID(tmplID, UserID int64) (FormTemplate, error) {
	t := FormTemplate{}
	err := db.DB.Get(&t, `SELECT * FROM formtemplates WHERE id=$1 AND users_id_fk=$2`, tmplID, UserID)
	return t, err
}

func (db *DB) FormTemplateGetByURL(URL string, UserID int64) (FormTemplate, error) {
	t := FormTemplate{}
	err := db.DB.Get(&t, `SELECT * FROM formtemplates WHERE url=$1 AND users_id_fk=$2`, URL, UserID)
	return t, err
}

func (db *DB) FormTemplateModByID(t *FormTemplate) error {
	_, err := db.DB.NamedExec(`UPDATE formtemplates SET name=:name, url=:url, howto=:howto, banner=:banner, thankyou=:thankyou, infopanel=:infopanel, content=:content, moneyfield=:moneyfield, bankaccounts_id_fk=:bankaccounts_id_fk, notifications_id_fk=:notifications_id_fk WHERE id=:id`, t)
	return err
}

func (db *DB) FormTemplateModByURL(t *FormTemplate) error {
	if t.UserID < 1 {
		return fmt.Errorf("FormTemplateModByURL: no userID given! Update aborted. %+v", t)
	}
	_, err := db.DB.NamedExec(`UPDATE formtemplates SET name=:name, content=:content, banner=:banner, howto=:howto, thankyou=:thankyou, infopanel=:infopanel, moneyfield=:moneyfield, bankaccounts_id_fk=:bankaccounts_id_fk, notifications_id_fk=:notifications_id_fk WHERE url=:url AND users_id_fk=:users_id_fk`, t)
	return err
}

func (db *DB) FormFieldAdd(f *FormField) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO formfields (name, display, type, formtemplates_id_fk)
	VALUES(:name, :display, :type, :formtemplates_id_fk)`, f)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) FormFieldGetAllForTmpl(formtemplateID int64) ([]FormField, error) {
	ff := []FormField{}
	err := db.DB.Select(&ff, `SELECT * FROM formfields WHERE formtemplates_id_fk=$1`, formtemplateID)
	return ff, err
}

// FormFieldGetByName - there is some strange problem with this function!
// TODO: It never finds any data, why?
func (db *DB) FormFieldGetByName(name string, formtemplateID int64) (FormField, error) {
	f := FormField{}
	err := db.DB.Get(&f, `SELECT * FROM formfields WHERE name=$1 AND formtemplates_id_fk=$1`, name, formtemplateID)
	return f, err
}

func (db *DB) FormFieldGetIDByName(name string, FormTemplateID int64) (int64, error) {
	var id int64
	if name == "" || FormTemplateID == 0 {
		return id, fmt.Errorf("FormFieldGetByName: empty name %q or formtemplates_id_fk %d", name, FormTemplateID)
	}
	err := db.DB.Get(&id, `SELECT id FROM formfields WHERE name=$1 AND formtemplates_id_fk=$2`, name, FormTemplateID)
	return id, err
}

func (db *DB) FormFieldGetIDByDisplay(display string, FormTemplateID int64) (int64, error) {
	var id int64
	if display == "" || FormTemplateID == 0 {
		return id, fmt.Errorf("FormFieldGetByDisplay: empty display name %q or formtemplates_id_fk %d", display, FormTemplateID)
	}
	err := db.DB.Get(&id, `SELECT id FROM formfields WHERE display=$1 AND formtemplates_id_fk=$2`, display, FormTemplateID)
	return id, err
}

// FormFieldModByName requires name and formtemplates_id_fk!
func (db *DB) FormFieldModByName(f *FormField) error {
	_, err := db.DB.NamedExec(`UPDATE formfields SET display=:display, type=:type WHERE name=:name AND formtemplates_id_fk=:formtemplates_id_fk`, f)
	return err
}

func (db *DB) FormFieldDelByName(name string, formtemplateID int64) error {
	ret, err := db.DB.Exec(`DELETE FROM formfields WHERE name=$1`, name)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("formfields: no rows deleted, tried to remove: %q", name)
	}
	return err
}

func (db *DB) FormAdd(f *Form) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO forms (name, surname, notes, email, created_date, status, users_id_fk, formtemplates_id_fk)
	VALUES(:name, :surname, :notes, :email, :created_date, :status, :users_id_fk, :formtemplates_id_fk)`, f)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) FormGetAll(UserID int64, FormTemplateID int64) ([]Form, error) {
	ff := []Form{}
	err := db.DB.Select(&ff, `SELECT * FROM forms WHERE users_id_fk=$1 AND formtemplates_id_fk=$2`, UserID, FormTemplateID)
	return ff, err
}

func (db *DB) FormGetAmmount(UserID, FormTemplateID int64) (int64, error) {
	var f int64
	err := db.DB.Get(&f, `SELECT COUNT(id) FROM forms WHERE users_id_fk=$1 AND formtemplates_id_fk=$2`, UserID, FormTemplateID)
	return f, err
}

func (db *DB) FormGetIDByEmail(email string, FormTemplateID, UserID int64) (int64, error) {
	var id int64
	if email == "" || FormTemplateID == 0 || UserID == 0 {
		return id, fmt.Errorf("FormGetIDByEmail: empty email %q or formtemplates_id_fk %d or users_id_fk %d", email, FormTemplateID, UserID)
	}
	err := db.DB.Get(&id, `SELECT id FROM forms WHERE email=$1 AND users_id_fk=$2 AND formtemplates_id_fk=$3`, email, UserID, FormTemplateID)
	return id, err
}

func (db *DB) FormGet(FormID, FormTemplateID int64) (Form, error) {
	var f Form
	if FormID == 0 || FormTemplateID == 0 {
		return f, fmt.Errorf("FormGetEmailNaS: undefined FormID %d or formtemplates_id_fk %d", FormID, FormTemplateID)
	}
	err := db.DB.Get(&f, `SELECT * FROM forms WHERE id=$1 AND formtemplates_id_fk=$2`, FormID, FormTemplateID)
	return f, err
}

func (db *DB) FormModByEmail(f *Form) error {
	if f.Email.String == "" || f.FormTemplateID == 0 || f.UserID == 0 {
		return fmt.Errorf("FormModByEmail: empty email %q or formtemplates_id_fk %d or users_id_fk %d", f.Email.String, f.FormTemplateID, f.UserID)
	}
	_, err := db.DB.NamedExec(`UPDATE forms SET name=:name, surname=:surname, notes=:notes WHERE email=:email AND users_id_fk=:users_id_fk AND  formtemplates_id_fk=:formtemplates_id_fk`, f)
	return err
}

func (db *DB) FormDel(FormID, TemplateID int64) error {
	ret, err := db.DB.Exec(`DELETE FROM forms WHERE id=$1 AND formtemplates_id_fk=$2`, FormID, TemplateID)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("form %d (templ: %d) not found", FormID, TemplateID)
	}
	return err
}

func (db *DB) FormAnswerAdd(fa *FormAnswer) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO formanswers (value, formfields_id_fk, forms_id_fk)
	VALUES(:value, :formfields_id_fk, :forms_id_fk)`, fa)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) FormAnswerGetByField(FormID, FormFieldID int64) (string, error) {
	var fa string
	err := db.DB.Get(&fa, `SELECT value FROM formanswers WHERE forms_id_fk=$1 AND formfields_id_fk=$2`, FormID, FormFieldID)
	return fa, err
}

func (db *DB) FormAnswerGetByFieldDisplay(FormID, TemplateID int64, FormFieldDisplay string) (string, error) {
	var fa string
	err := db.DB.Get(&fa, `SELECT value FROM formanswers WHERE forms_id_fk=$1 AND formfields_id_fk=(SELECT id FROM formfields WHERE display=$2 AND formtemplates_id_fk=$3)`, FormID, FormFieldDisplay, TemplateID)
	return fa, err
}

func (db *DB) FormAnswerGetByFieldName(FormID, TemplateID int64, FormFieldName string) (string, error) {
	var fa string
	err := db.DB.Get(&fa, `SELECT value FROM formanswers WHERE forms_id_fk=$1 AND formfields_id_fk=(SELECT id FROM formfields WHERE name=$2 AND formtemplates_id_fk=$3)`, FormID, FormFieldName, TemplateID)
	return fa, err
}

func (db *DB) FormAnswerGetAll(FormID int64) ([]FormAnswer, error) {
	fa := []FormAnswer{}
	err := db.DB.Select(&fa, `SELECT * FROM formanswers WHERE forms_id_fk=$1`, FormID)
	return fa, err
}

func (db *DB) FormAnswerGetAllForTemplate(FormTemplateID int64) ([]FormAnswer, error) {
	fa := []FormAnswer{}
	err := db.DB.Select(&fa, `SELECT * FROM formanswers WHERE formtemplates_id_fk=$1`, FormTemplateID)
	return fa, err
}

// FormAnswerGetAllAnswersForFieldInts works only for data which can be converted to int64
func (db *DB) FormAnswerGetAllAnswersForFieldInts(FieldID int64) ([]int64, error) {
	fa := []int64{}
	err := db.DB.Select(&fa, `SELECT Value FROM formanswers WHERE formfields_id_fk=$1`, FieldID)
	return fa, err
}

func (db *DB) FormAnswerMod(fa *FormAnswer) error {
	if fa.FormFieldID == 0 || fa.FormID == 0 {
		return fmt.Errorf("FormAnswerMod: empty FormFieldID %d or FormID %d", fa.FormFieldID, fa.FormID)
	}
	_, err := db.DB.NamedExec(`UPDATE formanswers SET value=:value WHERE formfields_id_fk=:formfields_id_fk AND forms_id_fk=:forms_id_fk`, fa)
	return err
}

func (db *DB) FormAnswerDel(FormID, TemplateID int64) error {
	ret, err := db.DB.Exec(`DELETE FROM formanswers
		WHERE id IN ( SELECT fa.id FROM formanswers fa
 						JOIN forms ON fa.forms_id_fk=forms.id 
						WHERE forms_id_fk=$1 AND forms.formtemplates_id_fk=$2)`, FormID, TemplateID) // join is just for security
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("formanswers for form %d (templ: %d) not found", FormID, TemplateID)
	}
	return err
}

func (db *DB) FormNotificationLogAdd(fa *FormNotificationLog) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO formnotificationslog (date, notifications_id_fk, forms_id_fk)
	VALUES(:date, :notifications_id_fk, :forms_id_fk)`, fa)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) FormNotificationLogGetAll(FormID int64) ([]FormNotificationLog, error) {
	ff := []FormNotificationLog{}
	err := db.DB.Select(&ff, `SELECT * FROM formnotificationslog WHERE forms_id_fk=$1`, FormID)
	return ff, err
}

func (db *DB) FormNotificationLogGetLast(FormID int64) (int64, error) {
	var f int64
	err := db.DB.Get(&f, `SELECT MAX(date) FROM formnotificationslog WHERE forms_id_fk=$1`, FormID)
	return f, err
}

func (db *DB) FormNotificationLogGetAmount(FormID int64) (int64, error) {
	var f int64
	err := db.DB.Get(&f, `SELECT COUNT(id) FROM formnotificationslog WHERE forms_id_fk=$1`, FormID)
	return f, err
}

func (db *DB) BankAccountAdd(ba *BankAccount) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO bankaccounts (name, iban, recipientname, bank, message, currency, varsymbol, amountfield, users_id_fk)
	VALUES (:name, :iban, :recipientname, :bank, :message, :currency, :varsymbol, :amountfield, :users_id_fk)`, ba)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) BankAccountMod(ba *BankAccount) error {
	if ba.ID == 0 || ba.UserID == 0 {
		return fmt.Errorf("BankAccountMod: empty ID %d or UserID %d", ba.ID, ba.UserID)
	}
	_, err := db.DB.NamedExec(`UPDATE bankaccounts SET name=:name, iban=:iban, recipientname=:recipientname, bank=:bank, message=:message, currency=:currency, varsymbol=:varsymbol, amountfield=:amountfield WHERE id=:id AND users_id_fk=:users_id_fk`, ba)
	return err
}

func (db *DB) BankAccountModByName(ba *BankAccount) error {
	if ba.Name == "" || ba.UserID == 0 {
		return fmt.Errorf("BankAccountMod: empty name %q or UserID %d", ba.Name, ba.UserID)
	}
	_, err := db.DB.NamedExec(`UPDATE bankaccounts SET iban=:iban, recipientname=:recipientname, bank=:bank, message=:message, currency=:currency, varsymbol=:varsymbol, amountfield=:amountfield WHERE name=:name AND users_id_fk=:users_id_fk`, ba)
	return err
}

func (db *DB) BankAccountGetAll(UserID int64) ([]BankAccount, error) {
	ba := []BankAccount{}
	err := db.DB.Select(&ba, `SELECT * FROM bankaccounts WHERE users_id_fk=$1`, UserID)
	return ba, err
}

// userID is there just to doublecheck
func (db *DB) BankAccountGetByID(id int64, userID int64) (BankAccount, error) {
	c := BankAccount{}
	err := db.DB.Get(&c, `SELECT * FROM bankaccounts WHERE id=$1 AND users_id_fk=$2 ORDER BY id DESC LIMIT 1`, id, userID)
	return c, err
}

func (db *DB) BankAccountGetByName(name string, userID int64) (BankAccount, error) {
	c := BankAccount{}
	err := db.DB.Get(&c, `SELECT * FROM bankaccounts WHERE name=$1 AND users_id_fk=$2 ORDER BY id DESC LIMIT 1`, name, userID)
	return c, err
}

func (db *DB) NotificationGetAllForUser(UserID int64) ([]Notification, error) {
	nn := []Notification{}
	err := db.DB.Select(&nn, `SELECT * FROM Notifications WHERE users_id_fk=$1`, UserID)
	return nn, err
}

func (db *DB) NotificationGetAllRelatedToEventsForUser(UserID int64) ([]Notification, error) {
	nn := []Notification{}
	err := db.DB.Select(&nn, `SELECT * FROM Notifications WHERE users_id_fk=$1 AND related_to='events'`, UserID)
	return nn, err
}

func (db *DB) NotificationGetAllRelatedToFormsForUser(UserID int64) ([]Notification, error) {
	nn := []Notification{}
	err := db.DB.Select(&nn, `SELECT * FROM Notifications WHERE users_id_fk=$1 AND related_to='forms'`, UserID)
	return nn, err
}

func (db *DB) NotificationGetAllUsersAndSharable(UserID int64) ([]Notification, error) {
	nn := []Notification{}
	err := db.DB.Select(&nn, `SELECT * FROM Notifications WHERE users_id_fk=$1 OR sharable=1`, UserID)
	return nn, err
}

// userID is there just to doublecheck
func (db *DB) NotificationGetByID(id int64, userID int64) (Notification, error) {
	c := Notification{}
	err := db.DB.Get(&c, `SELECT * FROM notifications WHERE id=$1 AND users_id_fk=$2 ORDER BY id DESC LIMIT 1`, id, userID)
	return c, err
}

// NotificationGetByIDUnsafe no user is check. This is useful for shared notifications,
// where other users need to be able to access notification data.
func (db *DB) NotificationGetByIDUnsafe(id int64) (Notification, error) {
	c := Notification{}
	err := db.DB.Get(&c, `SELECT * FROM notifications WHERE id=$1 ORDER BY id DESC LIMIT 1`, id)
	return c, err
}

func (db *DB) NotificationAdd(not *Notification) (int64, error) {
	if not.ID != 0 {
		return -1, fmt.Errorf("NotificationAdd: ID is not 0, this is existing notification with ID:%d", not.ID)
	}
	ret, err := db.DB.NamedExec(`INSERT INTO notifications (name, type, related_to, title, text, embedded_imgs, attached_imgs, sharable, created_date, users_id_fk) 
	VALUES (:name, :type, :related_to, :title, :text, :embedded_imgs, :attached_imgs, :sharable, :created_date, :users_id_fk)`, not)
	if err != nil {
		return -1, err
	}

	return ret.LastInsertId()
}

func (db *DB) NotificationModByID(not *Notification, UserID int64) error {
	if not.ID == 0 || not.UserID == 0 {
		return fmt.Errorf("NotificationModByID: ID is zero: %q or UserID %d is 0", not.Name, not.UserID)
	}
	if not.UserID != UserID {
		return fmt.Errorf("NotificationModByID: can not update notification %d, userID mismatch. not.User=%d vs UserID=%d", not.ID, not.UserID, UserID)
	}

	_, err := db.DB.NamedExec(`UPDATE notifications SET name=:name, type=:type, related_to=:related_to, title=:title, text=:text, embedded_imgs=:embedded_imgs, attached_imgs=:attached_imgs, sharable=:sharable, updated_date=:updated_date WHERE id=:id and users_id_fk=:users_id_fk`, not)

	return err
}

func (db *DB) EventAddonGetAllByEvent(EventID int64) ([]EventAddon, error) {
	adds := []EventAddon{}
	err := db.DB.Select(&adds, `SELECT * FROM eventsaddons WHERE
								and events_id_fk = $1`, EventID)
	return adds, err
}

func (db *DB) EventAddonAdd(e EventAddon) (int64, error) {
	if e.Name == "" || e.EventID == 0 {
		return -1, fmt.Errorf("EventAddonAdd: Name (%s) or EventID (%d) is empty", e.Name, e.EventID)
	}
	ret, err := db.DB.NamedExec(`INSERT INTO eventsaddons (name, price, currency, events_id_fk, users_id_fk) 
	VALUES (:name, :price, :currency, :events_id_fk, :users_id_fk)`, e)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

// UNUSED
func (db *DB) NotificationModByName(not *Notification) error {
	if not.Name == "" || not.UserID == 0 {
		return fmt.Errorf("NotificationModByName: empty name %q or UserID %d", not.Name, not.UserID)
	}
	_, err := db.DB.NamedExec(`UPDATE notifications SET type=:type, related_to=:related_to, title=:title, text=:text, embedded_imgs=:embedded_imgs, attached_imgs=:attached_imgs, sharable=:sharable, updated_date=:updated_date, users_id_fk=:users_id_fk`, not)
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

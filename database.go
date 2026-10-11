package main

import (
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// ConnOptions - przy zajętej bazie (np. kopia zapasowa z sqlite3) czekaj
// zamiast od razu zwracać "database is locked"
const ConnOptions = "?_pragma=busy_timeout(10000)"

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
	// Language - język panelu administracyjnego (tools/migration-redesign.sql)
	Language sql.NullString `db:"language"`
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
	Banner      sql.NullString `db:"banner_img"`  // also moving to events
	Description sql.NullString `db:"description"` // we are moving it to Event
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
	ID                      int64          `db:"id"`
	Name                    string         `db:"name"`
	Date                    int64          `db:"date"`
	FromDate                int64          `db:"from_date"`
	ToDate                  int64          `db:"to_date"`
	DefaultPrice            int64          `db:"default_price"`
	DefaultCurrency         string         `db:"default_currency"`
	NoSitsSelectedTitle     string         `db:"no_sits_selected_title"`
	NoSitsSelectedText      string         `db:"no_sits_selected_text"`
	OrderHowto              string         `db:"order_howto"`
	OrderNotesDescription   string         `db:"order_notes_desc"`
	OrderedNoteTitle        string         `db:"ordered_note_title"`
	OrderedNoteText         string         `db:"ordered_note_text"`
	HowTo                   string         `db:"how_to"`
	UserID                  int64          `db:"users_id_fk"`
	ThankYouNotificationsID sql.NullInt64  `db:"thankyou_notifications_id_fk"`
	AdminNotificationsID    sql.NullInt64  `db:"admin_notifications_id_fk"`
	Sharable                sql.NullBool   `db:"sharable"`
	BankAccountsID          sql.NullInt64  `db:"bankaccounts_id_fk"`
	Language                sql.NullString `db:"language"`
	HideCosts               sql.NullBool   `db:"hidecosts"`
	MainDesc                sql.NullString `db:"maindesc"`
	MainBanner              sql.NullString `db:"mainbanner"`
	Room1Desc               sql.NullString `db:"room1desc"`
	Room1Banner             sql.NullString `db:"room1banner"`
	Room2Desc               sql.NullString `db:"room2desc"`
	Room2Banner             sql.NullString `db:"room2banner"`
	Room3Desc               sql.NullString `db:"room3desc"`
	Room3Banner             sql.NullString `db:"room3banner"`
	Room4Desc               sql.NullString `db:"room4desc"`
	Room4Banner             sql.NullString `db:"room4banner"`
	// Kolumny dołożone przez redesign (tools/migration-redesign.sql,
	// tools/migration-addons.sql). Stare zapytania ich nie zapisują, więc
	// ustawia się je osobnymi metodami.
	PreviousEventID      sql.NullInt64  `db:"previous_event_id"`
	Status               sql.NullString `db:"status"`
	AddonsFormTemplateID sql.NullInt64  `db:"addons_formtemplate_id"`
	// Theme - temat graficzny strony gościa (themes.go, tools/migration-themes.sql)
	Theme sql.NullString `db:"theme"`
	// PayMessage - początek wiadomości dla odbiorcy przelewu (pay.go,
	// tools/migration-paymessage.sql)
	PayMessage string `db:"pay_message"`
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

type EventNotificationLog struct {
	ID             int64 `db:"id"`
	Date           int64 `db:"date"`
	NotificationID int64 `db:"notifications_id_fk"`
	ReservationID  int64 `db:"reservations_id_fk"`
}

type Reservation struct {
	ID           int64          `db:"id"`
	OrderedDate  sql.NullInt64  `db:"ordered_date"`
	OrderedDateS string         `db:"-"`
	PayedDate    sql.NullInt64  `db:"payed_date"`
	PayedDateS   string         `db:"-"`
	Price        sql.NullInt64  `db:"price"`
	Currency     sql.NullString `db:"currency"`
	// Status: free, marked, ordered, confirmed, disabled
	Status      string        `db:"status"`
	NoteID      sql.NullInt64 `db:"notes_id_fk"`
	FurnitureID int64         `db:"furnitures_id_fk"`
	EventID     int64         `db:"events_id_fk"`
	CustomerID  int64         `db:"customers_id_fk"`
	// OrderID - zamówienie, do którego należy miejsce (tools/migration-addons.sql)
	OrderID sql.NullInt64 `db:"orders_id_fk"`
}

// CustomerOrder to zamówienie gościa: spina klienta, imprezę, rezerwacje
// miejsc i pozycje dodatków. Nazwa nie może być "Order", bo tak nazywa się
// struktura szablonów maili w main.go.
type CustomerOrder struct {
	ID          int64          `db:"id"`
	CreatedDate int64          `db:"created_date"`
	Notes       sql.NullString `db:"notes"`
	EventID     int64          `db:"events_id_fk"`
	CustomerID  int64          `db:"customers_id_fk"`
}

// OrderItem to pozycja zamówienia: dodatek albo dopłata. Miejsca NIE są
// pozycjami - ich stan zostaje w reservations.status (rozdz. 9.4).
type OrderItem struct {
	ID          int64          `db:"id"`
	Kind        string         `db:"kind"` // addon albo surcharge
	Description string         `db:"description"`
	Amount      int64          `db:"amount"`
	Price       int64          `db:"price"`
	Currency    sql.NullString `db:"currency"`
	Status      string         `db:"status"` // ordered albo payed
	PayedDate   sql.NullInt64  `db:"payed_date"`
	CreatedDate int64          `db:"created_date"`
	OrderID     int64          `db:"orders_id_fk"`
}

// ReservationStatusLog - historia zmian statusu rezerwacji
// (tools/migration-redesign.sql).
type ReservationStatusLog struct {
	ID            int64          `db:"id"`
	Date          int64          `db:"date"`
	Status        string         `db:"status"`
	AdminEmail    sql.NullString `db:"admin_email"`
	ReservationID int64          `db:"reservations_id_fk"`
}

type ReservationFull struct {
	Reservation
	LastNotification  sql.NullInt64  `db:"enlog_date"`
	LastNotificationS string         `db:"-"`
	NotifAmmount      sql.NullInt64  `db:"enlog_ammount"`
	CustEmail         sql.NullString `db:"cust_email"`
	CustName          sql.NullString `db:"cust_name"`
	CustSurname       sql.NullString `db:"cust_surname"`
	CustPhone         sql.NullString `db:"cust_phone"`
	ChairNumber       int64          `db:"chair_number"`
	RoomName          string         `db:"room_name"`
	RoomID            int64          `db:"room_id"`
	Notes             sql.NullString `db:"reservation_notes"`
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
	// Kind - "form" (zwykły formularz) albo "order_addons" (definicja dodatków
	// do zamówienia, przypięta do imprezy przez events.addons_formtemplate_id)
	Kind sql.NullString `db:"kind"`
	// Language - język strony dla gościa (pl/cs/en), Status - open/closed,
	// CloseDate - opcjonalny termin zamknięcia (tools/migration-forms.sql)
	Language  string        `db:"language"`
	Status    string        `db:"status"`
	CloseDate sql.NullInt64 `db:"close_date"`
	// InfoPanelMode - none/auto/fields/text, patrz form_stats.go
	InfoPanelMode string `db:"infopanel_mode"`
	// Theme - temat graficzny strony gościa (themes.go, tools/migration-themes.sql)
	Theme string `db:"theme"`
	// PayMessage - początek wiadomości dla odbiorcy przelewu (pay.go,
	// tools/migration-paymessage.sql)
	PayMessage string `db:"pay_message"`
}

// IsOpen mówi, czy formularz przyjmuje teraz odpowiedzi.
func (t FormTemplate) IsOpen(now int64) bool {
	if t.Status == "closed" {
		return false
	}
	if t.CloseDate.Valid && t.CloseDate.Int64 > 0 && now > t.CloseDate.Int64 {
		return false
	}
	return true
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
	// OrderID - gdy odpowiedź to dodatki jednego zamówienia
	OrderID sql.NullInt64 `db:"orders_id_fk"`
}

type FormAnswer struct {
	ID          int64          `db:"id"`
	Value       sql.NullString `db:"value"`
	FormFieldID int64          `db:"formfields_id_fk"`
	FormID      int64          `db:"forms_id_fk"`
	// RepeatIndex - numer powtórzenia pola ("Kolacja 2" przy polu powtarzanym
	// wg liczby miejsc); 0 dla zwykłych pól
	RepeatIndex sql.NullInt64 `db:"repeat_index"`
}

type BankAccount struct {
	ID            int64          `db:"id"`
	Name          string         `db:"name"`
	IBAN          string         `db:"iban"`
	RecipientName sql.NullString `db:"recipientname"`
	BankID        sql.NullString `db:"bank"`
	Currency      string         `db:"currency"`
	// Message i AmountField - nieużywane: wiadomość dla odbiorcy ustawia się
	// w imprezie/formularzu (pay_message), pole kwoty w formularzu (moneyfield)
	Message     sql.NullString `db:"message"`
	VarSymbol   sql.NullInt64  `db:"varsymbol"`
	AmountField sql.NullString `db:"amountfield"`
	UserID      int64          `db:"users_id_fk"`
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

// UserSetPass zapisuje hash hasła (auth.go), nigdy jawne hasło.
func (db *DB) UserSetPass(email, hash string) error {
	_, err := db.DB.Exec(`UPDATE users SET passwd=$1 WHERE email=$2`, hash, email)
	return err
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

func (db *DB) RoomGetByID(id int64) (Room, error) {
	room := Room{}
	err := db.DB.Get(&room, `SELECT * FROM rooms WHERE id=$1`, id)
	return room, err
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

func (db *DB) RoomModSizeByID(room *Room) error {
	_, err := db.DB.NamedExec(`UPDATE rooms SET width=:width, height=:height WHERE id=:id`, room)
	return err
}

// RoomWithFurnituresCopy copies room with all furnitures in it by given room id,
// if name is empty, original room name is used
func (db *DB) RoomWithFurnituresCopy(id int64, name string) (int64, error) {
	r, err := db.RoomGetByID(id)
	if err != nil {
		return 0, fmt.Errorf("RoomWithFurnituresCopy: error getting room %v, %v", id, err)
	}
	r.ID = 0 // reset id
	if name != "" {
		r.Name = name
	}
	roomid, err := db.RoomAdd(&r)
	if err != nil {
		return 0, fmt.Errorf("RoomWithFurnituresCopy: error writing room copy to db (org: %v, new: %v), %v", id, roomid, err)
	}

	ff, err := db.FurnitureGetAllByRoomID(id)
	if err != nil {
		return roomid, fmt.Errorf("RoomWithFurnituresCopy: error getting furnitures for room copy (%v), %v", roomid, err)
	}

	for i := range ff {
		ff[i].ID = 0          // reset id
		ff[i].RoomID = roomid // change to new room
		_, err := db.FurnitureAdd(&ff[i])
		if err != nil {
			return roomid, fmt.Errorf("RoomWithFurnituresCopy: error adding furniture(%v) to copied room(%v), %v", ff[i], roomid, err)
		}
	}
	return roomid, nil
}

// rooms_id_fk is uniq, so only one owner of room can exist
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

func (db *DB) FurnitureAdd(f *Furniture) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO furnitures (number, type, orientation, x, y, width, height, color, label, capacity, rooms_id_fk)
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

func (db *DB) FurnitureGetAllByRoomNameOfType(name, ftype string) ([]Furniture, error) {
	furnitures := []Furniture{}
	err := db.DB.Select(&furnitures, `SELECT * FROM furnitures WHERE type=$1 AND rooms_id_fk=(SELECT id FROM rooms WHERE name=$2) ORDER BY number`, ftype, name)
	return furnitures, err
}

func (db *DB) FurnitureFullGetChairs(eventID int64, name string) ([]FurnitureFull, error) {
	furnitures := []FurnitureFull{}
	err := db.DB.Select(&furnitures, `SELECT f.*, p.price admin_price,p.currency admin_currency, p.disabled admin_disabled, r.price bought_price, r.status status, r.ordered_date ordered_date, r.payed_date payed_date, r.customers_id_fk reservation_customers_id_fk FROM furnitures f LEFT JOIN prices p ON f.id = p.furnitures_id_fk AND p.events_id_fk=$1 LEFT JOIN reservations r ON f.id = r.furnitures_id_fk AND r.events_id_fk=$1 WHERE f.type="chair" AND f.rooms_id_fk=(SELECT id FROM rooms WHERE name=$2) ORDER BY f.number`, eventID, name)
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

// we are not adding HideCosts here, as it is not available in interface
func (db *DB) EventAdd(e *Event) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO events (name, date, from_date, to_date, default_price, default_currency, no_sits_selected_title, no_sits_selected_text, order_howto, order_notes_desc, ordered_note_title, ordered_note_text, how_to, users_id_fk, thankyou_notifications_id_fk, admin_notifications_id_fk, sharable, bankaccounts_id_fk, language, maindesc, mainbanner, room1desc, room1banner, room2desc, room2banner, room3desc, room3banner, room4desc, room4banner)
	VALUES(:name, :date, :from_date, :to_date, :default_price, :default_currency, :no_sits_selected_title, :no_sits_selected_text, :order_howto, :order_notes_desc, :ordered_note_title, :ordered_note_text, :how_to, :users_id_fk, :thankyou_notifications_id_fk, :admin_notifications_id_fk, :sharable, :bankaccounts_id_fk, :language, :maindesc, :mainbanner, :room1desc, :room1banner, :room2desc, :room2banner, :room3desc, :room3banner, :room4desc, :room4banner)`, e)
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

// EventNameExists checks if user already has event with given name (name is uniq per user)
func (db *DB) EventNameExists(name string, userID int64) (bool, error) {
	var count int64
	err := db.DB.Get(&count, `SELECT count(*) FROM events WHERE name=$1 AND users_id_fk=$2`, name, userID)
	return count > 0, err
}

// PriceCopyForEventRoom copies prices (and admin disabled flag) of furnitures
// in room fromRoomID for event fromEventID to event toEventID and room toRoomID.
// Furnitures are matched by type and number, so it works for the same room
// and also for room copy.
func (db *DB) PriceCopyForEventRoom(fromEventID, fromRoomID, toEventID, toRoomID int64) (int64, error) {
	ret, err := db.DB.Exec(`INSERT INTO prices (price, currency, disabled, events_id_fk, furnitures_id_fk)
	SELECT p.price, p.currency, p.disabled, $1, nf.id FROM prices p
	JOIN furnitures f ON p.furnitures_id_fk = f.id
	JOIN furnitures nf ON nf.rooms_id_fk = $2 AND nf.type = f.type AND nf.number = f.number
	WHERE p.events_id_fk = $3 AND f.rooms_id_fk = $4`, toEventID, toRoomID, fromEventID, fromRoomID)
	if err != nil {
		return -1, err
	}
	return ret.RowsAffected()
}

// EventAddonCopy copies all addons from one event to another
func (db *DB) EventAddonCopy(fromEventID, toEventID, userID int64) (int64, error) {
	ret, err := db.DB.Exec(`INSERT INTO eventsaddons (name, price, currency, events_id_fk, users_id_fk)
	SELECT name, price, currency, $1, $2 FROM eventsaddons WHERE events_id_fk = $3`, toEventID, userID, fromEventID)
	if err != nil {
		return -1, err
	}
	return ret.RowsAffected()
}

func (db *DB) EventGetRooms(eventID int64) ([]Room, error) {
	rooms := []Room{}
	err := db.DB.Select(&rooms, `SELECT r.* FROM rooms r LEFT JOIN events_rooms er ON r.id = er.rooms_id_fk WHERE er.events_id_fk = $1 ORDER BY r.id`, eventID)
	return rooms, err
}

// EventClearRooms usuwa wszystkie przypisania sal do imprezy.
// Używa go zapis sekcji "Sala i ceny" w nowym panelu (admin_ui.go).
func (db *DB) EventClearRooms(eventID int64) error {
	if err := NoMinus("eventID", eventID); err != nil {
		return err
	}
	_, err := db.DB.Exec(`DELETE FROM events_rooms WHERE events_id_fk = $1`, eventID)
	return err
}

// EventSetStatus ustawia stan imprezy: "draft" (szkic, niewidoczna dla gości)
// albo "published". Osobna metoda, bo stare zapytania nie znają tej kolumny.
// EventSetPayMessage ustawia początek wiadomości dla odbiorcy przelewu.
func (db *DB) EventSetPayMessage(eventID, userID int64, msg string) error {
	_, err := db.DB.Exec(`UPDATE events SET pay_message=$1 WHERE id=$2 AND users_id_fk=$3`, msg, eventID, userID)
	return err
}

// EventSetTheme ustawia temat graficzny stron imprezy dla gości.
func (db *DB) EventSetTheme(eventID, userID int64, theme string) error {
	_, err := db.DB.Exec(`UPDATE events SET theme=$1 WHERE id=$2 AND users_id_fk=$3`, theme, eventID, userID)
	return err
}

func (db *DB) EventSetStatus(eventID, userID int64, status string) error {
	_, err := db.DB.Exec(`UPDATE events SET status=$1 WHERE id=$2 AND users_id_fk=$3`, status, eventID, userID)
	return err
}

// EventSetPreviousEdition wskazuje poprzednią edycję imprezy (do porównań).
func (db *DB) EventSetPreviousEdition(eventID, userID int64, prevID sql.NullInt64) error {
	if prevID.Valid && prevID.Int64 == eventID {
		return fmt.Errorf("event %d can not be its own previous edition", eventID)
	}
	_, err := db.DB.Exec(`UPDATE events SET previous_event_id=$1 WHERE id=$2 AND users_id_fk=$3`,
		prevID, eventID, userID)
	return err
}

// ReservationsSoldBefore liczy miejsca sprzedane do danej chwili (po dacie
// zamówienia). Służy do porównania edycji "tyle samo dni przed imprezą".
func (db *DB) ReservationsSoldBefore(eventID, until int64) (int64, int64, error) {
	var seats, sum sql.NullInt64
	err := db.DB.QueryRow(`SELECT count(*), COALESCE(sum(price), 0) FROM reservations
		WHERE events_id_fk=$1 AND status IN ('ordered','payed')
		AND COALESCE(ordered_date, 0) <= $2`, eventID, until).Scan(&seats, &sum)
	return seats.Int64, sum.Int64, err
}

// EventSetHideCosts zapisuje samo pole hidecosts.
// Osobno, bo EventModByID tej kolumny nie aktualizuje (stary edytor imprezy
// nie zna tego pola i skasowałby ustawienie).
func (db *DB) EventSetHideCosts(eventID, userID int64, hide bool) error {
	_, err := db.DB.Exec(`UPDATE events SET hidecosts=$1 WHERE id=$2 AND users_id_fk=$3`, hide, eventID, userID)
	return err
}

func (db *DB) EventModByID(event *Event, UserID int64) error {
	if event.ID == 0 {
		return fmt.Errorf("can not modify event if ID is 0")
	}
	if event.UserID != UserID {
		return fmt.Errorf("event.UserID %d do not match UserID %d, can't update event", event.UserID, UserID)
	}
	_, err := db.DB.NamedExec(`UPDATE events SET name=:name, date=:date, from_date=:from_date, to_date=:to_date, default_price=:default_price, default_currency=:default_currency, no_sits_selected_title=:no_sits_selected_title, no_sits_selected_text=:no_sits_selected_text, order_howto=:order_howto, order_notes_desc=:order_notes_desc, ordered_note_title=:ordered_note_title, ordered_note_text=:ordered_note_text, how_to=:how_to, thankyou_notifications_id_fk=:thankyou_notifications_id_fk, admin_notifications_id_fk=:admin_notifications_id_fk, sharable=:sharable, bankaccounts_id_fk=:bankaccounts_id_fk, language=:language, maindesc=:maindesc, mainbanner=:mainbanner, room1desc=:room1desc, room1banner=:room1banner, room2desc=:room2desc, room2banner=:room2banner, room3desc=:room3desc, room3banner=:room3banner, room4desc=:room4desc, room4banner=:room4banner WHERE id=:id AND users_id_fk=:users_id_fk`, event)
	return err
}

func (db *DB) ReservationGet(furnitureID, eventID int64) (Reservation, error) {
	r := Reservation{}
	err := db.DB.Get(&r, `SELECT * FROM reservations WHERE furnitures_id_fk=$1 AND events_id_fk=$2`, furnitureID, eventID)
	return r, err
}

// ReservationGetByID, eventID is just to be sure all is ok
//func (db *DB) ReservationGetByID(reservationID, eventID int64) (Reservation, error) {
//	r := Reservation{}
//	err := db.DB.Get(&r, `SELECT * FROM reservations WHERE id=$1 AND events_id_fk=$2`, reservationID, eventID)
//	return r, err
//}

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
	err = db.DB.Select(&rr, `SELECT r.*, c.email cust_email, c.name cust_name, c.surname cust_surname, c.phone cust_phone, f.number chair_number, rm.name room_name, rm.id room_id, n.text reservation_notes, enl.date enlog_date, (SELECT count(*) FROM eventnotificationslog ienl WHERE ienl.reservations_id_fk = r.id) enlog_ammount FROM reservations r LEFT JOIN furnitures f ON r.furnitures_id_fk = f.id LEFT JOIN rooms rm ON f.rooms_id_fk = rm.id LEFT JOIN customers c ON r.customers_id_fk = c.id LEFT JOIN events e ON r.events_id_fk = e.id LEFT JOIN notes n ON r.notes_id_fk = n.id LEFT JOIN eventnotificationslog enl ON r.id = enl.reservations_id_fk LEFT JOIN eventnotificationslog enl2 ON r.id = enl2.reservations_id_fk AND enl.id < enl2.id  WHERE enl2.reservations_id_fk IS NULL AND e.users_id_fk = $1 AND r.events_id_fk=$2 ORDER BY f.number`, userID, eventID)
	return rr, err

}

func (db *DB) ReservationFullGetByID(reservationID, eventID int64) (ReservationFull, error) {
	rr := ReservationFull{}
	err := NoMinus("reservationID", reservationID)
	if err != nil {
		return rr, err
	}
	err = NoMinus("eventID", eventID)
	if err != nil {
		return rr, err
	}
	err = db.DB.Get(&rr, `SELECT r.*, c.email cust_email, c.name cust_name, c.surname cust_surname, c.phone cust_phone, f.number chair_number, rm.name room_name, rm.id room_id, n.text reservation_notes, enl.date enlog_date, (SELECT count(*) FROM eventnotificationslog ienl WHERE ienl.reservations_id_fk = r.id) enlog_ammount FROM reservations r LEFT JOIN furnitures f ON r.furnitures_id_fk = f.id LEFT JOIN rooms rm ON f.rooms_id_fk = rm.id LEFT JOIN customers c ON r.customers_id_fk = c.id LEFT JOIN events e ON r.events_id_fk = e.id LEFT JOIN notes n ON r.notes_id_fk = n.id LEFT JOIN eventnotificationslog enl ON r.id = enl.reservations_id_fk LEFT JOIN eventnotificationslog enl2 ON r.id = enl2.reservations_id_fk AND enl.id < enl2.id  WHERE enl2.reservations_id_fk IS NULL AND r.id = $1 AND r.events_id_fk=$2 ORDER BY f.number`, reservationID, eventID)
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

// TODO: does affected check make any sense? there is error if no row affected IMO
// ReservationSetStatus ustawia status jednej rezerwacji po jej ID.
// Data zapłaty ustawiana jest na serwerze; cofnięcie statusu czyści ją (NULL),
// zamiast wpisywać 1970-01-01 jak stary panel.
func (db *DB) ReservationSetStatus(id, eventID int64, status string, payedDate sql.NullInt64) error {
	if err := NoMinus("reservationID", id); err != nil {
		return err
	}
	ret, err := db.DB.Exec(`UPDATE reservations SET status=$1, payed_date=$2 WHERE id=$3 AND events_id_fk=$4`,
		status, payedDate, id, eventID)
	if err != nil {
		return err
	}
	n, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no reservation %d in event %d", id, eventID)
	}
	return nil
}

// ReservationCountByFurnitureNumber liczy rezerwacje przypisane do mebla
// wskazanego tak, jak robi to designer: numerem, typem i salą.
// Służy do ochrony zajętych krzeseł przed skasowaniem (rozdz. 5.3 HARMONIJKA.md).
func (db *DB) ReservationCountByFurnitureNumber(number int64, ftype string, roomID int64) (int64, error) {
	var n int64
	err := db.DB.Get(&n, `SELECT count(*) FROM reservations r
		JOIN furnitures f ON r.furnitures_id_fk = f.id
		WHERE f.number=$1 AND f.type=$2 AND f.rooms_id_fk=$3`, number, ftype, roomID)
	return n, err
}

// ReservationCountByFurnitureID - to samo, ale po ID mebla.
func (db *DB) ReservationCountByFurnitureID(furnID int64) (int64, error) {
	var n int64
	err := db.DB.Get(&n, `SELECT count(*) FROM reservations WHERE furnitures_id_fk=$1`, furnID)
	return n, err
}

// ReservationStatusLogAdd zapisuje zmianę statusu rezerwacji
// (tabela z tools/migration-redesign.sql).
func (db *DB) ReservationStatusLogAdd(l *ReservationStatusLog) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO reservation_status_log (date, status, admin_email, reservations_id_fk)
	VALUES(:date, :status, :admin_email, :reservations_id_fk)`, l)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

// ReservationStatusLogGet zwraca historię zmian dla podanych rezerwacji,
// od najnowszej.
func (db *DB) ReservationStatusLogGet(ids []int64) ([]ReservationStatusLog, error) {
	ll := []ReservationStatusLog{}
	if len(ids) == 0 {
		return ll, nil
	}
	q, args, err := sqlx.In(`SELECT * FROM reservation_status_log
		WHERE reservations_id_fk IN (?) ORDER BY date DESC, id DESC`, ids)
	if err != nil {
		return ll, err
	}
	err = db.DB.Select(&ll, db.DB.Rebind(q), args...)
	return ll, err
}

// ReservationSetOrder przypisuje rezerwację do zamówienia.
func (db *DB) ReservationSetOrder(reservationID, orderID int64) error {
	_, err := db.DB.Exec(`UPDATE reservations SET orders_id_fk=$1 WHERE id=$2`, orderID, reservationID)
	return err
}

// ReservationGetByIDForNote zwraca rezerwację po ID (potrzebne m.in. po to,
// żeby przy kasowaniu wiedzieć, czy zostały jeszcze rezerwacje z tą notatką).
func (db *DB) ReservationGetByIDForNote(id int64) (Reservation, error) {
	r := Reservation{}
	err := db.DB.Get(&r, `SELECT * FROM reservations WHERE id=$1`, id)
	return r, err
}

// ReservationEventID zwraca ID imprezy, do której należy rezerwacja (kontrola dostępu).
func (db *DB) ReservationEventID(id int64) (int64, error) {
	var eventID int64
	err := db.DB.Get(&eventID, `SELECT events_id_fk FROM reservations WHERE id=$1`, id)
	return eventID, err
}

// ReservationSetNotes zapisuje uwagi do rezerwacji (tworzy wpis w notes, jeśli trzeba).
func (db *DB) ReservationSetNotes(id int64, text string) error {
	r := Reservation{}
	if err := db.DB.Get(&r, `SELECT * FROM reservations WHERE id=$1`, id); err != nil {
		return err
	}
	if r.NoteID.Valid {
		_, err := db.DB.Exec(`UPDATE notes SET text=$1 WHERE id=$2`, text, r.NoteID.Int64)
		return err
	}
	noteID, err := db.NoteAdd(text)
	if err != nil {
		return err
	}
	_, err = db.DB.Exec(`UPDATE reservations SET notes_id_fk=$1 WHERE id=$2`, noteID, id)
	return err
}

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

// AdminGetEmails retrieves all mails, TODO: maybe filter by type?
// AdminSetLanguage zapisuje język panelu na koncie admina
// (kolumna z tools/migration-redesign.sql).
func (db *DB) AdminSetLanguage(email, lang string) error {
	_, err := db.DB.Exec(`UPDATE admins SET language=$1 WHERE email=$2`, lang, email)
	return err
}

// AdminGetLanguage zwraca język panelu zapisany na koncie admina.
func (db *DB) AdminGetLanguage(email string) (string, error) {
	var lang sql.NullString
	err := db.DB.Get(&lang, `SELECT language FROM admins WHERE email=$1`, email)
	return lang.String, err
}

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

// FormTemplateModByIDUser jest jak FormTemplateModByID, ale zapis jest ograniczony
// do szablonów organizacji (WHERE users_id_fk). Używa go nowy panel.
func (db *DB) FormTemplateModByIDUser(t *FormTemplate) error {
	if t.UserID < 1 {
		return fmt.Errorf("FormTemplateModByIDUser: no userID given, update aborted")
	}
	_, err := db.DB.NamedExec(`UPDATE formtemplates SET name=:name, url=:url, howto=:howto, banner=:banner, thankyou=:thankyou, infopanel=:infopanel, moneyfield=:moneyfield, bankaccounts_id_fk=:bankaccounts_id_fk, notifications_id_fk=:notifications_id_fk WHERE id=:id AND users_id_fk=:users_id_fk`, t)
	return err
}

// FormTemplateSetAdminNotification i FormTemplateSetEvent zapisują pola, których
// nie ma w FormTemplateModByID (stary edytor formularza ich nie zna, więc nie
// dopisujemy ich tam - skasowałby ustawienie przy zwykłym zapisie).
func (db *DB) FormTemplateSetAdminNotification(tmplID, userID int64, notifID sql.NullInt64) error {
	_, err := db.DB.Exec(`UPDATE formtemplates SET admin_notifications_id_fk=$1 WHERE id=$2 AND users_id_fk=$3`,
		notifID, tmplID, userID)
	return err
}

func (db *DB) FormTemplateSetEvent(tmplID, userID int64, eventID sql.NullInt64) error {
	_, err := db.DB.Exec(`UPDATE formtemplates SET events_id_fk=$1 WHERE id=$2 AND users_id_fk=$3`,
		eventID, tmplID, userID)
	return err
}

// FormTemplateSetKind ustawia rodzaj szablonu: "form" albo "order_addons"
// (definicja dodatków do zamówienia).
func (db *DB) FormTemplateSetKind(tmplID, userID int64, kind string) error {
	_, err := db.DB.Exec(`UPDATE formtemplates SET kind=$1 WHERE id=$2 AND users_id_fk=$3`, kind, tmplID, userID)
	return err
}

// EventSetAddonsTemplate przypina do imprezy szablon z definicją dodatków.
func (db *DB) EventSetAddonsTemplate(eventID, userID int64, tmplID sql.NullInt64) error {
	_, err := db.DB.Exec(`UPDATE events SET addons_formtemplate_id=$1 WHERE id=$2 AND users_id_fk=$3`,
		tmplID, eventID, userID)
	return err
}

// OrderAdd zapisuje zamówienie (spinacz klienta, imprezy i pozycji).
func (db *DB) OrderAdd(o *CustomerOrder) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO orders (created_date, notes, events_id_fk, customers_id_fk)
	VALUES(:created_date, :notes, :events_id_fk, :customers_id_fk)`, o)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

// OrderGetByID zwraca zamówienie, sprawdzając przy okazji imprezę.
func (db *DB) OrderGetByID(orderID, eventID int64) (CustomerOrder, error) {
	o := CustomerOrder{}
	err := db.DB.Get(&o, `SELECT * FROM orders WHERE id=$1 AND events_id_fk=$2`, orderID, eventID)
	return o, err
}

// OrderItemAdd dopisuje pozycję zamówienia (dodatek albo dopłata).
func (db *DB) OrderItemAdd(i *OrderItem) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO order_items (kind, description, amount, price, currency, status, payed_date, created_date, orders_id_fk)
	VALUES(:kind, :description, :amount, :price, :currency, :status, :payed_date, :created_date, :orders_id_fk)`, i)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

// OrderItemGetByID zwraca pozycję zamówienia (z kontrolą, do którego należy).
func (db *DB) OrderItemGetByID(itemID, orderID int64) (OrderItem, error) {
	i := OrderItem{}
	err := db.DB.Get(&i, `SELECT * FROM order_items WHERE id=$1 AND orders_id_fk=$2`, itemID, orderID)
	return i, err
}

// OrderItemSetStatus ustawia stan pozycji zamówienia: "ordered" albo "payed".
// Data zapłaty liczona jest na serwerze, cofnięcie ją czyści.
func (db *DB) OrderItemSetStatus(itemID, orderID int64, status string, payedDate sql.NullInt64) error {
	ret, err := db.DB.Exec(`UPDATE order_items SET status=$1, payed_date=$2 WHERE id=$3 AND orders_id_fk=$4`,
		status, payedDate, itemID, orderID)
	if err != nil {
		return err
	}
	n, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no order item %d in order %d", itemID, orderID)
	}
	return nil
}

// OrderItemDel kasuje pozycję zamówienia.
func (db *DB) OrderItemDel(itemID, orderID int64) error {
	ret, err := db.DB.Exec(`DELETE FROM order_items WHERE id=$1 AND orders_id_fk=$2`, itemID, orderID)
	if err != nil {
		return err
	}
	n, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no order item %d in order %d", itemID, orderID)
	}
	return nil
}

// OrderItemsGetByOrder zwraca pozycje zamówienia.
func (db *DB) OrderItemsGetByOrder(orderID int64) ([]OrderItem, error) {
	ii := []OrderItem{}
	err := db.DB.Select(&ii, `SELECT * FROM order_items WHERE orders_id_fk=$1 ORDER BY id`, orderID)
	return ii, err
}

// AddonAnswerCount to jedna linia podsumowania dodatków: ile razy wybrano
// daną odpowiedź w danym polu (lista dla kuchni).
type AddonAnswerCount struct {
	Field string `db:"field"`
	Value string `db:"value"`
	Count int64  `db:"cnt"`
}

// AddonAnswerSummary liczy odpowiedzi dodatków wszystkich zamówień imprezy.
func (db *DB) AddonAnswerSummary(eventID int64) ([]AddonAnswerCount, error) {
	rr := []AddonAnswerCount{}
	err := db.DB.Select(&rr, `SELECT ff.display field, COALESCE(fa.value, '') value, count(*) cnt
		FROM formanswers fa
		JOIN forms f ON fa.forms_id_fk = f.id
		JOIN orders o ON f.orders_id_fk = o.id
		JOIN formfields ff ON fa.formfields_id_fk = ff.id
		WHERE o.events_id_fk = $1
		GROUP BY ff.display, fa.value
		ORDER BY ff.display, cnt DESC`, eventID)
	return rr, err
}

// OrderItemsGetByEvent zwraca wszystkie pozycje zamówień imprezy (raporty).
func (db *DB) OrderItemsGetByEvent(eventID int64) ([]OrderItem, error) {
	ii := []OrderItem{}
	err := db.DB.Select(&ii, `SELECT oi.* FROM order_items oi
		JOIN orders o ON oi.orders_id_fk = o.id
		WHERE o.events_id_fk=$1 ORDER BY oi.id`, eventID)
	return ii, err
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
	ret, err := db.DB.Exec(`DELETE FROM formfields WHERE name=$1 and formtemplates_id_fk=$2`, name, formtemplateID)
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

// --- nowe formularze (forms_fields.go, forms_manage.go) ---------------------

// FormTemplateSetContent zapisuje definicję pól i pole z kwotą (designer pól).
func (db *DB) FormTemplateSetContent(tmplID, userID int64, content string, moneyfield sql.NullString) error {
	_, err := db.DB.Exec(`UPDATE formtemplates SET content=$1, moneyfield=$2 WHERE id=$3 AND users_id_fk=$4`,
		content, moneyfield, tmplID, userID)
	return err
}

// FormTemplateSetAccess zapisuje język, status i termin zamknięcia formularza.
func (db *DB) FormTemplateSetAccess(tmplID, userID int64, language, status string, closeDate sql.NullInt64) error {
	_, err := db.DB.Exec(`UPDATE formtemplates SET language=$1, status=$2, close_date=$3 WHERE id=$4 AND users_id_fk=$5`,
		language, status, closeDate, tmplID, userID)
	return err
}

// FormTemplateSetPanelMode zapisuje tryb panelu informacyjnego.
// FormTemplateSetPayMessage ustawia początek wiadomości dla odbiorcy przelewu.
func (db *DB) FormTemplateSetPayMessage(tmplID, userID int64, msg string) error {
	_, err := db.DB.Exec(`UPDATE formtemplates SET pay_message=$1 WHERE id=$2 AND users_id_fk=$3`, msg, tmplID, userID)
	return err
}

// FormTemplateSetTheme ustawia temat graficzny strony formularza.
func (db *DB) FormTemplateSetTheme(tmplID, userID int64, theme string) error {
	_, err := db.DB.Exec(`UPDATE formtemplates SET theme=$1 WHERE id=$2 AND users_id_fk=$3`, theme, tmplID, userID)
	return err
}

func (db *DB) FormTemplateSetPanelMode(tmplID, userID int64, mode string) error {
	_, err := db.DB.Exec(`UPDATE formtemplates SET infopanel_mode=$1 WHERE id=$2 AND users_id_fk=$3`, mode, tmplID, userID)
	return err
}

type fieldAnswerRow struct {
	Name  string `db:"name"`
	Value string `db:"value"`
}

// FormAnswerValuesForTemplate zwraca wszystkie odpowiedzi formularza z nazwą
// pola - jedno zapytanie na całe zestawienie (form_stats.go).
func (db *DB) FormAnswerValuesForTemplate(tmplID int64) ([]fieldAnswerRow, error) {
	var rows []fieldAnswerRow
	err := db.DB.Select(&rows, `SELECT ff.name AS name, fa.value AS value FROM formanswers fa
		JOIN formfields ff ON ff.id=fa.formfields_id_fk
		JOIN forms f ON f.id=fa.forms_id_fk
		WHERE ff.formtemplates_id_fk=$1 AND fa.value IS NOT NULL`, tmplID)
	return rows, err
}

type formAnswerRecord struct {
	FormID int64  `db:"form_id"`
	Name   string `db:"name"`
	Value  string `db:"value"`
}

// FormAnswerRecordsForTemplate zwraca odpowiedzi formularza z id zgłoszenia
// i nazwą pola - zestawienia pól zależnych potrzebują całych zgłoszeń
// (form_stats.go).
func (db *DB) FormAnswerRecordsForTemplate(tmplID int64) ([]formAnswerRecord, error) {
	var rows []formAnswerRecord
	err := db.DB.Select(&rows, `SELECT fa.forms_id_fk AS form_id, ff.name AS name, fa.value AS value FROM formanswers fa
		JOIN formfields ff ON ff.id=fa.formfields_id_fk
		JOIN forms f ON f.id=fa.forms_id_fk
		WHERE ff.formtemplates_id_fk=$1 AND fa.value IS NOT NULL`, tmplID)
	return rows, err
}

// FormTemplateURLTaken mówi, czy organizacja ma już formularz pod tym adresem
// (poza formularzem exceptID).
func (db *DB) FormTemplateURLTaken(url string, userID, exceptID int64) bool {
	var n int64
	err := db.DB.Get(&n, `SELECT COUNT(id) FROM formtemplates WHERE url=$1 AND users_id_fk=$2 AND id<>$3`, url, userID, exceptID)
	return err != nil || n > 0
}

// FormFieldRename zmienia nazwę pola, zachowując jego odpowiedzi
// (formanswers wskazują na id pola, nie na nazwę).
func (db *DB) FormFieldRename(tmplID int64, oldName, newName string) error {
	_, err := db.DB.Exec(`UPDATE formfields SET name=$1 WHERE name=$2 AND formtemplates_id_fk=$3`, newName, oldName, tmplID)
	return err
}

// FormFieldAnswerCounts zwraca, ile niepustych odpowiedzi ma każde pole
// (klucz: nazwa pola). Designer ostrzega przed skasowaniem takich pól.
func (db *DB) FormFieldAnswerCounts(tmplID int64) (map[string]int64, error) {
	type row struct {
		Name  string `db:"name"`
		Count int64  `db:"cnt"`
	}
	var rows []row
	err := db.DB.Select(&rows, `SELECT ff.name AS name, COUNT(fa.id) AS cnt FROM formfields ff
		JOIN formanswers fa ON fa.formfields_id_fk=ff.id
		WHERE ff.formtemplates_id_fk=$1 AND fa.value IS NOT NULL AND fa.value<>''
		GROUP BY ff.name`, tmplID)
	out := map[string]int64{}
	for _, r := range rows {
		out[r.Name] = r.Count
	}
	return out, err
}

// FormTemplateDelete kasuje formularz razem z polami, odpowiedziami i
// zgłoszeniami. Dotyczy tylko zwykłych formularzy organizacji (nie dodatków).
func (db *DB) FormTemplateDelete(tmplID, userID int64) error {
	tx, err := db.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var n int64
	if err := tx.Get(&n, `SELECT COUNT(id) FROM formtemplates WHERE id=$1 AND users_id_fk=$2 AND kind<>'order_addons'`, tmplID, userID); err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("FormTemplateDelete: form %d not found", tmplID)
	}
	stmts := []string{
		`DELETE FROM formnotificationslog WHERE forms_id_fk IN (SELECT id FROM forms WHERE formtemplates_id_fk=$1)`,
		`DELETE FROM formtemplates_notifs WHERE formtemplates_id_fk=$1`,
		`DELETE FROM formanswers WHERE forms_id_fk IN (SELECT id FROM forms WHERE formtemplates_id_fk=$1)`,
		`DELETE FROM formanswers WHERE formfields_id_fk IN (SELECT id FROM formfields WHERE formtemplates_id_fk=$1)`,
		`DELETE FROM forms WHERE formtemplates_id_fk=$1`,
		`DELETE FROM formfields WHERE formtemplates_id_fk=$1`,
		`DELETE FROM formtemplates WHERE id=$1`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s, tmplID); err != nil {
			return err
		}
	}
	return tx.Commit()
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

// FormSetOrder spina odpowiedź formularza z zamówieniem (dodatki gościa).
// Osobna metoda, bo FormAdd wypisuje kolumny wprost i nie zna orders_id_fk.
func (db *DB) FormSetOrder(formID, orderID int64) error {
	_, err := db.DB.Exec(`UPDATE forms SET orders_id_fk=$1 WHERE id=$2`, orderID, formID)
	return err
}

// FormAnswerSetRepeat zapisuje numer powtórzenia odpowiedzi (pola dodatków
// powtarzane wg liczby miejsc). 0 = pole niepowtarzane.
func (db *DB) FormAnswerSetRepeat(answerID, repeatIndex int64) error {
	_, err := db.DB.Exec(`UPDATE formanswers SET repeat_index=$1 WHERE id=$2`, repeatIndex, answerID)
	return err
}

// FormSetStatus ustawia status odpowiedzi formularza (np. "payed").
// Stare FormChangeStatusAPI było zaślepką i nic nie zapisywało.
func (db *DB) FormSetStatus(formID, templateID int64, status string) error {
	ret, err := db.DB.Exec(`UPDATE forms SET status=$1 WHERE id=$2 AND formtemplates_id_fk=$3`,
		status, formID, templateID)
	if err != nil {
		return err
	}
	n, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no form %d in template %d", formID, templateID)
	}
	return nil
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

// FormAnswerGetAllAnswersForFieldInts works only for data which can be converted to int64
func (db *DB) FormAnswerGetAllAnswersForFieldInts(FieldID int64) ([]int64, error) {
	fa := []int64{}
	err := db.DB.Select(&fa, `SELECT Value FROM formanswers WHERE formfields_id_fk=$1 AND Value IS NOT NULL AND trim(Value)<>''`, FieldID)
	return fa, err
}

// FormAnswerGetAllAnswersForFieldStrings works only for data which can be converted to int64
func (db *DB) FormAnswerGetAllAnswersForFieldStrings(FieldID int64) ([]string, error) {
	fa := []string{}
	err := db.DB.Select(&fa, `SELECT Value FROM formanswers WHERE formfields_id_fk=$1 AND Value IS NOT NULL AND trim(Value)<>''`, FieldID)
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

func (db *DB) EventNotificationLogAdd(ea *EventNotificationLog) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO eventnotificationslog (date, notifications_id_fk, reservations_id_fk)
	VALUES(:date, :notifications_id_fk, :reservations_id_fk)`, ea)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
}

func (db *DB) FormNotificationLogAdd(fa *FormNotificationLog) (int64, error) {
	ret, err := db.DB.NamedExec(`INSERT INTO formnotificationslog (date, notifications_id_fk, forms_id_fk)
	VALUES(:date, :notifications_id_fk, :forms_id_fk)`, fa)
	if err != nil {
		return -1, err
	}
	return ret.LastInsertId()
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

func (db *DB) Close() {
	db.DB.Close()
}

func NoMinus(n string, i int64) error {
	if i < 0 {
		return fmt.Errorf("wrong input param %q: %d", n, i)
	}
	return nil
}

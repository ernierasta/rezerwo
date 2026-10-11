-- Schemat bazy produkcyjnej (bez danych) dla testów w database_test.go.
-- Odświeżenie po migracji: sqlite3 db.sql .schema | grep -v sqlite_sequence > testdata/schema.sql
CREATE TABLE users (id INTEGER NOT NULL PRIMARY KEY, email TEXT NOT NULL UNIQUE, url TEXT NOT NULL, passwd TEXT NOT NULL, name TEXT, surname TEXT, organization TEXT, phone TEXT, alt_email TEXT);
CREATE TABLE users_rooms (users_id_fk INTEGER NOT NULL, rooms_id_fk INTEGER NOT NULL UNIQUE, FOREIGN KEY(users_id_fk) REFERENCES users(id), FOREIGN KEY(rooms_id_fk) REFERENCES rooms(id));
CREATE TABLE furnitures (id INTEGER NOT NULL PRIMARY KEY, number INTEGER NOT NULL, type TEXT NOT NULL, orientation TEXT, x INTEGER NOT NULL, y INTEGER NOT NULL, width INTEGER, height INTEGER, color TEXT, label TEXT, capacity INTEGER, rooms_id_fk INTEGER NOT NULL, UNIQUE(number, type, rooms_id_fk) ON CONFLICT ROLLBACK,FOREIGN KEY(rooms_id_fk) REFERENCES rooms(id));
CREATE TABLE events_rooms (events_id_fk INTEGER NOT NULL, rooms_id_fk INTEGER NOT NULL, FOREIGN KEY(events_id_fk) REFERENCES events(id), FOREIGN KEY(rooms_id_fk) REFERENCES rooms(id), UNIQUE(events_id_fk, rooms_id_fk) ON CONFLICT ROLLBACK);
CREATE TABLE notes (id INTEGER NOT NULL PRIMARY KEY, text TEXT NOT NULL);
CREATE TABLE reservations (id INTEGER NOT NULL PRIMARY KEY, ordered_date INTEGER, payed_date INTEGER, price INTEGER, currency TEXT, status TEXT NOT NULL, notes_id_fk INTEGER, furnitures_id_fk INTEGER NOT NULL, events_id_fk INTEGER NOT NULL, customers_id_fk INTEGER NOT NULL, orders_id_fk INTEGER, FOREIGN KEY(notes_id_fk) REFERENCES notes(id), FOREIGN KEY(furnitures_id_fk) REFERENCES furnitures(id), FOREIGN KEY(events_id_fk) REFERENCES events(id), FOREIGN KEY(customers_id_fk) REFERENCES customers(id), UNIQUE(furnitures_id_fk, events_id_fk) ON CONFLICT ROLLBACK);
CREATE TABLE users_customers (users_id_fk INTEGER NOT NULL, customers_id_fk INTEGER NOT NULL, FOREIGN KEY(users_id_fk) REFERENCES users(id), FOREIGN KEY(customers_id_fk) REFERENCES customers(id), UNIQUE(users_id_fk, customers_id_fk) ON CONFLICT ROLLBACK);
CREATE TABLE admins (id INTEGER NOT NULL PRIMARY KEY, type TEXT NOT NULL, email TEXT NOT NULL, passwd TEXT, notes TEXT, users_id_fk INTEGER NOT NULL, language TEXT, FOREIGN KEY(users_id_fk) REFERENCES users(id));
CREATE TABLE "prices" (id INTEGER NOT NULL PRIMARY KEY, price INTEGER NOT NULL, currency TEXT NOT NULL, disabled INTEGER NOT NULL, events_id_fk INTEGER NOT NULL, furnitures_id_fk INTEGER NOT NULL, FOREIGN KEY(events_id_fk) REFERENCES events(id), FOREIGN KEY(furnitures_id_fk) REFERENCES furnitures(id), UNIQUE(furnitures_id_fk, events_id_fk) ON CONFLICT ROLLBACK);
CREATE TABLE events (id INTEGER NOT NULL PRIMARY KEY, name TEXT NOT NULL, date INTEGER NOT NULL, from_date INTEGER NOT NULL, to_date INTEGER NOT NULL, default_price INTEGER NOT NULL, default_currency TEXT NOT NULL, no_sits_selected_title TEXT NOT NULL, no_sits_selected_text TEXT NOT NULL, how_to TEXT NOT NULL, order_howto TEXT NOT NULL, order_notes_desc TEXT NOT NULL, ordered_note_title TEXT NOT NULL, ordered_note_text TEXT NOT NULL, users_id_fk INTEGER NOT NULL, thankyou_notifications_id_fk INTEGER REFERENCES notifications(id), admin_notifications_id_fk INTEGER REFERENCES notifications(id), "bankaccounts_id_fk" INTEGER REFERENCES bankaccounts(id), sharable INTEGER, language TEXT, hidecosts INTEGER, mainbanner string, maindesc string, room1desc string, room1banner string, room2desc string, room2banner string, room3desc string, room3banner string, room4desc string, room4banner string, previous_event_id INTEGER, status TEXT NOT NULL DEFAULT 'published', addons_formtemplate_id INTEGER, theme TEXT DEFAULT '', pay_message TEXT NOT NULL DEFAULT '', FOREIGN KEY(users_id_fk) REFERENCES users(id), UNIQUE(name, users_id_fk) ON CONFLICT ROLLBACK);
CREATE TABLE "customers" (
	"id"	INTEGER NOT NULL,
	"email"	TEXT NOT NULL,
	"passwd"	TEXT,
	"name"	TEXT,
	"surname"	TEXT,
	"phone"	TEXT,
	"notes"	TEXT,
	PRIMARY KEY("id")
);
CREATE TABLE "forms" (
	"id"	INTEGER NOT NULL UNIQUE,
	"name"	TEXT,
	"surname"	TEXT,
	"notes"	TEXT,
	"email"	TEXT,
	"created_date"	INTEGER NOT NULL,
	"users_id_fk"	INTEGER NOT NULL,
	"formtemplates_id_fk"	INTEGER NOT NULL, status TEXT, orders_id_fk INTEGER,
	PRIMARY KEY("id" AUTOINCREMENT),
	FOREIGN KEY("formtemplates_id_fk") REFERENCES "formtemplates"("id"),
	UNIQUE("surname","email","formtemplates_id_fk"),
	FOREIGN KEY("users_id_fk") REFERENCES "users"("id")
);
CREATE TABLE "formtemplates" (
	"id"	INTEGER NOT NULL,
	"name"	TEXT NOT NULL,
	"url"	TEXT NOT NULL,
	"banner"	TEXT,
	"howto"	TEXT,
	"content"	TEXT,
	"created_date"	INTEGER NOT NULL,
	"thankyou"	TEXT,
	"users_id_fk"	INTEGER NOT NULL,
	"events_id_fk"	INTEGER, infopanel TEXT, bankaccounts_id_fk INTEGER REFERENCES bankaccounts(id), moneyfield TEXT, notifications_id_fk TEXT, admin_notifications_id_fk INTEGER REFERENCES notifications(id), kind TEXT NOT NULL DEFAULT 'form', language TEXT NOT NULL DEFAULT 'pl', status TEXT NOT NULL DEFAULT 'open', close_date INTEGER, infopanel_mode TEXT NOT NULL DEFAULT 'text', theme TEXT NOT NULL DEFAULT '', pay_message TEXT NOT NULL DEFAULT '',
	FOREIGN KEY("users_id_fk") REFERENCES "users"("id"),
	UNIQUE("url","users_id_fk"),
	FOREIGN KEY("events_id_fk") REFERENCES "events"("id"),
	PRIMARY KEY("id")
);
CREATE TABLE "formnotificationslog" (
	"id"	INTEGER NOT NULL UNIQUE,
	"date"	INTEGER NOT NULL,
	"notifications_id_fk" INTEGER NOT NULL,
	"forms_id_fk"	INTEGER NOT NULL,
	FOREIGN KEY("forms_id_fk") REFERENCES "forms"("id"),
	PRIMARY KEY("id" AUTOINCREMENT)
);
CREATE TABLE "notifications" (
	"id"	INTEGER NOT NULL UNIQUE,
	"name"	TEXT NOT NULL,
	"type"	TEXT NOT NULL,
	"related_to"	TEXT NOT NULL,
	"title"	TEXT,
	"text"	TEXT NOT NULL,
	"embedded_imgs"	TEXT,
	"attached_imgs"	TEXT,
	"sharable"	TEXT,
	"created_date"	INTEGER NOT NULL,
	"updated_date"	INTEGER,
	"users_id_fk"	INTEGER,
	PRIMARY KEY("id" AUTOINCREMENT)
);
CREATE TABLE "formfields" (
	"id"	INTEGER NOT NULL,
	"name"	TEXT NOT NULL,
	"display"	TEXT NOT NULL,
	"type"	TEXT NOT NULL,
	"formtemplates_id_fk"	INTEGER NOT NULL,
	PRIMARY KEY("id" AUTOINCREMENT),
	FOREIGN KEY("formtemplates_id_fk") REFERENCES "formtemplates"("id"),
	UNIQUE("name","formtemplates_id_fk")
);
CREATE TABLE "events_notifs" (
	"events_id_fk"	INTEGER NOT NULL,
	"notifications_id_fk"	INTEGER NOT NULL,
	FOREIGN KEY("events_id_fk") REFERENCES "events"("id")
);
CREATE TABLE "formtemplates_notifs" (
	"formtemplates_id_fk"	INTEGER NOT NULL,
	"notifications_id_fk"	INTEGER NOT NULL,
	FOREIGN KEY("formtemplates_id_fk") REFERENCES "formtemplates"("id"),
	FOREIGN KEY("notifications_id_fk") REFERENCES "notifications"("id")
);
CREATE TABLE "eventsaddons" (
	"id"	INTEGER NOT NULL,
	"name"	INTEGER NOT NULL,
	"price"	INTEGER,
	"currency"	INTEGER,
	"events_id_fk"	INTEGER NOT NULL,
	"users_id_fk"	INTEGER NOT NULL,
	FOREIGN KEY("users_id_fk") REFERENCES "users"("id"),
	FOREIGN KEY("events_id_fk") REFERENCES "events"("id"),
	PRIMARY KEY("id" AUTOINCREMENT)
);
CREATE TABLE "bankaccounts" (
	"id"	INTEGER NOT NULL UNIQUE,
	"name"	TEXT NOT NULL,
	"iban"	TEXT NOT NULL,
	"recipientname"	TEXT,
	"bank"	TEXT,
	"currency"	TEXT NOT NULL,
	"message"	TEXT,
	"varsymbol"	INTEGER,
	"amountfield"	TEXT,
	"users_id_fk"	INTEGER NOT NULL,
	FOREIGN KEY("users_id_fk") REFERENCES "users"("id"),
	PRIMARY KEY("id" AUTOINCREMENT),
	UNIQUE("name","users_id_fk")
);
CREATE TABLE "rooms" (
	"id"	INTEGER NOT NULL,
	"name"	TEXT NOT NULL,
	"description"	TEXT,
	"width"	INTEGER NOT NULL,
	"height"	INTEGER NOT NULL,
	"banner_img"	TEXT,
	"sharable"	INTEGER,
	PRIMARY KEY("id")
);
CREATE TABLE "eventnotificationslog" (
	"id"	INTEGER NOT NULL UNIQUE,
	"date"	INTEGER NOT NULL,
	"notifications_id_fk" INTEGER NOT NULL,
	"reservations_id_fk"	INTEGER NOT NULL,
	FOREIGN KEY("reservations_id_fk") REFERENCES "reservations"("id"),
	PRIMARY KEY("id" AUTOINCREMENT)
);
CREATE TABLE reservation_status_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	date INTEGER NOT NULL,
	status TEXT NOT NULL,
	admin_email TEXT,
	reservations_id_fk INTEGER NOT NULL,
	FOREIGN KEY (reservations_id_fk) REFERENCES reservations(id) ON DELETE CASCADE
);
CREATE INDEX reservation_status_log_res
	ON reservation_status_log (reservations_id_fk);
CREATE TABLE orders (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_date INTEGER NOT NULL,
	notes TEXT,
	events_id_fk INTEGER NOT NULL,
	customers_id_fk INTEGER NOT NULL,
	FOREIGN KEY (events_id_fk) REFERENCES events(id) ON DELETE CASCADE,
	FOREIGN KEY (customers_id_fk) REFERENCES customers(id)
);
CREATE INDEX orders_event ON orders (events_id_fk);
CREATE INDEX orders_customer ON orders (customers_id_fk);
CREATE TABLE order_items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	kind TEXT NOT NULL DEFAULT 'addon',   -- addon albo surcharge (dopłata)
	description TEXT NOT NULL,
	amount INTEGER NOT NULL DEFAULT 1,    -- ile sztuk
	price INTEGER NOT NULL DEFAULT 0,     -- cena za sztukę
	currency TEXT,
	status TEXT NOT NULL DEFAULT 'ordered', -- ordered albo payed
	payed_date INTEGER,
	created_date INTEGER NOT NULL,
	orders_id_fk INTEGER NOT NULL,
	FOREIGN KEY (orders_id_fk) REFERENCES orders(id) ON DELETE CASCADE
);
CREATE INDEX order_items_order ON order_items (orders_id_fk);
CREATE INDEX reservations_order ON reservations (orders_id_fk);
CREATE TABLE "formanswers" (
	"id"	INTEGER NOT NULL UNIQUE,
	"value"	TEXT,
	"formfields_id_fk"	INTEGER NOT NULL,
	"forms_id_fk"	INTEGER NOT NULL,
	repeat_index INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY("forms_id_fk") REFERENCES "forms"("id"),
	PRIMARY KEY("id" AUTOINCREMENT),
	UNIQUE("formfields_id_fk","forms_id_fk",repeat_index)
);

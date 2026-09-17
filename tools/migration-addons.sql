-- Migracja dla zamówień i dodatków do zamówienia (rozdz. 9.5 i 14
-- redesign/HARMONIJKA.md). Uruchamiana RAZ, po tools/migration-redesign.sql.
--
-- Przed uruchomieniem: tools/backup-db
-- użycie: sqlite3 db.sql < tools/migration-addons.sql
--
-- UWAGA: kod Go musi mieć już pola na nowe kolumny (structs w database.go),
-- bo zapytania SELECT * wymagają, żeby każda kolumna miała swoje pole.

-- 1. Zamówienie jako byt: dane osoby są dalej w customers, tu jest spinacz
--    między klientem, imprezą i pozycjami.
CREATE TABLE IF NOT EXISTS orders (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_date INTEGER NOT NULL,
	notes TEXT,
	events_id_fk INTEGER NOT NULL,
	customers_id_fk INTEGER NOT NULL,
	FOREIGN KEY (events_id_fk) REFERENCES events(id) ON DELETE CASCADE,
	FOREIGN KEY (customers_id_fk) REFERENCES customers(id)
);
CREATE INDEX IF NOT EXISTS orders_event ON orders (events_id_fk);
CREATE INDEX IF NOT EXISTS orders_customer ON orders (customers_id_fk);

-- 2. Pozycje zamówienia: TYLKO dodatki i dopłaty. Miejsca zostają w
--    reservations.status - to dalej jedyne źródło prawdy o stanie miejsca.
CREATE TABLE IF NOT EXISTS order_items (
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
CREATE INDEX IF NOT EXISTS order_items_order ON order_items (orders_id_fk);

-- 3. Powiązania: rezerwacja należy do zamówienia, odpowiedź formularza
--    (dodatki) też; definicja dodatków to szablon formularza przypięty do imprezy.
ALTER TABLE reservations ADD COLUMN orders_id_fk INTEGER;
ALTER TABLE forms ADD COLUMN orders_id_fk INTEGER;
ALTER TABLE formtemplates ADD COLUMN kind TEXT NOT NULL DEFAULT 'form';
ALTER TABLE events ADD COLUMN addons_formtemplate_id INTEGER;
ALTER TABLE formanswers ADD COLUMN repeat_index INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS reservations_order ON reservations (orders_id_fk);

-- 4. Odtworzenie zamówień z istniejących rezerwacji: jedno zamówienie na
--    klienta, imprezę i dzień złożenia (rozdz. 14). Rezerwacje bez klienta
--    (marked z -1) zostają bez zamówienia - i tak kasuje je ticker.
INSERT INTO orders (created_date, events_id_fk, customers_id_fk)
SELECT MIN(COALESCE(ordered_date, 0)), events_id_fk, customers_id_fk
	FROM reservations
	WHERE customers_id_fk > 0
	GROUP BY events_id_fk, customers_id_fk, date(COALESCE(ordered_date, 0), 'unixepoch');

UPDATE reservations SET orders_id_fk = (
	SELECT o.id FROM orders o
	WHERE o.events_id_fk = reservations.events_id_fk
		AND o.customers_id_fk = reservations.customers_id_fk
		AND date(o.created_date, 'unixepoch') = date(COALESCE(reservations.ordered_date, 0), 'unixepoch')
) WHERE customers_id_fk > 0;

-- Sprawdzenie po migracji (powinno zwrócić 0):
-- SELECT count(*) FROM reservations WHERE customers_id_fk > 0 AND orders_id_fk IS NULL;

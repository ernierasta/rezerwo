-- Uprawnienia menedżerów i dopisywanie osób przez menedżera
-- (access.go, managers_ui.go, form_fill.go).
-- Uruchamiana RAZ, przy wdrożeniu, przed restartem serwera.
-- Przed uruchomieniem: tools/backup-db
--
-- użycie: sqlite3 db.sql < tools/migration-managerperms.sql
-- (powtórzenie zgłosi "duplicate column name" i nic nie zmieni)

BEGIN;

-- 1. Uprawnienia menedżera: wszystkie imprezy i formularze (1) albo tylko
--    wybrane z manager_access (0); zarządzanie organizacją: sale, maile,
--    konta, katalog, nowe imprezy i formularze. Obecni menedżerowie mają oba,
--    więc dla nich nic się nie zmienia.
ALTER TABLE admins ADD COLUMN all_access INTEGER NOT NULL DEFAULT 1;
ALTER TABLE admins ADD COLUMN org_manage INTEGER NOT NULL DEFAULT 1;

-- 2. Wybrane imprezy (kind='event') i formularze (kind='form') menedżera;
--    level: 'results' (obsługa - rezerwacje, odpowiedzi, raporty, bez zmian
--    ustawień) albo 'full'.
CREATE TABLE IF NOT EXISTS manager_access (
	admins_id_fk INTEGER NOT NULL,
	kind TEXT NOT NULL,
	item_id INTEGER NOT NULL,
	level TEXT NOT NULL,
	PRIMARY KEY (admins_id_fk, kind, item_id),
	FOREIGN KEY (admins_id_fk) REFERENCES admins(id) ON DELETE CASCADE
);

-- 3. Kto dopisał zgłoszenie / odpowiedź: NULL = gość, 0 = konto organizacji,
--    >0 = admins.id menedżera.
ALTER TABLE forms ADD COLUMN added_by INTEGER;
ALTER TABLE formanswers ADD COLUMN added_by INTEGER;

-- 4. Mail do osób dopisanych przez menedżera: włączony (domyślnie) i szablon
--    (NULL = ten sam, co dostaje gość wypełniający sam).
ALTER TABLE formtemplates ADD COLUMN mgr_mail INTEGER NOT NULL DEFAULT 1;
ALTER TABLE formtemplates ADD COLUMN mgr_notifications_id_fk INTEGER REFERENCES notifications(id);

-- 5. Osoby wpisane przez menedżera - podpowiedzi w każdym formularzu.
--    admins_id = 0 dla konta organizacji.
CREATE TABLE IF NOT EXISTS manager_contacts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	users_id_fk INTEGER NOT NULL,
	admins_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	surname TEXT NOT NULL,
	email TEXT NOT NULL,
	used INTEGER NOT NULL,
	UNIQUE (users_id_fk, admins_id, email)
);

COMMIT;

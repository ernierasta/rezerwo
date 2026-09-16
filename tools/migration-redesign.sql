-- Migracja bazy dla redesignu (rozdz. 14 redesign/HARMONIJKA.md).
-- Uruchamiana RAZ, przy wdrożeniu, na sprawdzonej kopii bazy.
-- Przed uruchomieniem: tools/backup-db
--
-- użycie: sqlite3 db.sql < tools/migration-redesign.sql
--
-- Skrypt jest podzielony na części; każda jest bezpieczna do powtórzenia tylko
-- raz (SQLite nie ma ADD COLUMN IF NOT EXISTS - powtórzenie zgłosi błąd
-- "duplicate column name" i nic nie zmieni).

-- 1. Porównanie edycji imprez (dashboard, raporty).
ALTER TABLE events ADD COLUMN previous_event_id INTEGER;

-- 2. Szkice: impreza widoczna dla gości dopiero po opublikowaniu.
ALTER TABLE events ADD COLUMN status TEXT NOT NULL DEFAULT 'published';

-- 3. Język panelu administracyjnego (dziś z ciasteczka, docelowo z konta).
ALTER TABLE admins ADD COLUMN language TEXT;

-- 4. Historia zmian statusu rezerwacji (panel szczegółów, cofanie).
CREATE TABLE IF NOT EXISTS reservation_status_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	date INTEGER NOT NULL,
	status TEXT NOT NULL,
	admin_email TEXT,
	reservations_id_fk INTEGER NOT NULL,
	FOREIGN KEY (reservations_id_fk) REFERENCES reservations(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS reservation_status_log_res
	ON reservation_status_log (reservations_id_fk);

-- 5. Porządki po starym panelu: data zapłaty 1970-01-01 (wpisywana przez
--    przeglądarkę przy cofaniu statusu) to w rzeczywistości "nie zapłacono".
--    Nowy panel zapisuje w takiej sytuacji NULL.
UPDATE reservations SET payed_date = NULL
	WHERE payed_date IS NOT NULL AND payed_date < 86400;
UPDATE reservations SET payed_date = NULL
	WHERE status != 'payed' AND payed_date IS NOT NULL;

-- Tabele orders / order_items (rozdz. 9.5) powstaną osobnym skryptem razem
-- z etapem 5b - dopóki nie ma kodu, który ich używa, nie zakładamy ich.

-- Loginy menedżerów organizacji (auth.go, managers_ui.go).
-- Menedżer = wiersz w admins z type='manager': własny e-mail i hasło (bcrypt),
-- users_id_fk = organizacja, notes = opis (imię, funkcja).
-- Uruchamiana RAZ, przy wdrożeniu, przed restartem serwera.
-- Przed uruchomieniem: tools/backup-db
--
-- użycie: sqlite3 db.sql < tools/migration-managers.sql
-- (powtórzenie zgłosi "duplicate column name" i nic nie zmieni)

-- 1. Ostatnie logowanie - widać na liście menedżerów.
ALTER TABLE admins ADD COLUMN last_login INTEGER;

-- 2. Jeden login = jeden e-mail (bez względu na wielkość liter).
CREATE UNIQUE INDEX IF NOT EXISTS admins_email ON admins (email COLLATE NOCASE);

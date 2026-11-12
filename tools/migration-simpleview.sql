-- Uproszczony widok menedżera: na stronie startowej najpierw wielki przycisk
-- "Dodaj osoby do funkcji", dopiero pod nim imprezy i formularze
-- (access.go, managers_ui.go, admin_ui.go).
-- Uruchamiana RAZ, przy wdrożeniu, przed restartem serwera.
-- Przed uruchomieniem: tools/backup-db
--
-- użycie: sqlite3 db.sql < tools/migration-simpleview.sql
-- (powtórzenie zgłosi "duplicate column name" i nic nie zmieni)

BEGIN;

ALTER TABLE admins ADD COLUMN simple_view INTEGER NOT NULL DEFAULT 0;

COMMIT;

-- Nagłówek strony gościa imprezy i formularza (pagehead.go).
--
-- events.pagehead i formtemplates.pagehead - JSON z wyświetlaną nazwą
-- (pusta = nazwa imprezy/formularza), jej wyrównaniem i linijką "nad
-- nazwą" z kolorem, wielkością i formatowaniem. Pusty = wszystko domyślne,
-- więc istniejące strony wyglądają jak dotąd.
--
-- użycie: sqlite3 db.sql < tools/migration-pagehead.sql

BEGIN;
ALTER TABLE events ADD COLUMN pagehead TEXT NOT NULL DEFAULT '';
ALTER TABLE formtemplates ADD COLUMN pagehead TEXT NOT NULL DEFAULT '';
COMMIT;

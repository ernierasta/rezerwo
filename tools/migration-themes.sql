-- Tematy graficzne stron dla gości (themes.go).
--
-- events.theme i formtemplates.theme - klucz tematu: pastel, piasek, szalwia
-- (jasne), klasyczny (kolory starej wersji), noc, grafit, sliwka (ciemne).
-- Pusty = klasyczny.
--
-- użycie: sqlite3 db.sql < tools/migration-themes.sql

BEGIN;
ALTER TABLE events ADD COLUMN theme TEXT DEFAULT '';
ALTER TABLE formtemplates ADD COLUMN theme TEXT NOT NULL DEFAULT '';
COMMIT;

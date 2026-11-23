-- Sekcja "Sponsorzy" imprezy (sponsors.go, js/n_sponsors.js).
--
-- events.sponsors: JSON z logo w wierszach, tytułem, wielkością i stronami,
-- na których sekcja jest widoczna; pusto = brak sekcji (nic się nie zmienia).
--
-- użycie: sqlite3 db.sql < tools/migration-sponsors.sql

BEGIN;
ALTER TABLE events ADD COLUMN sponsors TEXT NOT NULL DEFAULT '';
COMMIT;

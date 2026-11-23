-- Dodatki do zamówienia we własnej sekcji albo bez niej (guest_order.go,
-- tmpl/n_order.html).
--
-- events.addons_section = 1: dodatki w ramce z tytułem "Dodatki do
-- zamówienia" (jak dotąd), 0: pola zaraz pod danymi osoby, bez tytułu.
-- Istniejące imprezy zostają z sekcją, nic się nie zmienia.
--
-- użycie: sqlite3 db.sql < tools/migration-addonsection.sql

BEGIN;
ALTER TABLE events ADD COLUMN addons_section INTEGER NOT NULL DEFAULT 1;
COMMIT;

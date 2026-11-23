-- Miejsce dodatków do zamówienia w formularzu zamówienia (guest_order.go,
-- tmpl/n_order.html, AddonsPositions w addons.go).
--
-- events.addons_after: po którym polu danych osoby stoją dodatki - start
-- (przed e-mailem), email, name (imię i nazwisko), phone albo pusto = na
-- końcu, po uwagach (jak dotąd, nic się nie zmienia).
--
-- użycie: sqlite3 db.sql < tools/migration-addonsafter.sql

BEGIN;
ALTER TABLE events ADD COLUMN addons_after TEXT NOT NULL DEFAULT '';
COMMIT;

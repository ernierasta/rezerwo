-- Mail formularza należy do jednego formularza (notifications.formtemplates_id_fk).
--
-- Edytor maila podpowiada pola tego formularza, a okno "Nowy mail" tworzy
-- mail od razu z jego polami. Istniejące maile formularzy przypisujemy do
-- formularza, który ich używa (najnowszego, jeśli kilku - np. po skopiowaniu formularza na nowy rok); nieużywane
-- zostają bez formularza - admin wybierze go przy następnym zapisie.
-- formtemplates.notifications_id_fk jest w bazie typu TEXT, stąd CAST.
--
-- użycie: sqlite3 db.sql < tools/migration-mailform.sql

BEGIN;
ALTER TABLE notifications ADD COLUMN formtemplates_id_fk INTEGER REFERENCES formtemplates(id);
UPDATE notifications SET formtemplates_id_fk = (
	SELECT MAX(f.id) FROM formtemplates f
	WHERE f.users_id_fk = notifications.users_id_fk
	  AND (CAST(f.notifications_id_fk AS INTEGER) = notifications.id OR f.admin_notifications_id_fk = notifications.id)
) WHERE IFNULL(related_to, '') <> 'events';
COMMIT;

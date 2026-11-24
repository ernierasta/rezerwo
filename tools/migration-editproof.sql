-- Poprawianie zgłoszenia formularza tylko z dowodem (form_edit.go).
--
-- formtemplates.edit_proof: 1 = ponowne wysłanie z tym samym adresem e-mail
-- tylko z linku z maila albo z przeglądarki, z której wysłano zgłoszenie
-- (dawne zachowanie); 0 = sam adres e-mail wystarczy (domyślnie). 0 dla
-- wszystkich, bo tak ma być domyślnie.
--
-- Uwaga: formtemplates czytamy przez SELECT *, więc stary program nie
-- znajdzie miejsca na nową kolumnę - uruchomić tuż przed restartem nowego.
--
-- użycie: sqlite3 db.sql < tools/migration-editproof.sql

BEGIN;
ALTER TABLE formtemplates ADD COLUMN edit_proof INTEGER NOT NULL DEFAULT 0;
COMMIT;

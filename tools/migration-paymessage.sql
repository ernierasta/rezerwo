-- Wiadomość dla odbiorcy przelewu na poziomie imprezy i formularza (pay.go).
--
-- events.pay_message i formtemplates.pay_message - początek wiadomości dla
-- odbiorcy (w kodzie QR i w danych do przelewu), dopisywane jest do niego
-- nazwisko i imię. bankaccounts.message nie jest już używane - istniejące
-- imprezy i formularze przejmują wiadomość ze swojego konta, żeby nic się
-- nie zmieniło.
--
-- użycie: sqlite3 db.sql < tools/migration-paymessage.sql

BEGIN;
ALTER TABLE events ADD COLUMN pay_message TEXT NOT NULL DEFAULT '';
UPDATE events SET pay_message = COALESCE((SELECT message FROM bankaccounts WHERE bankaccounts.id = events.bankaccounts_id_fk), '');
ALTER TABLE formtemplates ADD COLUMN pay_message TEXT NOT NULL DEFAULT '';
UPDATE formtemplates SET pay_message = COALESCE((SELECT message FROM bankaccounts WHERE bankaccounts.id = formtemplates.bankaccounts_id_fk), '');
COMMIT;

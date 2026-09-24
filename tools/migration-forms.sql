-- Migracja dla nowych formularzy (rozdz. 7 redesign/HARMONIJKA.md).
-- Uruchamiana RAZ, po tools/migration-redesign.sql i tools/migration-addons.sql.
--
-- Przed uruchomieniem: tools/backup-db
-- użycie: sqlite3 db.sql < tools/migration-forms.sql
--
-- UWAGA: kod Go musi mieć już pola na nowe kolumny (FormTemplate w
-- database.go), bo zapytania SELECT * wymagają pola dla każdej kolumny.

-- Język strony formularza dla gościa (przyciski, komunikaty walidacji).
-- Wszystkie dotychczasowe formularze są po polsku.
ALTER TABLE formtemplates ADD COLUMN language TEXT NOT NULL DEFAULT 'pl';

-- Formularz otwarty albo zamknięty (zamknięty nie przyjmuje odpowiedzi).
ALTER TABLE formtemplates ADD COLUMN status TEXT NOT NULL DEFAULT 'open';

-- Opcjonalny termin: po tej chwili (unix) formularz sam przestaje przyjmować
-- odpowiedzi. NULL = bez terminu.
ALTER TABLE formtemplates ADD COLUMN close_date INTEGER;

-- Tryb panelu informacyjnego: none / auto / fields / text (form_stats.go).
-- Dotychczasowe formularze mają własny tekst.
ALTER TABLE formtemplates ADD COLUMN infopanel_mode TEXT NOT NULL DEFAULT 'text';

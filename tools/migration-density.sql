-- Odstępy między polami strony formularza gościa (density.go).
--
-- formtemplates.density: "" = zwykłe, "compact" = mniejsze, "tight" =
-- najmniejsze. Puste dla wszystkich, więc istniejące formularze wyglądają
-- jak dotąd.
--
-- Uwaga: formtemplates czytamy przez SELECT *, więc stary program nie
-- znajdzie miejsca na nową kolumnę - uruchomić tuż przed restartem nowego.
--
-- użycie: sqlite3 db.sql < tools/migration-density.sql

BEGIN;
ALTER TABLE formtemplates ADD COLUMN density TEXT NOT NULL DEFAULT '';
COMMIT;

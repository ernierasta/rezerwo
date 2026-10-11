-- Dodatki powtarzane wg liczby miejsc ("Kolacja 1", "Kolacja 2"): każde
-- powtórzenie to osobna odpowiedź tego samego pola w jednym zgłoszeniu.
-- UNIQUE(formfields_id_fk, forms_id_fk) przyjmował tylko pierwszą, więc
-- klucz dostaje repeat_index. Na produkcji (2026-10) nie ma jeszcze żadnych
-- dodatków, dane się nie zmieniają.
--
-- użycie (przy zatrzymanym serwerze): sqlite3 db.sql < tools/migration-perseat.sql

PRAGMA foreign_keys=off;
BEGIN;
CREATE TABLE "formanswers_new" (
	"id"	INTEGER NOT NULL UNIQUE,
	"value"	TEXT,
	"formfields_id_fk"	INTEGER NOT NULL,
	"forms_id_fk"	INTEGER NOT NULL,
	repeat_index INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY("forms_id_fk") REFERENCES "forms"("id"),
	PRIMARY KEY("id" AUTOINCREMENT),
	UNIQUE("formfields_id_fk","forms_id_fk",repeat_index)
);
INSERT INTO formanswers_new (id, value, formfields_id_fk, forms_id_fk, repeat_index)
	SELECT id, value, formfields_id_fk, forms_id_fk, repeat_index FROM formanswers;
DROP TABLE formanswers;
ALTER TABLE formanswers_new RENAME TO formanswers;
COMMIT;
PRAGMA foreign_keys=on;

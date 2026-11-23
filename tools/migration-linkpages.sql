-- Strony z linkami (linkpages.go): jedna strona organizacji z bannerem,
-- nagłówkiem, tekstem i kilkoma przyciskami do rezerwacji, formularzy albo
-- dowolnych adresów - pod nią kod QR na plakaty.
--
-- buttons - JSON [{"kind":"event|form|url","id":12,"url":"…","label":"…"}],
-- banner i pagehead - jak w events/formtemplates (banner.go, pagehead.go).
--
-- użycie: sqlite3 db.sql < tools/migration-linkpages.sql

BEGIN;
CREATE TABLE linkpages (
	id INTEGER NOT NULL PRIMARY KEY,
	users_id_fk INTEGER NOT NULL REFERENCES users(id),
	name TEXT NOT NULL,
	url TEXT NOT NULL,
	language TEXT NOT NULL DEFAULT 'pl',
	theme TEXT NOT NULL DEFAULT '',
	banner TEXT NOT NULL DEFAULT '',
	pagehead TEXT NOT NULL DEFAULT '',
	text TEXT NOT NULL DEFAULT '',
	buttons TEXT NOT NULL DEFAULT '',
	UNIQUE(users_id_fk, url)
);
COMMIT;

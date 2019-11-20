DROP TABLE mails;
PRAGMA foreign_keys=off;
DROP table events;
CREATE TABLE IF NOT EXISTS prices_new (id INTEGER NOT NULL PRIMARY KEY, price INTEGER NOT NULL, currency TEXT NOT NULL, disabled INTEGER NOT NULL, events_id_fk INTEGER NOT NULL, furnitures_id_fk INTEGER NOT NULL, FOREIGN KEY(events_id_fk) REFERENCES events(id), FOREIGN KEY(furnitures_id_fk) REFERENCES furnitures(id), UNIQUE(furnitures_id_fk, events_id_fk) ON CONFLICT ROLLBACK);
INSERT INTO prices_new(id, price, currency, disabled, events_id_fk, furnitures_id_fk)
SELECT id, price, currency, disabled, events_id_fk, furnitures_id_fk
FROM prices;
DROP TABLE prices;
ALTER TABLE prices_new RENAME TO prices;
PRAGMA foreign_keys=on;
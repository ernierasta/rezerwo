-- Historia ręcznych zmian schematu i przydatne zapytania (dawniej komentarz
-- w database.go). Wszystko już wykonane na produkcji - tylko do wglądu.

-- QUERIES which may be helpful in the future:

-- Add prices for given room:

-- INSERT INTO prices (price, currency, disabled, events_id_fk, furnitures_id_fk)
-- SELECT 650, "Kč", 0, 6, furnitures.id
-- FROM furnitures
-- WHERE furnitures.rooms_id_fk = 4

-- Check which prices are defined:

-- SELECT * FROM prices p
-- JOIN furnitures f ON f.id = p.furnitures_id_fk
-- WHERE
-- p.events_id_fk = 6
-- AND f.rooms_id_fk = 4

-- Example of alter table:
-- ALTER TABLE users
--   ADD COLUMN alt_email TEXT;

-- ALTER TABLE events
--   ADD COLUMN mail_attachments TEXT;

-- ALTER TABLE formtemplates
--   ADD COLUMN infopanel TEXT;

-- ALTER TABLE formtemplates
--   ADD COLUMN bankaccounts_id_fk INTEGER REFERENCES bankaccounts(id);

-- ALTER TABLE formtemplates
--   ADD COLUMN moneyfield TEXT;
-- ALTER TABLE formtemplates
--   ADD COLUMN thankyoumailsubject TEXT;
-- ALTER TABLE formtemplates
--   ADD COLUMN thankyoumailtext TEXT;

-- ALTER TABLE events
--   ADD COLUMN mail_embeded_imgs TEXT;

-- ALTER TABLE forms
--   ADD COLUMN status TEXT;

-- ALTER TABLE formtemplates
--   ADD COLUMN notifications_id_fk TEXT;

-- Mods for notifications integration

-- ALTER TABLE events
--   ADD COLUMN thankyou_notifications_id_fk INTEGER REFERENCES notifications(id);

-- ALTER TABLE events
--   ADD COLUMN admin_notifications_id_fk INTEGER REFERENCES notifications(id);

-- ALTER TABLE events
--   ADD COLUMN "bankaccounts_id_fk" INTEGER REFERENCES bankaccounts(id);

-- ALTER TABLE rooms
-- 	ADD COLUMN sharable INTEGER;

-- ALTER TABLE events
-- 	ADD COLUMN sharable INTEGER;

-- copy customers mail to notifications table
-- INSERT INTO notifications (name, type, related_to, title, text, embedded_imgs, attached_imgs, sharable, created_date, users_id_fk)
-- SELECT name, 'mail', 'events', mail_subject, mail_text, mail_embeded_imgs, mail_attachments, 0, 1711746719, users_id_fk
-- FROM events

-- copy admin mail to notifications table
-- INSERT INTO notifications (name, type, related_to, title, text, embedded_imgs, attached_imgs, sharable, created_date, users_id_fk)
--  SELECT name||' (admin)', 'mail', 'events', admin_mail_subject, admin_mail_text, mail_embeded_imgs, mail_attachments, 0, 1711746719, users_id_fk
-- FROM events

-- link mails to events
-- UPDATE events SET
-- thankyou_notifications_id_fk=(SELECT id FROM notifications WHERE events.name = notifications.name),
-- admin_notifications_id_fk=(SELECT id FROM notifications WHERE events.name||' (admin)' = notifications.name)

-- remove mails from events table
-- ALTER TABLE events DROP COLUMN mail_subject;
-- ALTER TABLE events DROP COLUMN mail_text;
-- ALTER TABLE events DROP COLUMN admin_mail_subject;
-- ALTER TABLE events DROP COLUMN admin_mail_text;
-- ALTER TABLE events DROP COLUMN mail_attachments;
-- ALTER TABLE events DROP COLUMN mail_embeded_imgs;

-- copy thankyou subject/text from formtemplates to notifications
-- INSERT INTO notifications (name, type, related_to, title, text, embedded_imgs, attached_imgs, sharable, created_date, users_id_fk)
-- SELECT name||' (form)', 'mail', 'forms', thankyoumailsubject, thankyoumailtext, '', '', 0, 1711746719, users_id_fk

-- FROM formtemplates

-- remove subject/text from formtemplates
-- 
-- ALTER TABLE formtemplates DROP COLUMN thankyoumailsubject;
-- ALTER TABLE formtemplates DROP COLUMN thankyoumailtext;
-- 
-- 
-- END

-- Get form data:
-- SELECT display, value FROM formanswers
-- JOIN formfields ON (formanswers.formfields_id_fk=formfields.id)
-- WHERE forms_id_fk=4

-- FormAnswers:
-- SELECT f.name, f.surname, ff.display, fa.value FROM forms f
-- JOIN formanswers fa ON fa.forms_id_fk=f.id
-- JOIN formfields ff ON fa.formfields_id_fk=ff.id
-- WHERE ff.display != "Nazwisko"

-- SELECT display, SUM(fa.value) FROM forms f
-- JOIN formanswers fa ON fa.forms_id_fk=f.id
-- JOIN formfields ff ON fa.formfields_id_fk=ff.id
-- WHERE ff.display != "Nazwisko" AND ff.display != "E-mail" AND display != "Imię dziecka"
-- AND display != "Klasa/przedszkole"
-- GROUP BY display

-- Renumbering tables:
-- UPDATE furnitures
-- SET number = number + 20
-- WHERE rooms_id_fk = 4 AND type = "table"

-- update to enable language switch - 10.2025
-- ALTER TABLE events
--   ADD COLUMN language TEXT;

-- update events to enable total costs hidding
-- it is possible to set it only directly in DB
-- ALTER TABLE events
--   ADD COLUMN hidecosts INTEGER;

-- update events to contain rooms descriptions and banners
-- ALTER TABLE events ADD COLUMN mainbanner string;
-- ALTER TABLE events ADD COLUMN maindesc string;
-- ALTER TABLE events ADD COLUMN room1desc string;
-- ALTER TABLE events ADD COLUMN room1banner string;
-- ALTER TABLE events ADD COLUMN room2desc string;
-- ALTER TABLE events ADD COLUMN room2banner string;
-- ALTER TABLE events ADD COLUMN room3desc string;
-- ALTER TABLE events ADD COLUMN room3banner string;
-- ALTER TABLE events ADD COLUMN room4desc string;
-- ALTER TABLE events ADD COLUMN room4banner string;

-- add new table:
-- CREATE TABLE "eventnotificationslog" (
-- 	"id"	INTEGER NOT NULL UNIQUE,
-- 	"date"	INTEGER NOT NULL,
-- 	"notifications_id_fk" INTEGER NOT NULL,
-- 	"reservations_id_fk"	INTEGER NOT NULL,
-- 	FOREIGN KEY("reservations_id_fk") REFERENCES "reservations"("id"),
-- 	PRIMARY KEY("id" AUTOINCREMENT)
-- )

-- remove all chairs with round number (10,20,30, ...)
-- DELETE FROM prices
-- WHERE furnitures_id_fk IN (
--     SELECT id FROM furnitures
--     WHERE rooms_id_fk = 14
--     AND type = 'chair'
--     AND CAST(number AS TEXT) LIKE '%0'
-- );
-- 
-- then:
-- 
-- DELETE FROM furnitures
-- WHERE rooms_id_fk = 14
-- AND type = 'chair'
-- AND CAST(number AS TEXT) LIKE '%0';
-- 
-- and then Renumber chair button in administration makes wonders!

-- Przeniesienie opisów i bannerów z pomieszczeń (rooms) do imprez (events).
--
-- Strona rezerwacji brała rooms.description / rooms.banner_img, gdy odpowiednie
-- pole events.roomNdesc / events.roomNbanner było puste. Ten fallback został
-- usunięty z kodu, więc PRZED wdrożeniem nowej wersji trzeba skopiować te
-- wartości do pustych pól imprez - strona będzie wyglądała tak samo jak dotąd.
--
-- Pomieszczenie N = N-te pomieszczenie imprezy wg rosnącego id (tak zwraca
-- EventGetRooms). Wypełniane są tylko puste pola, niczego nie nadpisujemy.
--
-- użycie: sqlite3 db.sql < tools/migration-event-desc.sql

BEGIN;

CREATE TEMP TABLE er_ranked AS
SELECT er.events_id_fk AS ev, r.description AS d, r.banner_img AS b,
  (SELECT count(*) FROM events_rooms er2 JOIN rooms r2 ON r2.id = er2.rooms_id_fk
   WHERE er2.events_id_fk = er.events_id_fk AND er2.rooms_id_fk <= er.rooms_id_fk) AS rn
FROM events_rooms er JOIN rooms r ON r.id = er.rooms_id_fk;

UPDATE events SET room1desc = (SELECT d FROM er_ranked WHERE ev = events.id AND rn = 1)
WHERE coalesce(room1desc, '') = '' AND EXISTS (SELECT 1 FROM er_ranked WHERE ev = events.id AND rn = 1 AND coalesce(d, '') <> '');
UPDATE events SET room2desc = (SELECT d FROM er_ranked WHERE ev = events.id AND rn = 2)
WHERE coalesce(room2desc, '') = '' AND EXISTS (SELECT 1 FROM er_ranked WHERE ev = events.id AND rn = 2 AND coalesce(d, '') <> '');
UPDATE events SET room3desc = (SELECT d FROM er_ranked WHERE ev = events.id AND rn = 3)
WHERE coalesce(room3desc, '') = '' AND EXISTS (SELECT 1 FROM er_ranked WHERE ev = events.id AND rn = 3 AND coalesce(d, '') <> '');
UPDATE events SET room4desc = (SELECT d FROM er_ranked WHERE ev = events.id AND rn = 4)
WHERE coalesce(room4desc, '') = '' AND EXISTS (SELECT 1 FROM er_ranked WHERE ev = events.id AND rn = 4 AND coalesce(d, '') <> '');

UPDATE events SET room1banner = (SELECT b FROM er_ranked WHERE ev = events.id AND rn = 1)
WHERE coalesce(room1banner, '') = '' AND EXISTS (SELECT 1 FROM er_ranked WHERE ev = events.id AND rn = 1 AND coalesce(b, '') <> '');
UPDATE events SET room2banner = (SELECT b FROM er_ranked WHERE ev = events.id AND rn = 2)
WHERE coalesce(room2banner, '') = '' AND EXISTS (SELECT 1 FROM er_ranked WHERE ev = events.id AND rn = 2 AND coalesce(b, '') <> '');
UPDATE events SET room3banner = (SELECT b FROM er_ranked WHERE ev = events.id AND rn = 3)
WHERE coalesce(room3banner, '') = '' AND EXISTS (SELECT 1 FROM er_ranked WHERE ev = events.id AND rn = 3 AND coalesce(b, '') <> '');
UPDATE events SET room4banner = (SELECT b FROM er_ranked WHERE ev = events.id AND rn = 4)
WHERE coalesce(room4banner, '') = '' AND EXISTS (SELECT 1 FROM er_ranked WHERE ev = events.id AND rn = 4 AND coalesce(b, '') <> '');

DROP TABLE er_ranked;

COMMIT;

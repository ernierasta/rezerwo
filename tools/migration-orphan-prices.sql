-- Ceny usuniętych mebli: do 2026-10 designer kasował mebel, ale jego cen nie
-- (PriceDelByEventIDFurn pytał o nieistniejące kolumny). Wiersze niczemu nie
-- szkodzą, tylko zaśmiecają tabelę. Opcjonalne, można uruchomić w każdej chwili:
--   sqlite3 db.sql < tools/migration-orphan-prices.sql
DELETE FROM prices WHERE furnitures_id_fk NOT IN (SELECT id FROM furnitures);

PRAGMA foreign_keys=off;

ALTER TABLE formfields RENAME TO formfields_old;

 CREATE TABLE "formfields" (
	"id"	INTEGER NOT NULL,
	"name"	TEXT NOT NULL,
	"display"	TEXT NOT NULL,
	"type"	TEXT NOT NULL,
	"formtemplates_id_fk"	INTEGER NOT NULL,
	PRIMARY KEY("id" AUTOINCREMENT),
	FOREIGN KEY("formtemplates_id_fk") REFERENCES "formtemplates"("id"),
	UNIQUE("name","formtemplates_id_fk")
);

INSERT INTO formfields SELECT * FROM formfields_old;

PRAGMA foreign_keys=on;

-- remove old table after checking it is ok

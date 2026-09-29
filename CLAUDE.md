# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Rezerwo: seat-reservation + online-forms system for Polish minority organizations in the Czech Republic (production at zori.cz). Single Go binary (`package main`, module `github.com/ernierasta/rezerwo`), SQLite, server-rendered `html/template` pages with plain JS and own SCSS (no Bootstrap; jQuery UI only in the room designer). UI text and comments are largely Polish.

## Commands

```sh
CGO_ENABLED=0 go build -v .   # build (as CI does); pure-Go sqlite (modernc.org/sqlite), no cgo needed
./rezerwo                      # run; listens on :3002, must run from repo root (relative paths)
tools/watch.sh                 # dev: sass watcher (scss -> css) + air hot reload (.air.conf)
tools/sass scss:css --no-source-map   # compile styles once
go run tools/securecookie.go   # generate AuthenticationKey / EncryptionKey for config.toml
tools/backup-db                # copy db.sql into backup/
```

Tests: only `mail_test.go`, an integration test that sends real mail:
`MSERVER= MUSER= MPASS= MTO= go test -run TestSendMail`. `database_test.go.off` is intentionally disabled.

## Runtime requirements

The binary resolves everything relative to the working directory: `config.toml` (gitignored; Location, cookie keys, SMTP settings), `db.sql` (gitignored SQLite DB — `initDB` hardcodes `"db.sql"`, the `DBFile` setting is ignored), `tmpl/`, `js/`, `css/`, `img/`, `media/`.

`media/<user-url>/` holds per-organization user data (subdirs `email/` for embedded mail images/attachments, `qrcode/` generated payment QR codes, `pdf/` tickets); `media/fonts/` has NotoSans used for PDF tickets. `media/` is gitignored.

## Architecture

- `main.go`: `main()` wires all routes (mostly gorilla/mux, a few older `/api/*` on `http.DefaultServeMux`), handlers are closure factories `Xxx(db, cookieStore, ...) func(w, r)`. Admin panel: `admin_ui.go` (dashboard `/admin`, event page `/admin/events/{id}`), `reservations_api.go`, `forms_ui.go`/`forms_fields.go`/`forms_manage.go`, `catalog_ui.go`, `designer_ui.go` (`/admin/rooms/{id}`). Guest pages: `guest.go` (`/res/{user}`), `guest_order.go` (`/order`), `forms_guest.go` (`/form/{userurl}/{formurl}`). `main.go` also holds mail/template helpers and the `Null*` conversion helpers (`ToNS`, `ToNI`, `ToNB`). UI texts: `locales/{pl,cs,en}.toml` via `T`/`TN` (`i18n.go`), panel language per admin.
- Auth (`auth.go`): `users.passwd` holds bcrypt hashes; a plain text password typed into the DB by hand still works and is hashed on next login or server start (`HashPlainPasswords`). Gorilla/sessions cookie store; `InitSession(..., requiredAdmin)` + `mustBeAdmin` guard admin handlers. Note that some `/api/*` designer/reservation endpoints take no cookie store and are unauthenticated.
- `database.go`: `DB` wraps `sqlx`; all queries are hand-written methods named `<Entity><Action>[By...]` (e.g. `ReservationFullGetAll`, `FormAnswerGetByFieldName`). Structs use `db:` tags and `sql.Null*` types. `StructureCreate()` is stale (only called from the unused `initDBUnusedAnymore`); the real schema lives in the production `db.sql`, and schema changes are done by hand-written SQL (see `tools/migration.sql`, `tools/recreate_table.sql`) — when adding columns/tables, provide such a migration script.
- Domain model: `users` = organizations (each has a `url` slug); `admins` belong to users. Reservation side: `rooms` (designed in `/admin/rooms/{id}`, `js/n_designer.js`) contain `furnitures` (types table/chair/object/label, numbered per room — renumbering in `furnitures.go`); `events` link to rooms, `prices` are per event+furniture, `reservations` per event+furniture with status `marked` → `ordered` → `payed` (`disabled` for unavailable). `ticker.go` deletes stale `marked` reservations every 5 min. Forms side: `formtemplates` → `formfields`, submissions are `forms` → `formanswers`; form definitions are formBuilder-format JSON edited by the own field designer (`js/n_fields.js`, `formdef.go`).
- Templates: `tmpl/n_*.html`, re-parsed on every request, so template edits need no rebuild. Admin pages use `n_base.html` (`nRender`), guest pages `n_guest_base.html` (`guestRender`).
- User-authored templates stored in DB (event/form thank-you mails, info panels, notifications) are executed as Go templates with method receivers `FormFuncs` (`.Sum`, `.Field`, `.QRPay`, `.MTotal`, …) and `OrderFuncs` (`.QRPay`, `.PDF`) — changing these method signatures breaks existing templates in production data.
- `mail.go`: SMTP sending via `MailConfig` (HTML + text alternative, attachments, CID-embedded images). `qrcode.go`: Czech QR payment (SPAYD) codes. `pdf-tickets.go`: gopdf ticket generation (A4 sheet or single tickets).
- Styles: edit `scss/*.scss`, never `css/` output for those files. Vendored JS/CSS libraries in `js/`/`css/` (sources/versions in `tools/download.sh`, `tools/src/`).

## Deployment

Push to `master` triggers `.github/workflows/go.yml`: builds, scp's binary to the server, backs up `db.sql`, restarts the `rezerwo` systemd service and runs `git pull` there (so templates/static files deploy via git). Every push to master goes to production.

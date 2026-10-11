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

Tests: `go test .` runs the unit tests (`TestSendMail` skips itself without its env vars, `TestGenerateTicketsPDF` without `media/fonts`). `database_test.go` runs DB methods on a fresh temp DB built from `testdata/schema.sql` (production schema dump) — refresh it after every schema migration: `sqlite3 db.sql .schema | grep -v sqlite_sequence > testdata/schema.sql` (keep the header comment).
`mail_test.go` is an integration test that sends real mail:
`MSERVER= MUSER= MPASS= MTO= go test -run TestSendMail`.

## Runtime requirements

The binary resolves everything relative to the working directory: `config.toml` (gitignored; cookie keys, SMTP settings — see `settings.go`; old keys like `Location`, `DBFile`, `MailHostname` are ignored), `db.sql` (gitignored SQLite DB, hardcoded in `initDB`), `tmpl/`, `js/`, `css/`, `img/`, `media/`.

`media/<user-url>/` holds per-organization user data (subdirs `email/` for embedded mail images/attachments, `qrcode/` generated payment QR codes, `pdf/` tickets); `media/fonts/` has NotoSans used for PDF tickets; the UI webfonts (Red Hat) are in git under `css/fonts/` (`tools/download-fonts.sh`). `media/` is gitignored.

## Architecture

- `main.go`: `main()` wires all routes (mostly gorilla/mux, a few older `/api/*` on `http.DefaultServeMux`), handlers are closure factories `Xxx(db, cookieStore, ...) func(w, r)`. Admin panel: `admin_ui.go` (dashboard `/admin`, event page `/admin/events/{id}`), `reservations_api.go`, `forms_ui.go`/`forms_fields.go`/`forms_manage.go`, `catalog_ui.go`, `designer_ui.go` (`/admin/rooms/{id}`). Designer saves: `designer_api.go`; event copy: `event_copy.go`. Guest pages: `guest.go` (`/res/{user}`), `guest_order.go` (`/order`, `/order/status` = order confirmation + mails, `/api/ordercancel`), `forms_guest.go` (`/form/{userurl}/{formurl}`). `main.go` also holds the `Null*` conversion helpers (`ToNS`, `ToNI`, `ToNB`). UI texts: `locales/{pl,cs,en}.toml` via `T`/`TN` (`i18n.go`), panel language per admin.
- Auth (`auth.go`): `users.passwd` holds bcrypt hashes; a plain text password typed into the DB by hand still works and is hashed on next login or server start (`HashPlainPasswords`). Gorilla/sessions cookie store (signed + encrypted); admin handlers start with `adminUser(w, r, db, cs)` (redirects to login, returns the organization), `sessionAdmin` checks a session without redirecting. Guest endpoints (`/order*`, `/api/ordercancel`, `/form/...`) are public and may only touch `marked` reservations / own submissions; never trust their form data (`order_check.go`: seat prices and totals come from the DB, seats must be free and in the event's rooms during the sale window, `/order/status` and `/api/ordercancel` need the signed `lock` from the `/order` step; `checkValues` in `forms_guest.go` re-checks e-mail/number fields; one submission per e-mail and form: the same e-mail + name + surname gets a diff of changed answers to confirm (`ownSubmission`, `resubmitDiff`), another person with that e-mail gets 409). IDs referenced in admin saves (rooms, mails, accounts, events) must belong to the admin's organization (`refOwned`, `setEventRooms`). Static dirs are served without directory listings (`noDirFS`).
- `database.go`: `DB` wraps `sqlx`; all queries are hand-written methods named `<Entity><Action>[By...]` (e.g. `ReservationFullGetAll`, `FormAnswerGetByFieldName`). Structs use `db:` tags and `sql.Null*` types. The schema lives in the production `db.sql` (dump in `testdata/schema.sql`), and schema changes are done by hand-written SQL (see `tools/migration.sql`, `tools/recreate_table.sql`) — when adding columns/tables, provide such a migration script.
- Domain model: `users` = organizations (each has a `url` slug); `admins` belong to users. Reservation side: `rooms` (designed in `/admin/rooms/{id}`, `js/n_designer.js`) contain `furnitures` (types table/chair/object/label, numbered per room — renumbering in `furnitures.go`); `events` link to rooms, `prices` are per event+furniture, `reservations` per event+furniture with status `marked` → `ordered` → `payed` (`disabled` for unavailable). `ticker.go` deletes stale `marked` reservations every 5 min. Forms side: `formtemplates` → `formfields`, submissions are `forms` → `formanswers`; form definitions are formBuilder-format JSON edited by the own field designer (`js/n_fields.js`, `formdef.go`). Conditional fields (`show_if`, header = section) and dependent options (`depends_on` + option `when`) live in `formconds.go`, mirrored on the guest page and in the designer's try mode by `js/n_formconds.js`; hidden fields are blanked server-side on submit.
- Templates: `tmpl/n_*.html`, re-parsed on every request, so template edits need no rebuild. Admin pages use `n_base.html` (`nRender`), guest pages `n_guest_base.html` (`guestRender`).
- User-authored templates stored in DB (event/form thank-you mails, info panels, notifications) are executed as Go templates with method receivers `FormFuncs` (`.Sum`, `.Field`, `.QRPay`, `.MTotal`, …) and `OrderFuncs` (`.QRPay`, `.PDF`) — changing these method signatures breaks existing templates in production data.
- `mail.go`: `MailConfig` (SMTP from config) `.Send(Mail)` / `.SendAsOrg(...)` (HTML + text alternative, attachments, CID-embedded images; `Notification.Files/Images` give a mail's media paths). Template data for mails: `form_funcs.go` (`FormFuncs`), `mail_vars.go` (`OrderFuncs`). `qrcode.go`: Czech QR payment (SPAYD) codes. `pdf-tickets.go`: gopdf ticket generation (A4 sheet or single tickets).
- **Guest pages are always in the event/form language, without exception** (`events.language` / `formtemplates.language`; `KnownLang(event.Language.String)`, `formLang(templ)`). Many guests have Czech browsers while events are Polish, so neither `Accept-Language`, the panel-language cookie nor the browser may affect what a guest sees:
  - every guest text comes from `locales/*.toml` in all three languages; no hard-coded strings in handlers;
  - no browser-drawn UI: guest `<form>`s have `novalidate` and validate in JS with `RZ.inputError` (`js/n_ui.js`, `gform.err.*` texts); no `type="date|time|color|file|…"` inputs (date fields are text inputs with `data-date`, stored as `dd.mm.yyyy`); no `alert/confirm/prompt`, no `beforeunload`, no `toLocaleString()` without a language;
  - only pages with no event/form at all (unknown organization) fall back to `DEFAULTLANG`; an organization without an open event uses its newest event's language (`orgLang`).
  `guest_lang_test.go` enforces this; extend it when adding a guest page or script.
- Own scripts are included via `{{asset "/js/n_….js"}}` (mtime version) so a deploy never pairs new HTML with a cached old script.
- Styles: edit `scss/*.scss`, never `css/` output for those files. Vendored JS/CSS libraries in `js/`/`css/` (sources/versions in `tools/download.sh`, `tools/src/`).

## Deployment

Production is upgraded by hand on the server (`ssh web`, app in `/var/www/zori.cz/rezerwo`, Alpine with OpenRC, no systemd) after pushing to `master`: back up `db.sql` into `backup/`, `git pull --ff-only`, `CGO_ENABLED=0 go build`, then `rc-service rezerwo restart`. Pushing alone does not upgrade production: `.github/workflows/go.yml` only builds and runs the tests (Go version from `go.mod`). Schema changes need their `tools/migration-*.sql` run on the production `db.sql` before the restart.

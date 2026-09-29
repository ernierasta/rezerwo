2024:

* bug when adding new notification, users_id is 0, should be current user id!


* Mail is generated from form data (Name and Surname), not from stored data.
* Send mail after setting "payed" status. Only one mail for one person.
* Mail - send room name!
* GDPR - events table.

Admin XP:
* map view for admin,
* Different color after payed on table (green),
* Dock buttons
* Remove filters

Customer XP:
* total/free chairs - no reservation page.
* show from witch rooms are selected chairs.

Designer:
* numeric input - current chair number and current table number, current object number.
* possibility to change object color

Bugs:
* reservation - bug, not working payed sometimes. TODO: show bug if happen

---

## QR Code generation: disk vs memory investigation

### Current flow

#### Payment QR codes (SPAYD) – written to disk, never cleaned up

Two code paths share `GenerateQRimg()` (main.go:5222):

1. **Order/event flow** (guest books seats):
   - `OrderFuncs.QRPay()` → `<img src="/media/<user>/qrcode/<file>.jpeg">` for web (main.go:5184)
   - `OrderFuncs.QRPayMail()` → CID-embedded image for mail (main.go:5189)
   - Mail library reads the file back from disk at send time (`message.EmbedFile(namePath)` in mail.go)

2. **Form flow** (submitting online forms):
   - `FormFuncs.QRPay()` → `<img src="/media/<user>/qrcode/<file>.jpeg">` for web (main.go:5128)
   - `FormFuncs.QRPayMail()` → CID-embedded for mail (main.go:5141)
   - Same `QRPayFieldPrefix` / `QRPayMailFieldPrefix` variants for multiple QR codes per form

Both paths call `GenerateQRimg()` → `qr.GenCode(imgpath)` which writes JPEG to:

```
media/<user-url>/qrcode/<templURL>_<NameSurname>.jpeg
```

**No cleanup mechanism exists for these files.** They accumulate permanently.

#### PDF ticket QR codes – written to disk, then cleaned up

`pdf-tickets.go:renderTicket()`:
- Writes temporary PNG to `media/tmp/qr_<userURL>_<seat>.png`
- Embeds it into the PDF via gopdf
- Calls `os.Remove(qrPath)` immediately after (line 178) ✅

This path is already correct.

#### Web serving

Payment QR images are served via Go's static file serving (`/media/` → filesystem path). The browser makes a separate HTTP request, so the file must exist at request time.

#### File naming and overwriting

File name is deterministic: `<templateURL>_<NameSurname>.jpeg` (with optional `ImgPrefix`). Same customer + same template → same filename → overwrites each time. Multiple QR codes per form use `ImgPrefix` to differentiate.

### Issues

1. **Disk accumulation** — payment QR files in `media/<user>/qrcode/` are never deleted. Over months/years this grows unboundedly. Each JPEG is ~300–500 bytes, so even 100k submissions ≈ 50 MB — trivial for modern disks, but messy.

2. **Deterministic filenames** — same name + same template always writes the same file. This is *mostly* fine (overwrite is harmless), but two *different* people with identical names submitting the same form at nearly the same time could theoretically race. Low practical risk.

3. **Web display requires filesystem path** — `<img src="/media/...">` needs the file on disk. Serving from memory would require a custom HTTP handler or switching to data URIs.

4. **Mail CID embedding reads from disk** — `go-mail` library's `EmbedFile()` opens and reads the file from disk. No obvious `EmbedData()` or `EmbedReader()` API in the current version.

### Options

#### A. Keep as-is (do nothing)

- ✅ Zero effort
- ✅ Works reliably
- ❌ Files never cleaned up (disk bloat, mainly cosmetic)
- ❌ Writes to disk unnecessarily (writing is fast, though)

**Verdict**: Perfectly acceptable. For this application's scale (hundreds of submissions/year per organization), the accumulated disk usage is negligible (~50 KB/year per organization). Not worth refactoring.

#### B. Keep disk writes, add cleanup

Add a cleanup goroutine similar to `ticker.go`'s `RemoveStalledOrders()`:
- Delete QR files older than 24h from `media/*/qrcode/`
- Or delete all QR files on application startup

- ✅ Simple to implement
- ✅ Solves accumulation
- ❌ Still writes to disk
- ❌ Must be careful about race with concurrent requests
- ❌ After cleanup, revisiting a thank-you page or resending mail would regenerate (fine — it generates a fresh QR anyway)

#### C. Generate to memory, serve via custom handler

Change `GenerateQRimg` to return `[]byte` + MIME type instead of writing to disk:
- QR is held in memory during the template parse phase
- A dedicated `/qrimg/` HTTP handler serves the image from an in-memory cache keyed by filename
- For mail, find a way to pass `[]byte` to the mail library

- ✅ No disk writes
- ✅ No cleanup needed
- ❌ Requires custom HTTP handler (though trivial to add)
- ❌ `go-mail`'s `EmbedFile` takes a filesystem path — would need to check if `EmbedReader` exists
- ❌ More complex, more room for bugs

#### D. Data URI for web, memory buffer for mail

For web display: `<img src="data:image/jpeg;base64,...">` — embed the QR directly in HTML.
For mail: check if `go-mail` supports embedding from `io.Reader` or `[]byte`.

- ✅ No disk writes
- ✅ No HTTP handler changes needed for web
- ❌ Data URI in mail would not work consistently across email clients (CID is the standard)
- ❌ `go-mail` library — `EmbedFile` only takes a path; no `EmbedData` or `EmbedReader` in the current API

### Recommendation: Option A (keep as-is)

For this application:
- **Scale is small**: a few hundred submissions per year per organization, max a few thousand total
- QR files are ~300–500 bytes each → even 10,000 files = ~5 MB
- The payoff from optimizing is near-zero
- **PDF ticket QR handling is already correct** (cleaned up immediately via `os.Remove`)

If cleanup is ever wanted, **Option B** (periodic cleanup of `media/*/qrcode/` files older than a configurable TTL) is the pragmatic choice. It's a ~20-line goroutine in `ticker.go`.

### Additional notes

- The `generateQRimg` and `QRCode` functions in `qrcode.go` are clean and well-structured
- `go-qrcode/v2` writes via `writer/standard` which outputs to an `io.Writer` — it *can* write to `bytes.Buffer`, but currently always writes to a file opened by `standard.New(imgpath)`. To go in-memory, you'd either use a different writer or write to a temp buffer then copy to a file. The `standard` writer package only exports `New(filepath)` in v2.2.2 — no `NewWithWriter` option exists.
- `qrcode.Save(w)` accepts any `writer.Writer` interface, so other writer implementations are possible, but switching writers just for this is overkill.
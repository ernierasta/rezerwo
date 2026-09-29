# LIVE_MAP.md — mapka miejsc na żywo dla gości

Propozycja zmiany: zakładka/plan sali (`/n/res/{user}`, szablon `tmpl/n_reservation.html`,
skrypt `js/n_room.js`) ma wyświetlać zajęte miejsca na bieżąco — gdy jeden gość
zaznaczy krzesło, u pozostałych gości z otwartą mapką miejsce od razu staje się
zajęte. Do tego limit: jeden gość nie może zaznaczyć więcej niż 1/3 wszystkich
miejsc. System zwalniania niezrealizowanych blokad ma pozostać taki sam
(`ticker.go`, 5 minut po statusie `marked`).

---

## 1. Stan obecny

1. **Get** `/n/res/{user}` → `GuestReservationPage` (`guest.go`) buduje `nGuestVars`
   z salami i krzesłami. Status krzesła liczy `chairStatus()` (`room_svg.go`):
   `ordered|payed|marked` → `taken`, `disabled` → `disabled`, reszta → `free`.
2. **HTML** (`n_reservation.html`) rysuje SVG; każde krzesło to
   `<g class="chair" data-number=".." data-room=".." data-status="free|taken|disabled">`.
3. **JS** (`n_room.js`) trzyma wybór **tylko po stronie klienta** w tablicy
   `selected`. Kliknięcie zmienia klasę (`chair-free` ↔ `chair-selected`), ale
   **nie wywołuje serwera**.
4. Dopiero **klik „Zamawiam”** wysyła `POST /n/order` (`GuestOrderPage`,
   `guest_order.go`), który dla każdego wybranego krzesła zapisuje wiersz
   `reservations` ze statusem `marked` i `customers_id_fk = -1`.
5. **Ticker** (`ticker.go`) co 1 minutę usuwa `marked` z `customers_id_fk = -1`
   starsze niż 5 minut — to jest obecne „zwalnianie niezarezerwowanych miejsc”.

Wniosek: dzisiaj blokada pojawia się **dopiero po kliknięciu „Zamawiam”**,
nie przy wyborze krzesła. Stąd cała reszta gości widzi „wolne” miejsce, dopóki
ktoś nie przejdzie do formularza danych.

---

## 2. Wymagania

1. **Live** — zaznaczenie krzesła ma od razu (≤ ok. 2–3 s) zablokować je u
   innych gości i odwrotnie.
2. **Limit 1/3** — gość nie może zablokować więcej niż `max(1, floor(liczba_miejsc/3))`
   miejsc całej imprezy (wszystkie sale łącznie).
3. **Zwalnianie jak dotąd** — niezrealizowane blokady dalej znikają po 5 min
   przez `ticker.go`; bez nowego, osobnego mechanizmu TTL.

---

## 3. Decyzje projektowe

### 3.1. Transport live: **polling** (nie WebSocket / SSE)

| Opcja | Plusy | Minusy |
|---|---|---|
| **Polling (rekomendowane)** | prosty; działa z obecnym serwerem i `http.DefaultServeMux`/mux; łatwe cache'owanie | opóźnienie równe interwałowi (2–3 s); stały ruch |
| SSE | push, mniejszy ruch | trzymanie połączeń; więcej stanu po stronie serwera; nie pasuje do obecnego stylu |
| WebSocket | pełny dupleks | zupełnie nowa zależność/obsługa; zbędne przy mapce |

**Rekomendacja:** `setInterval` co **2,5 s** w `n_room.js` → `GET` lekki endpoint
statusów. W razie potrzeby da się podnieść do SSE bez zmiany modelu danych.

### 3.2. Gdzie trzymać blokady: **te same `reservations` + nowa kolumna `hold_token`**

Nie wprowadzamy osobnego magazynu w pamięci, bo wtedy musielibyśmy wymyślić **drugi**
mechanizm wygaszania — a wymaganie 3 mówi, żeby użyć **istniejącego** `ticker.go`.

Zamiast tego:
- zaznaczenie krzesła → od razu wiersz `reservations` ze statusem `marked`,
  `customers_id_fk = -1` (tak jak dziś w `/n/order`, tylko wcześniej),
- unikalny identyfikator gościa w nowej kolumnie `hold_token`,
- `ticker.go` **bez zmian** — dalej kasuje `marked`+`-1` starsze niż 5 min.

Zaleţy to działać też dla starej strony: wiersze `marked` utworzone przez stary
`/order` nadal będą widoczne jako `taken` na nowej mapce.

---

## 4. Model danych (migracja)

Nowy plik `tools/migration-live-map.sql`:

```sql
ALTER TABLE reservations ADD COLUMN hold_token TEXT DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_res_hold
    ON reservations (events_id_fk, hold_token, status);
```

- `hold_token` = losowy identyfikator gościa (ciasteczko), tylko dla wierszy
  `marked` z `customers_id_fk = -1`. Po przejściu do `ordered` pole można
  zostawić albo wyczyścić (nie ma znaczenia).
- `Reservation` w `database.go` dostaje pole `HoldToken sql.NullString db:"hold_token"`.

### Identyfikator gościa (ciasteczko)

Przy pierwszym wejściu na `/n/res/{user}` serwer ustawia `SetCookie`:

```
Name:   guest_hold
Value:  64-bitowy losowy heks (crypto/rand)
HttpOnly: true
Path:    /
MaxAge:  24h
```

Każda kartka/urządzenie dostaje własny token; ten sam token łączy wybory w
ramach jednej przeglądarki (zakładki się sumują — to pożądane).

---

## 5. API (nowe, publiczne — bez sesji admina)

Wszystkie endpointy biorą `eventID` z URL-a. Walidacja taka jak w `guest.go`:
impreza publiczna (`status != 'draft'`) albo podgląd przy ważnej sesji admina.

### 5.1. Status miejsc (polling)

```
GET /api/events/{id}/seatmap
```

Odpowiedź (lekka, tylko krzesła):

```json
{
  "event_id": 42,
  "limit": 3,
  "held":   2,
  "seats": [
    {"id": 901, "room_id": 7, "number": 1, "status": "free"},
    {"id": 902, "room_id": 7, "number": 2, "status": "taken", "own": false},
    {"id": 903, "room_id": 7, "number": 3, "status": "selected", "own": true}
  ]
}
```

- `status` dla gościa: `free`, `taken` (cudze `marked/ordered/payed`),
  `disabled`, `selected` (własne `marked`).
- `own=true` gdy `hold_token` pasuje do ciasteczka żądania.
- `limit` i `held` pozwalają JS-owi blokować nadmiarowe wybory.

Źródło: nowa metoda `db.SeatStatesByEvent(eventID, holdToken)` — zapytanie
podobne do `FurnitureFullGetChairs`, ale dla wszystkich sal imprezy naraz:

```sql
SELECT f.id, f.number, f.rooms_id_fk,
       COALESCE(r.status,'free') AS status,
       (r.hold_token = $2)      AS own
FROM furnitures f
JOIN events_rooms er ON er.rooms_id_fk = f.rooms_id_fk
                     AND er.events_id_fk = $1
LEFT JOIN reservations r ON r.furnitures_id_fk = f.id
                        AND r.events_id_fk  = $1
WHERE f.type = 'chair'
ORDER BY f.rooms_id_fk, f.number;
```

(Stan `disabled` z tabeli `prices` trzeba doliczyć tak samo jak robi to
`chairStatus` — admin może zablokować krzesło dla imprezy.)

### 5.2. Zaznaczenie / odznaczenie

```
POST   /api/events/{id}/hold      {"seat_id": 901}
DELETE /api/events/{id}/hold      {"seat_id": 901}
```

POST (atomowo):
1. sprawdź, czy krzesło wolne (brak wiersza `reservations` dla `furniture+event`),
2. policz obecne holdy gościa `held` i porównaj z `limit`,
3. jeśli OK → `INSERT reservations(status='marked', customers_id_fk=-1,
   hold_token=..., ordered_date=now, price..., currency...)`.

DELETE: usuń tylko wiersz `marked` z `hold_token` = ciasteczko (nie cudzy).

Błędy zwracają 409 z kodem: `seat_taken` albo `limit_reached` (JS pokazuje toast).

### 5.3. Odświeżenie czasu (sercowanie blokady)

```
POST /api/events/{id}/hold/heartbeat
```

Update `ordered_date = now` dla wszystkich `marked` tego gościa (tej imprezy).
Wywoływany **tylko przy akcji użytkownika** (klik dodaj/usuń oraz przy przejściu
do formularza `/n/order`), **nie** w tle — patrz 7.

---

## 6. Zmiany w kodzie

### 6.1. `guest.go` / `n_reservation.html`

- `GuestReservationPage` ustawia ciasteczko `guest_hold`, jeśli go brak.
- `guestRoom()` dostaje `holdToken`; własne `marked` (hold_token == token)
  renderuje jako `data-status="selected"` i klasę `chair-selected`, żeby po
  odświeżeniu strony wybór nie zniknął.
- W szablonie dodać `data-id="{{.ID}}"` do `<g class="chair">` (dziś jest tylko
  `data-number`, `data-room`, `data-table`) — `ID` to `furnitures.id` i już jest
  w `svgFurniture`.
- Przekazać do szablonu `data-limit="{{.Limit}}"` (limit dla JS).

### 6.2. `n_room.js`

- Na klik wolnego krzesła: zamiast tylko lokalnej klasy, wywołaj
  `POST /api/events/{id}/hold`. Po sukcesie dodaj do `selected`; po 409
  (`seat_taken`/`limit_reached`) pokaż toast i nie dopisuj.
- Klik zaznaczonego (własnego): `DELETE .../hold`, usuń z `selected`.
- `setInterval(…, 2500)` + od razu po starcie:
  - pobierz `GET .../seatmap`,
  - dla każdego krzesła zaktualizuj klasę/status; `own=true` → `selected`,
    cudzy `taken`/`disabled` → odpowiednia klasa; krzesło z lokalnego `selected`,
    które nagle wróciło do `free` (wygasło) → usuń z koszyka i daj toast.
- Przycisk „Wyczyść”: najpierw `DELETE` wszystkich własnych holdów, potem wyczyść UI.
- Przy „Zamawiam”: wyślij `POST /n/order` jak dziś (holdy już są w DB).

### 6.3. `guest_order.go` (`/n/order`)

Po przejściu do formularza danych **nie tworzy już** wierszy `marked` od zera
(byłby konflikt z `UNIQUE(furnitures_id_fk, events_id_fk)`), tylko:

1. zweryfikuj, że przesłane miejsca są trzymane przez `hold_token` (ew. utwórz
   brakujące dla gości ze starej/bez-javascriptowej ścieżki),
2. `UPDATE` `ordered_date = now` dla przesłanych miejsc (fresh 5 minut na
   wypełnienie danych),
3. usuń pozostałe `marked` tego gościa, których NIE ma w przesłanym wyborze
   (gość zmienił zdanie między mapką a wysłaniem),
4. reszta bez zmian.

Stary `/order` (`ReservationOrderHTML`) może zostać jak jest — nadal tworzy
`marked`; będzie widoczny na nowej mapce jako `taken`. Limit 1/3 tam **nie**
obowiązuje w v1 (patrz 9).

### 6.4. `ticker.go` — **bez zmian**

`RemoveStalledOrders` dalej usuwa `status='marked' AND customers_id_fk=-1`
starsze niż 5 min. Nasze holdy mają dokładnie te same cechy, więc zwalnianie
działa identycznie jak dziś.

---

## 7. Semantyka zwalniania (żeby „było tak samo”)

- Hold żyje **5 minut** od ostatniej akcji: `ordered_date` jest ustawiany przy
  utworzeniu i odświeżany na każde „dodaj/usuń” oraz przy `/n/order`.
- **Nie** odświeżamy go w tle pollingiem — inaczej otwarta na noc kartka
  blokowałaby miejsca w nieskończoność.
- Zamknięcie karty / przerwanie działania → brak nowych akcji → po 5 min ticker
  zwalnia miejsca. Dokładnie jak dzisiejszy `marked`.

---

## 8. Limit 1/3 i ochrona przed spamem

```
totalSeats = liczba krzeseł we wszystkich salach imprezy
limit      = max(1, totalSeats / 3)          // floor, min. 1 dla małych imprez
held       = COUNT(*) FROM reservations
             WHERE events_id_fk=? AND status='marked' AND hold_token=?
```

- Blokada nadmiaru jest **po stronie serwera** (POST `.../hold`), a `limit`/`held`
  dodatkowo blokuje kliknięcia w JS-ie (UX).
- Osobny gość = osobny `hold_token`, więc limit liczony jest per przeglądarka,
  nie globalnie.

**Atomiczność** (ważne): sprawdzenie `held < limit` + `INSERT` muszą być w jednej
transakcji, bo inaczej dwa równoległe żądania tego samego gościa przekroczą limit.
SQLite (modernc.org/sqlite) — propozycja: per-gość mutex w pamięci
(`sync.Map[string]*sync.Mutex`) + transakcja `db.Beginx()`; dodatkowo unikalny
`UNIQUE(furnitures_id_fk, events_id_fk)` już chroni przed podwójnym zajęciem
tego samego krzesła przez dwie osoby.

Alternatywnie jeden atomowy `INSERT ... SELECT ... WHERE NOT EXISTS(...) AND
(SELECT COUNT(*)...) < limit` — do sprawdzenia pod kątem semantyki SQLite.

---

## 9. Plan wdrożenia (przyrostowo)

1. **Migracja** `tools/migration-live-map.sql` (kolumna + indeks) + pole w
   strukturze `Reservation`.
2. **Backend read:** `SeatStatesByEvent` + endpoint `GET /api/events/{id}/seatmap`
   (najpierw bez holdów, zwraca dzisiejsze statusy).
3. **Backend write:** `POST/DELETE .../hold` + `.../hold/heartbeat` z limitem,
   atomową blokadą i ciasteczkiem `guest_hold`.
4. **Frontend:** `data-id`, `data-limit`, obsługa klik→hold, polling, koszyk,
   „Wyczyść”, toasty.
5. **Render początkowy:** własne holdy jako `selected` po odświeżeniu strony.
6. **`/n/order`:** refresh `ordered_date` + sprzątanie nieprzesłanych holdów.
7. Testy + przełączenie starej `/res` na nową mapkę (albo `go run` lokalnie
   z dwoma przeglądarkami).

---

## 10. Ryzyka / pytania otwarte

- **Ukryta zakładka w tle** nadal pollinguje (ruch), ale nie dotyka holdów —
  akceptowalne.
- **Nowy kolizyjny wybór** — gość klika wolne krzesło, ale inny zdążył je zająć
  → serwer zwraca `seat_taken`, JS pokazuje komunikat i nie zaznacza. OK.
- **Admin wyłącza krzesło** gdy gość je trzyma — na mapce stan `disabled`
  nadpisuje; przy `/n/order` trzeba uwzględnić spójność (ew. odrzucić wybór).
- **Stary `/order` bez limitu** — do zdecydowania, czy też objąć limitem/dodatkową
  migracją, czy zostawić jako tor leganie.
- **Limit 1/3 a imprezy z kilkoma salami** — liczymy globalnie („wszystkie”), jak
  w wymaganiu.
- **Czy limit ma być per-impreza konfigurowalny** (kolumna w `events`)? W v1 stały.
- **Skalowalność pollingu** — dla bardzo dużych imprez rozważyć krótkie cache
  statusów per event (np. 500 ms) albo SSE.
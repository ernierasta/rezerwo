package main

// room_svg.go: geometria planu sali dla strony gościa (etap 4 redesignu).
//
// Stara strona rezerwacji układa sale z elementów <div> pozycjonowanych w px,
// a rozmiary stołów biorą się z klas CSS (scss/room.scss). Nowa strona rysuje
// to samo w SVG z viewBox równym wymiarom sali, więc plan skaluje się do
// szerokości ekranu bez utraty ostrości.
//
// WAŻNE: liczby poniżej muszą dawać ten sam układ co scss/room.scss i
// js/room.js (designer). Stół ma margines 10 px, krzesło 2 px, krzesło ma
// 20 px boku - stąd przesunięcia względem zapisanych x/y.

import (
	"html/template"
	"math"
	"strconv"
	"strings"
)

const (
	svgChairSize   = 20.0 // .chair width/height ($table w scss/variables.scss)
	svgChairMargin = 2.0  // .chair margin
	svgTableMargin = 10.0 // .table margin
	svgTableUnit   = 20.0 // $table
	svgTableGap    = 5.0  // $table-gap
	svgChairPitch  = 30.0 // $round-chair-pitch
	svgChairDist   = 15.0 // $round-chair-dist = $table/2 + $table-gap
)

// svgFurniture to jeden element planu, już przeliczony na współrzędne SVG.
type svgFurniture struct {
	ID     int64
	Number int64
	X, Y   float64
	W, H   float64
	// Shape: "rect" albo "ellipse"
	Shape  string
	Color  string
	Label  string
	Status string // tylko krzesła: free, taken, selected, disabled
	// State - dokładny stan zajętego krzesła (marked/ordered/payed); temat
	// "klasyczny" koloruje je osobno jak dawna strona rezerwacji
	State    string
	Price    int64
	Currency string
}

// svgRoom to sala gotowa do narysowania.
type svgRoom struct {
	RoomID     int64
	Name       string
	Width      int64
	Height     int64
	Tables     []svgFurniture
	Chairs     []svgFurniture
	Objects    []svgFurniture
	Labels     []svgFurniture
	SeatsFree  int
	SeatsTotal int
	Banner     template.HTML
	Desc       template.HTML
}

// tableLen liczy długość stołu dla danej liczby miejsc, tak jak $table-N
// w scss/variables.scss: połowa krzeseł wzdłuż stołu, z przerwami między nimi.
func tableLen(capacity int64) float64 {
	n := capacity / 2
	if n < 1 {
		n = 1
	}
	return float64(n)*svgTableUnit + float64(n-1)*svgTableGap
}

// tableSize zwraca rozmiar i kształt stołu dla orientacji z bazy
// ("horizontal", "vertical", "round", "oval-horizontal", "oval-vertical").
func tableSize(orientation string, capacity int64) (w, h float64, shape string) {
	n := float64(capacity)
	switch {
	case strings.HasPrefix(orientation, "round"):
		d := math.Max(40, math.Round(svgChairPitch*n/math.Pi-2*svgChairDist))
		return d, d, "ellipse"

	case strings.HasPrefix(orientation, "oval"):
		s := math.Max(40, 3*n)
		b := s/2 + svgChairDist
		a := math.Max(1.5*b, (1.06*svgChairPitch*n-2*math.Pi*b)/4+b)
		l := math.Round(2 * (a - svgChairDist))
		if strings.HasSuffix(orientation, "vertical") {
			return s, l, "ellipse"
		}
		return l, s, "ellipse"

	case orientation == "vertical":
		return svgTableUnit, tableLen(capacity), "rect"

	default: // horizontal
		return tableLen(capacity), svgTableUnit, "rect"
	}
}

// svgChairAssign - krzesło należy do stołu, jeżeli jego środek jest najwyżej
// tyle od krawędzi stołu. Ta sama liczba co CHAIR_ASSIGN_DIST w js/room.js.
const svgChairAssign = svgChairSize + svgTableGap + svgChairSize/2

// chairsOfTable zwraca krzesła stojące przy danym stole. Używa tej samej
// geometrii co plan sali, żeby designer i strona gościa liczyły tak samo.
func chairsOfTable(db *DB, table Furniture) ([]Furniture, error) {
	chairs, err := db.FurnitureGetAllByRoomIDOfType(table.RoomID, "chair")
	if err != nil {
		return nil, err
	}
	tables, err := db.FurnitureGetAllByRoomIDOfType(table.RoomID, "table")
	if err != nil {
		return nil, err
	}

	box := func(f Furniture) (x, y, w, h float64) {
		if f.Type == "table" {
			tw, th, _ := tableSize(f.Orientation.String, f.Capacity.Int64)
			if f.Width.Valid && f.Width.Int64 > 0 {
				tw = float64(f.Width.Int64)
			}
			if f.Height.Valid && f.Height.Int64 > 0 {
				th = float64(f.Height.Int64)
			}
			return float64(f.X) + svgTableMargin, float64(f.Y) + svgTableMargin, tw, th
		}
		return float64(f.X) + svgChairMargin, float64(f.Y) + svgChairMargin, svgChairSize, svgChairSize
	}

	// odległość środka krzesła od prostokąta stołu (0 = w środku stołu)
	dist := func(cx, cy float64, t Furniture) float64 {
		x, y, w, h := box(t)
		dx := 0.0
		if cx < x {
			dx = x - cx
		} else if cx > x+w {
			dx = cx - (x + w)
		}
		dy := 0.0
		if cy < y {
			dy = y - cy
		} else if cy > y+h {
			dy = cy - (y + h)
		}
		return math.Hypot(dx, dy)
	}

	var out []Furniture
	for _, c := range chairs {
		cx, cy, cw, ch := box(c)
		centerX, centerY := cx+cw/2, cy+ch/2

		d := dist(centerX, centerY, table)
		if d > svgChairAssign {
			continue
		}
		// krzesło liczy się tylko do najbliższego stołu
		nearest := true
		for _, other := range tables {
			if other.ID == table.ID {
				continue
			}
			if dist(centerX, centerY, other) < d {
				nearest = false
				break
			}
		}
		if nearest {
			out = append(out, c)
		}
	}
	return out, nil
}

// chairStatus tłumaczy stan rezerwacji na stan widoczny dla gościa.
// Gość nie musi odróżniać "zamówione" od "zapłacone" - dla niego miejsce jest
// wolne, zajęte albo niedostępne (rozdz. 8.3 HARMONIJKA.md).
func chairStatus(f FurnitureFull) string {
	if f.AdminDisabled.Valid && f.AdminDisabled.Int64 == 1 {
		return "disabled"
	}
	switch f.Status.String {
	case "ordered", "payed", "marked":
		return "taken"
	case "disabled":
		return "disabled"
	default:
		return "free"
	}
}

// takenState - stan rezerwacji zajętego krzesła, pusty dla pozostałych.
func takenState(status, reservation string) string {
	if status != "taken" {
		return ""
	}
	switch reservation {
	case "marked", "ordered", "payed":
		return reservation
	}
	return ""
}

// reservedUnder mówi, ile rezerwacji blokuje skasowanie mebla i czego dotyczą:
// "chair" - samo krzesło, "table" - krzesła stojące przy stole.
func reservedUnder(db *DB, m DeleteMsg) (int64, string, error) {
	switch m.Type {
	case "chair":
		n, err := db.ReservationCountByFurnitureNumber(m.Number, "chair", m.RoomID)
		return n, "chair", err

	case "table":
		table, err := db.FurnitureGetByTypeNumberRoom("table", m.Number, m.RoomID)
		if err != nil {
			return 0, "table", err
		}
		chairs, err := chairsOfTable(db, table)
		if err != nil {
			return 0, "table", err
		}
		var total int64
		for _, c := range chairs {
			n, err := db.ReservationCountByFurnitureID(c.ID)
			if err != nil {
				return 0, "table", err
			}
			total += n
		}
		return total, "table", nil

	default: // obiekty i napisy nikogo nie blokują
		return 0, m.Type, nil
	}
}

// buildSVGRoom przelicza meble sali na elementy SVG.
func buildSVGRoom(db *DB, room Room, event Event) svgRoom {
	rv := roomFurnitures(db, room.ID, event.ID)

	out := svgRoom{
		RoomID: room.ID,
		Name:   room.Name,
		Width:  room.Width,
		Height: room.Height,
	}

	for _, t := range rv.Tables {
		w, h, shape := tableSize(t.Orientation.String, t.Capacity.Int64)
		if t.Width.Valid && t.Width.Int64 > 0 {
			w = float64(t.Width.Int64)
		}
		if t.Height.Valid && t.Height.Int64 > 0 {
			h = float64(t.Height.Int64)
		}
		out.Tables = append(out.Tables, svgFurniture{
			ID:     t.ID,
			Number: t.Number,
			X:      float64(t.X) + svgTableMargin,
			Y:      float64(t.Y) + svgTableMargin,
			W:      w,
			H:      h,
			Shape:  shape,
		})
	}

	for _, c := range rv.Chairs {
		status := chairStatus(c)
		price, currency := chairPrice(c, event)

		out.Chairs = append(out.Chairs, svgFurniture{
			ID:       c.ID,
			Number:   c.Number,
			X:        float64(c.X) + svgChairMargin,
			Y:        float64(c.Y) + svgChairMargin,
			W:        svgChairSize,
			H:        svgChairSize,
			Shape:    "rect",
			Status:   status,
			State:    takenState(status, c.Status.String),
			Price:    price,
			Currency: currency,
		})

		out.SeatsTotal++
		if status == "free" {
			out.SeatsFree++
		}
	}

	for _, o := range rv.Objects {
		out.Objects = append(out.Objects, svgFurniture{
			ID:     o.ID,
			Number: o.Number,
			X:      float64(o.X),
			Y:      float64(o.Y),
			W:      float64(o.Width.Int64),
			H:      float64(o.Height.Int64),
			Shape:  "rect",
			Color:  o.Color.String,
			Label:  o.Label.String,
		})
	}

	for _, l := range rv.Labels {
		w := float64(l.Width.Int64)
		h := float64(l.Height.Int64)
		if h == 0 {
			h = svgChairSize
		}
		out.Labels = append(out.Labels, svgFurniture{
			ID:     l.ID,
			Number: l.Number,
			X:      float64(l.X),
			Y:      float64(l.Y),
			W:      w,
			H:      h,
			Shape:  "rect",
			Color:  l.Color.String,
			Label:  l.Label.String,
		})
	}

	return out
}

// contrastInk wybiera kolor tekstu (biały albo prawie czarny) czytelniejszy
// na tle w kolorze bg ("#rgb" albo "#rrggbb", jak zapisuje designer), wg
// kontrastu WCAG. Dla innych zapisów zwraca "" - zostaje kolor domyślny.
func contrastInk(bg string) string {
	h := strings.TrimPrefix(strings.TrimSpace(bg), "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return ""
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return ""
	}
	lin := func(c uint64) float64 {
		s := float64(c) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	l := 0.2126*lin(v>>16&0xff) + 0.7152*lin(v>>8&0xff) + 0.0722*lin(v&0xff)
	const dark = 0.0137 // luminancja #212529
	if (1.05)/(l+0.05) > (l+0.05)/(dark+0.05) {
		return "#FFFFFF"
	}
	return "#212529"
}

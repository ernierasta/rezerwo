package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/signintech/gopdf"
	qrcode "github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
)

const (
	TicketW = 200.0
	TicketH = 50.0

	A4W = 210.0
	A4H = 297.0

	TicketsPerPage = 4 // bilety jeden pod drugim na stronie A4

	FontsDir = "media/fonts"
)

type TicketParams struct {
	UserName         string
	UserURL          string
	EventName        string
	EventDescription string
	SeatNrs          []int
	Rooms            []string
	Lang             string // język imprezy (napis "MIEJSCE")
}

// GenerateTicketsPDF tworzy bilety w media/<organizacja>/pdf: wszystkie na
// arkuszach A4 (pdf/all) i każdy osobno (pdf/one); zwraca ścieżki plików.
//
// TODO(bezpieczeństwo, na później - przegląd 2026-10-11):
//   - nazwy plików da się zgadnąć (<Impreza>_<miejsce>.pdf, publiczne pod
//     /media/), więc każdy może pobrać bilet na dowolne miejsce;
//   - kod QR na bilecie to "<Impreza>|<miejsce>" bez sekretu - bilet można
//     podrobić bez pobierania czegokolwiek, kontrola przy wejściu nic nie
//     sprawdzi.
//
// Gdy bilety mają coś znaczyć: losowa część nazwy pliku (albo wydawanie przez
// handler z tokenem) i podpis HMAC w kodzie QR (impreza, miejsce, rezerwacja),
// sprawdzany przy wejściu.
func GenerateTicketsPDF(p *TicketParams) ([]string, error) {
	if len(p.SeatNrs) == 0 {
		return nil, fmt.Errorf("SeatNrs cannot be empty")
	}
	allDir := path.Join("media", p.UserURL, "pdf", "all")
	oneDir := path.Join("media", p.UserURL, "pdf", "one")
	for _, d := range []string{allDir, oneDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, err
		}
	}

	name := sanitize(p.EventName)
	allFile := path.Join(allDir, fmt.Sprintf("%s_%d_%d.pdf", name, p.SeatNrs[0], p.SeatNrs[len(p.SeatNrs)-1]))
	if err := writeTickets(allFile, gopdf.Rect{W: A4W, H: A4H}, p, p.SeatNrs, p.Rooms, TicketsPerPage); err != nil {
		return nil, err
	}
	files := []string{allFile}

	for i, seat := range p.SeatNrs {
		var rooms []string
		if i < len(p.Rooms) {
			rooms = p.Rooms[i : i+1]
		}
		f := path.Join(oneDir, fmt.Sprintf("%s_%d.pdf", name, seat))
		if err := writeTickets(f, gopdf.Rect{W: TicketW, H: TicketH}, p, []int{seat}, rooms, 1); err != nil {
			log.Printf("GenerateTicketsPDF: seat %d: %v", seat, err)
			continue
		}
		files = append(files, f)
	}
	return files, nil
}

// writeTickets zapisuje bilety miejsc seats (w salach rooms) do pliku PDF,
// perPage na stronę
func writeTickets(file string, page gopdf.Rect, p *TicketParams, seats []int, rooms []string, perPage int) error {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: page, Unit: gopdf.UnitMM})
	for _, f := range []struct{ name, file string }{{"Noto", "NotoSans-Regular.ttf"}, {"Noto-Bold", "NotoSans-Bold.ttf"}} {
		if err := pdf.AddTTFFont(f.name, path.Join(FontsDir, f.file)); err != nil {
			return fmt.Errorf("font %s: %v", f.name, err)
		}
	}
	for i, seat := range seats {
		if i%perPage == 0 {
			pdf.AddPage()
		}
		room := ""
		if i < len(rooms) {
			room = rooms[i]
		}
		if err := renderTicket(pdf, p, seat, room, 0, float64(i%perPage)*TicketH); err != nil {
			return fmt.Errorf("seat %d: %v", seat, err)
		}
	}
	return pdf.WritePdf(file)
}

// bufCloser - bytes.Buffer jako io.WriteCloser dla zapisu kodu QR
type bufCloser struct{ *bytes.Buffer }

func (bufCloser) Close() error { return nil }

// renderTicket rysuje jeden bilet (z ramką do wycięcia) w punkcie x, y
func renderTicket(pdf *gopdf.GoPdf, p *TicketParams, seat int, room string, x, y float64) error {
	qr, err := qrcode.NewWith(fmt.Sprintf("%s|%d", p.EventName, seat),
		qrcode.WithEncodingMode(qrcode.EncModeByte), qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionLow))
	if err != nil {
		return err
	}
	var png bytes.Buffer
	if err := qr.Save(standard.NewWithWriter(bufCloser{&png}, standard.WithBorderWidth(1))); err != nil {
		return err
	}
	img, err := gopdf.ImageHolderByBytes(png.Bytes())
	if err != nil {
		return err
	}
	if err := pdf.ImageByHolder(img, x+TicketW-28, y+8, &gopdf.Rect{W: 20, H: 20}); err != nil {
		return err
	}

	text := func(font string, size, dx, dy float64, s string) {
		pdf.SetFont(font, "", size)
		pdf.SetX(x + dx)
		pdf.SetY(y + dy)
		pdf.Cell(nil, s)
	}
	text("Noto-Bold", 14, 8, 12, p.EventName)

	pdf.SetFont("Noto", "", 9)
	pdf.SetX(x + 8)
	pdf.SetY(y + 18)
	pdf.MultiCell(&gopdf.Rect{W: TicketW - 1, H: TicketH}, p.EventDescription)

	text("Noto-Bold", 26, 8, 22, T(p.Lang, "pdf.seat", seat))
	text("Noto", 8, 8, 34, room)
	text("Noto", 10, 8, TicketH-10, p.UserName)

	// ramka do wycięcia
	pdf.SetLineWidth(0.2)
	pdf.SetStrokeColor(0, 0, 0)
	pdf.RectFromUpperLeft(x, y, TicketW, TicketH)
	return nil
}

func sanitize(s string) string {
	return strings.NewReplacer(" ", "_", "/", "_", "\\", "_").Replace(s)
}

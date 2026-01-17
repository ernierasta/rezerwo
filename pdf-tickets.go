package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
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

	GridCols = 1
	GridRows = 4

	FontsDir = "media/fonts"
	TempDir  = "media/tmp"
)

type TicketParams struct {
	UserName         string
	UserURL          string
	EventName        string
	EventDescription string
	SeatNrs          []int
	Rooms            []string
	Background       *string
	PathAllInOne     string
	PathOne          string
}

// ====================== ENTRY POINT ======================

func GenerateTicketsPDF(p *TicketParams) ([]string, error) {
	var allFiles []string
	if len(p.SeatNrs) == 0 {
		return allFiles, fmt.Errorf("SeatNrs cannot be empty")
	}

	p.PathAllInOne = path.Join("media", p.UserURL, "pdf", "all")
	p.PathOne = path.Join("media", p.UserURL, "pdf", "one")

	if err := os.MkdirAll(p.PathAllInOne, 0755); err != nil {
		return allFiles, err
	}
	if err := os.MkdirAll(p.PathOne, 0755); err != nil {
		return allFiles, err
	}
	if err := os.MkdirAll(TempDir, 0755); err != nil {
		return allFiles, err
	}

	allFile, err := generateAllTicketsA4(p)
	if err != nil {
		return allFiles, err
	}

	allFiles = append(allFiles, allFile)

	oneFiles, err := generateSingleTickets(p)
	if err != nil {
		return allFiles, err
	}
	allFiles = append(allFiles, oneFiles...)

	return allFiles, nil
}

// ====================== ALL TICKETS ON A4 GRID ======================

func generateAllTicketsA4(p *TicketParams) (string, error) {
	fileName := fmt.Sprintf(
		"%s_%d_%d.pdf",
		sanitize(p.EventName),
		p.SeatNrs[0],
		p.SeatNrs[len(p.SeatNrs)-1],
	)

	pdf := newA4PDF()
	index := 0
	rl := len(p.Rooms)

	for i := range p.SeatNrs {
		if index%(GridCols*GridRows) == 0 {
			pdf.AddPage()
		}

		col := index % GridCols
		row := (index / GridCols) % GridRows

		x := float64(col) * TicketW
		y := float64(row) * TicketH

		room := ""
		if i < rl {
			room = p.Rooms[i]
		}

		renderTicket(&pdf, p, p.SeatNrs[i], room, x, y)
		drawCutMarks(&pdf, x, y, TicketW, TicketH)

		index++
	}

	return filepath.Join(p.PathAllInOne, fileName), pdf.WritePdf(filepath.Join(p.PathAllInOne, fileName))
}

// ====================== SINGLE TICKET PDFs ======================

func generateSingleTickets(p *TicketParams) ([]string, error) {
	var files []string
	for i := range p.SeatNrs {
		pdf := newTicketPDF()
		pdf.AddPage()

		room := ""
		if i < len(p.Rooms) {
			room = p.Rooms[i]
		}

		renderTicket(&pdf, p, p.SeatNrs[i], room, 0, 0)
		drawCutMarks(&pdf, 0, 0, TicketW, TicketH)

		fileName := fmt.Sprintf(
			"%s_%d.pdf",
			sanitize(p.EventName),
			p.SeatNrs[i],
		)

		if err := pdf.WritePdf(filepath.Join(p.PathOne, fileName)); err != nil {
			log.Printf("generateSingleTickets: seat: %v, %v", p.SeatNrs[i], err)
		} else {
			files = append(files, filepath.Join(p.PathOne, fileName))
		}

	}
	return files, nil
}

// ====================== RENDER SINGLE TICKET ======================

func renderTicket(pdf *gopdf.GoPdf, p *TicketParams, seat int, room string, x, y float64) error {

	// Background
	if p.Background != nil {
		if err := pdf.Image(*p.Background, x, y, &gopdf.Rect{W: TicketW, H: TicketH}); err != nil {
			fmt.Printf("Warning: cannot load background: %v\n", err)
		}
	}

	// QR code - temporary file
	qrPath := fmt.Sprintf(path.Join(TempDir, "qr_%s_%d.png"), sanitize(p.UserURL), seat)
	qr, _ := qrcode.NewWith(fmt.Sprintf("%s|%d",
		p.EventName, seat), qrcode.WithEncodingMode(qrcode.EncModeByte), qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionLow))
	w, err := standard.New(qrPath, standard.WithBorderWidth(1))
	if err != nil {
		return err
	}
	err = qr.Save(w)
	if err != nil {
		return err
	}
	err = pdf.Image(qrPath, x+TicketW-28, y+8, &gopdf.Rect{W: 20, H: 20})
	if err != nil {
		return err
	}
	// clean up - remove tmp file
	err = os.Remove(qrPath)
	if err != nil {
		return err
	}

	// Tytuł
	pdf.SetFont("Noto-Bold", "", 14)
	pdf.SetX(x + 8)
	pdf.SetY(y + 12)
	pdf.Cell(nil, p.EventName)

	// Opis
	pdf.SetFont("Noto", "", 9)
	pdf.SetX(x + 8)
	pdf.SetY(y + 18)
	pdf.MultiCell(&gopdf.Rect{W: TicketW - 1, H: TicketH}, p.EventDescription)

	// Numer miejsca
	pdf.SetFont("Noto-Bold", "", 26)
	pdf.SetX(x + 8)
	//pdf.SetY(y + TicketH - 35)
	pdf.SetY(y + 22)
	pdf.Cell(nil, fmt.Sprintf("MIEJSCE %d", seat))

	// Room name
	pdf.SetFont("Noto", "", 8)
	//pdf.SetX(pdf.GetX() + 2)
	//pdf.SetY(pdf.GetY() + 7)
	pdf.SetX(x + 8)
	pdf.SetY(y + 34)
	pdf.Cell(nil, room)

	// Organization (user) name
	pdf.SetFont("Noto", "", 10)
	pdf.SetX(x + 8)
	pdf.SetY(y + TicketH - 10)
	pdf.Cell(nil, p.UserName)

	return nil
}

// ====================== CUTTING MARKS / BORDER ======================

func drawCutMarks(pdf *gopdf.GoPdf, x, y, w, h float64) {
	pdf.SetLineWidth(0.2)
	pdf.SetStrokeColor(0, 0, 0) // czarny
	pdf.RectFromUpperLeft(x, y, w, h)
}

// ====================== PDF SETUP ======================

func newA4PDF() gopdf.GoPdf {
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{
		PageSize: gopdf.Rect{W: A4W, H: A4H},
		Unit:     gopdf.UnitMM,
	})
	loadFonts(&pdf)
	return pdf
}

func newTicketPDF() gopdf.GoPdf {
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{
		PageSize: gopdf.Rect{W: TicketW, H: TicketH},
		Unit:     gopdf.UnitMM,
	})
	loadFonts(&pdf)
	return pdf
}

func loadFonts(pdf *gopdf.GoPdf) {
	if err := pdf.AddTTFFont("Noto", path.Join(FontsDir, "NotoSans-Regular.ttf")); err != nil {
		panic(fmt.Errorf("cannot load font Noto: %v", err))
	}
	if err := pdf.AddTTFFont("Noto-Bold", path.Join(FontsDir, "NotoSans-Bold.ttf")); err != nil {
		panic(fmt.Errorf("cannot load font Noto-Bold: %v", err))
	}
}

// ====================== HELPERS ======================

func sanitize(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	return s
}

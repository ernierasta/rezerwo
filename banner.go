package main

// banner.go: bannery imprez i formularzy - zapis w bazie, obrazek na stronie
// gościa, wgrywanie i lista plików organizacji dla panelu.
//
// W bazie (events.mainbanner, events.room1banner.., formtemplates.banner)
// jest nazwa pliku z katalogu media/<organizacja>/ i opcjonalnie szerokość:
// "banner.png" (na całą szerokość) albo "banner.png;50%", albo
// "banner.png;full" (na pełną szerokość strony, tylko główny banner
// imprezy i formularza - pagehead.go, GuestHead). Stare wpisy
// "banner.png;1600;400" podawały wymiary obrazka, które nigdy nie były
// używane - czytamy je jako "na całą szerokość".

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gorilla/sessions"
)

// bannerWidths - dozwolone szerokości bannera w % szerokości strony;
// 100 = "auto", wartość domyślna.
var bannerWidths = []int{100, 75, 50, 33}

// bannerExts - rozszerzenia obrazków, które można wgrać jako banner.
var bannerExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true}

// BANNERMAXSIZE - największy plik bannera (bajty).
const BANNERMAXSIZE = 8 << 20

type Banner struct {
	File  string // ścieżka względem media/<organizacja>/
	Width int    // % szerokości strony, jedna z bannerWidths
	Full  bool   // na całą stronę (wtedy Width = 100)
}

// BANNERFULL - szerokość "cała strona" w zapisie bannera: pełna szerokość
// strony, także w formularzu, gdzie "auto" to szerokość nagłówka.
const BANNERFULL = "full"

// ParseBanner czyta banner zapisany w bazie.
func ParseBanner(s string) Banner {
	parts := strings.Split(strings.TrimSpace(s), ";")
	b := Banner{File: strings.TrimSpace(parts[0]), Width: 100}
	if len(parts) == 2 && strings.TrimSpace(parts[1]) == BANNERFULL {
		b.Full = true
	} else if len(parts) == 2 {
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(parts[1]), "%"))
		if err == nil && validBannerWidth(n) {
			b.Width = n
		}
	}
	return b
}

func validBannerWidth(n int) bool {
	for _, w := range bannerWidths {
		if w == n {
			return true
		}
	}
	return false
}

// String - postać do zapisu w bazie.
func (b Banner) String() string {
	if b.File == "" {
		return ""
	}
	if b.Full {
		return b.File + ";" + BANNERFULL
	}
	if b.Width == 100 || !validBannerWidth(b.Width) {
		return b.File
	}
	return fmt.Sprintf("%s;%d%%", b.File, b.Width)
}

// ImgHTML - obrazek bannera na stronę gościa (pusty, gdy nie ma pliku).
func (b Banner) ImgHTML(userURL string) template.HTML {
	if b.File == "" {
		return ""
	}
	src := path.Join("/", MEDIAROOT, userURL, b.File)
	style := ""
	if b.Width != 100 && validBannerWidth(b.Width) {
		style = fmt.Sprintf(` style="width:%d%%"`, b.Width)
	}
	return template.HTML(fmt.Sprintf(`<img src="%s" alt=""%s>`, template.HTMLEscapeString(src), style))
}

// cleanBanner porządkuje wartość z panelu: znany format, bez wychodzenia
// poza katalog organizacji.
func cleanBanner(s string) string {
	b := ParseBanner(s)
	f := path.Clean("/" + b.File)
	if f == "/" {
		return ""
	}
	b.File = strings.TrimPrefix(f, "/")
	return b.String()
}

// bannerFileRe - dozwolone znaki w nazwie wgranego pliku.
var bannerFileRe = regexp.MustCompile(`[^a-z0-9._-]+`)

// bannerFileName - bezpieczna nazwa pliku z nazwy podanej przez przeglądarkę:
// małe litery, bez polskich znaków i spacji ("Bal 2026 – plakat.PNG" ->
// "bal-2026-plakat.png").
func bannerFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	ext := strings.ToLower(filepath.Ext(name))
	base := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
	base = strings.NewReplacer(
		"ą", "a", "ć", "c", "ę", "e", "ł", "l", "ń", "n", "ó", "o", "ś", "s", "ź", "z", "ż", "z",
		"á", "a", "č", "c", "ď", "d", "é", "e", "ě", "e", "í", "i", "ň", "n", "ř", "r",
		"š", "s", "ť", "t", "ú", "u", "ů", "u", "ý", "y", "ž", "z",
	).Replace(base)
	base = strings.Trim(bannerFileRe.ReplaceAllString(base, "-"), "-.")
	if base == "" {
		base = "banner"
	}
	return base + ext
}

// freeFileName dopisuje -2, -3... gdy plik o tej nazwie już jest, żeby nowy
// banner nie podmienił obrazka używanego przez inną imprezę.
func freeFileName(dir, name string) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	candidate := name
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(dir, candidate)); os.IsNotExist(err) {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d%s", base, i, ext)
	}
}

type nBannerFile struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// BannerFilesAPI - GET /api/media/banners: obrazki z katalogu organizacji
// (bez podkatalogów email/, qrcode/, pdf/), od najnowszego.
// POST /api/media/banners (multipart, pole "file"): wgrywa nowy obrazek.
func BannerFilesAPI(db *DB, cs *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := adminUser(w, r, db, cs)
		if !ok {
			return
		}
		lang := PanelLang(r)
		dir := filepath.Join(MEDIAROOT, user.URL)

		if r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, BANNERMAXSIZE+1<<20)
			file, hdr, err := r.FormFile("file")
			if err != nil {
				msg := err.Error()
				var tooBig *http.MaxBytesError
				if errors.As(err, &tooBig) {
					msg = T(lang, "banner.too_big")
				}
				jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": msg})
				return
			}
			defer file.Close()
			name := bannerFileName(hdr.Filename)
			if !bannerExts[filepath.Ext(name)] {
				jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "banner.bad_type")})
				return
			}
			if hdr.Size > BANNERMAXSIZE {
				jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "banner.too_big")})
				return
			}
			// zawartość musi być obrazkiem, nie tylko nazwa
			head := make([]byte, 512)
			n, _ := io.ReadFull(file, head)
			if !strings.HasPrefix(http.DetectContentType(head[:n]), "image/") {
				jsonMsg(w, http.StatusBadRequest, map[string]string{"msg": T(lang, "banner.bad_type")})
				return
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				log.Printf("BannerFilesAPI: mkdir %s: %v", dir, err)
				jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
				return
			}
			name = freeFileName(dir, name)
			out, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if err != nil {
				log.Printf("BannerFilesAPI: create %s: %v", name, err)
				jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
				return
			}
			_, err = out.Write(head[:n])
			if err == nil {
				_, err = io.Copy(out, file)
			}
			if cerr := out.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				os.Remove(filepath.Join(dir, name))
				log.Printf("BannerFilesAPI: write %s: %v", name, err)
				jsonMsg(w, http.StatusTeapot, map[string]string{"msg": err.Error()})
				return
			}
			jsonMsg(w, http.StatusOK, nBannerFile{Name: name, URL: path.Join("/", MEDIAROOT, user.URL, name)})
			return
		}

		entries, err := os.ReadDir(dir)
		if err != nil && !os.IsNotExist(err) {
			log.Printf("BannerFilesAPI: read %s: %v", dir, err)
		}
		type item struct {
			nBannerFile
			mod int64
		}
		var items []item
		for _, e := range entries {
			if e.IsDir() || !bannerExts[strings.ToLower(filepath.Ext(e.Name()))] {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			items = append(items, item{nBannerFile{e.Name(), path.Join("/", MEDIAROOT, user.URL, e.Name())}, info.ModTime().Unix()})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].mod > items[j].mod })
		files := make([]nBannerFile, 0, len(items))
		for _, it := range items {
			files = append(files, it.nBannerFile)
		}
		jsonMsg(w, http.StatusOK, files)
	}
}

// nBannerPick - dane wyboru bannera w panelu (szablon "bannerpick").
type nBannerPick struct {
	Label string
	Field string // nazwa pola w sekcji (data-field)
	Value string // wartość z bazy
	Base  string // adres katalogu organizacji, np. /media/mskarwina/
	Full  bool   // można wybrać "cała strona" (główny banner)
}

func bannerPick(label, field, value string, user User) nBannerPick {
	return nBannerPick{Label: label, Field: field, Value: cleanBanner(value), Base: path.Join("/", MEDIAROOT, user.URL) + "/"}
}

// mainBannerPick - wybór głównego bannera, z szerokością "cała strona".
func mainBannerPick(label, field, value string, user User) nBannerPick {
	p := bannerPick(label, field, value, user)
	p.Full = true
	return p
}

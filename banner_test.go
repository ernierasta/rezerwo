package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBanner(t *testing.T) {
	cases := []struct {
		in   string
		want Banner
		out  string
	}{
		{"", Banner{File: "", Width: 100}, ""},
		{"banner.png", Banner{File: "banner.png", Width: 100}, "banner.png"},
		{"banner.png;50%", Banner{File: "banner.png", Width: 50}, "banner.png;50%"},
		{"banner.png;75", Banner{File: "banner.png", Width: 75}, "banner.png;75%"},
		{"banner.png;100%", Banner{File: "banner.png", Width: 100}, "banner.png"},
		{"banner.png;42%", Banner{File: "banner.png", Width: 100}, "banner.png"},
		// stary zapis z wymiarami obrazka: wymiary nigdy nie działały
		{"banner-test.png;1600;400", Banner{File: "banner-test.png", Width: 100}, "banner-test.png"},
		{"banner2026.jpeg;1;1", Banner{File: "banner2026.jpeg", Width: 100}, "banner2026.jpeg"},
		{" test/banner.png ", Banner{File: "test/banner.png", Width: 100}, "test/banner.png"},
		{"banner.png;full", Banner{File: "banner.png", Width: 100, Full: true}, "banner.png;full"},
	}
	for _, c := range cases {
		got := ParseBanner(c.in)
		if got != c.want {
			t.Errorf("ParseBanner(%q) = %+v, want %+v", c.in, got, c.want)
		}
		if got.String() != c.out {
			t.Errorf("ParseBanner(%q).String() = %q, want %q", c.in, got.String(), c.out)
		}
	}
}

func TestBannerImgHTML(t *testing.T) {
	if got := ParseBanner("").ImgHTML("org"); got != "" {
		t.Errorf("empty banner: %q", got)
	}
	if got := string(ParseBanner("b.png").ImgHTML("org")); got != `<img src="/media/org/b.png" alt="">` {
		t.Errorf("auto: %q", got)
	}
	if got := string(ParseBanner("b.png;33%").ImgHTML("org")); got != `<img src="/media/org/b.png" alt="" style="width:33%">` {
		t.Errorf("33%%: %q", got)
	}
	if got := string(ParseBanner(`a"b.png`).ImgHTML("org")); got != `<img src="/media/org/a&#34;b.png" alt="">` {
		t.Errorf("escaping: %q", got)
	}
}

func TestCleanBanner(t *testing.T) {
	cases := map[string]string{
		"":                      "",
		"b.png;50%":             "b.png;50%",
		"../../etc/passwd":      "etc/passwd",
		"/abs.png":              "abs.png",
		"test/../b.png;75%":     "b.png;75%",
		"banner-x.png;1600;400": "banner-x.png",
		"..":                    "",
	}
	for in, want := range cases {
		if got := cleanBanner(in); got != want {
			t.Errorf("cleanBanner(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBannerFileName(t *testing.T) {
	cases := map[string]string{
		"Bal 2026 – plakat.PNG": "bal-2026-plakat.png",
		"Źdźbło żółte.jpg":      "zdzblo-zolte.jpg",
		"Žluťoučký kůň.webp":    "zlutoucky-kun.webp",
		`C:\Users\x\baner.png`:  "baner.png",
		"../../evil.png":        "evil.png",
		".png":                  "banner.png",
		"a;50%.png":             "a-50.png",
	}
	for in, want := range cases {
		if got := bannerFileName(in); got != want {
			t.Errorf("bannerFileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFreeFileName(t *testing.T) {
	dir := t.TempDir()
	if got := freeFileName(dir, "b.png"); got != "b.png" {
		t.Fatalf("free: %q", got)
	}
	os.WriteFile(filepath.Join(dir, "b.png"), nil, 0o644)
	os.WriteFile(filepath.Join(dir, "b-2.png"), nil, 0o644)
	if got := freeFileName(dir, "b.png"); got != "b-3.png" {
		t.Fatalf("taken: %q", got)
	}
}

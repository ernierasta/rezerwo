package main

import "testing"

func TestContrastInk(t *testing.T) {
	for bg, want := range map[string]string{
		"#ffffff": "#212529", "#eaeaea": "#212529", "#e5a50a": "#212529", "#99c1f1": "#212529",
		"#9a1620": "#FFFFFF", "#1657a6": "#FFFFFF", "#63452c": "#FFFFFF", "#000": "#FFFFFF",
		"red": "", "": "",
	} {
		if got := contrastInk(bg); got != want {
			t.Errorf("contrastInk(%q) = %q, want %q", bg, got, want)
		}
	}
}

func TestThemeDefault(t *testing.T) {
	if ThemeKey("") != "klasyczny" || ThemeKey("xyz") != "klasyczny" || ThemeKey("noc") != "noc" {
		t.Error("unexpected ThemeKey")
	}
	if guestThemes[0].Key != DEFAULTTHEME {
		t.Error("default theme should be first")
	}
}

func TestThemeLogo(t *testing.T) {
	for _, c := range []struct {
		key  string
		logo bool
		want string
	}{
		{"pzko", false, "pzko"},
		{"pzko", true, "pzko;logo"},
		{"macierz;logo", true, "macierz;logo"},
		{"noc", true, "noc"}, // temat bez znaku
		{"xyz", true, DEFAULTTHEME},
	} {
		if got := ThemeValue(c.key, c.logo); got != c.want {
			t.Errorf("ThemeValue(%q, %v) = %q, want %q", c.key, c.logo, got, c.want)
		}
	}
	if ThemeKey("pzko;logo") != "pzko" || !ThemeLogo("pzko;logo") || ThemeLogo("pzko") || ThemeLogo("noc;logo") {
		t.Error("unexpected ThemeKey/ThemeLogo")
	}
	if ThemeNorm("pzko;logo") != "pzko;logo" || ThemeNorm("noc;logo") != "noc" {
		t.Error("unexpected ThemeNorm")
	}
}

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

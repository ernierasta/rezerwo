package main

// Odstępy między polami formularza gościa, niezależne od tematu kolorów
// (formtemplates.density, tools/migration-density.sql). Pusty klucz to
// zwykłe odstępy; pozostałe dają klasę density-<klucz> na .gform strony
// gościa i podglądu w designerze pól (scss/ui.scss).
var densities = []string{"", "compact", "tight"}

// DensityKey zwraca znany klucz odstępów albo "" (zwykłe).
func DensityKey(key string) string {
	for _, d := range densities {
		if d != "" && d == key {
			return d
		}
	}
	return ""
}

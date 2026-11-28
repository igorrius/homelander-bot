package httpapi

import "testing"

func TestShortName(t *testing.T) {
	for in, want := range map[string]string{
		"Homelander":                "Homelander",
		"Тестова громада":           "Тестова",
		"Green Garden Society":      "Green",
		"Супердовгоназвана громада": "Супердовгона",
		"Exactly12chr":              "Exactly12chr",
		"Sun valley 12":             "Sun",
	} {
		if got := shortName(in); got != want {
			t.Errorf("shortName(%q) = %q, want %q", in, got, want)
		}
	}
}

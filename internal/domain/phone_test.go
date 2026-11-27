package domain

import (
	"errors"
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	ok := map[string]string{
		"050 000 00 01":       "+380500000001",
		"+380500000001":       "+380500000001",
		"380500000001":        "+380500000001",
		"0500000001":          "+380500000001",
		"(050) 000-00-01":     "+380500000001",
		" +38 050 000 00 01 ": "+380500000001",
		"+48 600 000 001":     "+48600000001",
	}
	for in, want := range ok {
		got, err := NormalizePhone(in, "UA")
		if err != nil || got != want {
			t.Errorf("NormalizePhone(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "12", "abc", "+380 12"} {
		if got, err := NormalizePhone(in, "UA"); !errors.Is(err, ErrInvalidPhone) {
			t.Errorf("NormalizePhone(%q) = %q, %v; want ErrInvalidPhone", in, got, err)
		}
	}
}

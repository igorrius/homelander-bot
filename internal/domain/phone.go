package domain

import (
	"errors"
	"strings"
	"unicode"

	"github.com/nyaruka/phonenumbers"
)

// ErrInvalidPhone means the text is not a valid phone number.
var ErrInvalidPhone = errors.New("invalid_phone")

// NormalizePhone returns raw in E.164 form (+380501234567). Numbers without a
// country code are read in region. Digits that already start with a country
// code but lack the "+" (as Telegram sends contacts) are accepted too.
func NormalizePhone(raw, region string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrInvalidPhone
	}
	candidates := []string{raw}
	if !strings.HasPrefix(raw, "+") && onlyDigitsAndSeparators(raw) {
		candidates = append(candidates, "+"+raw)
	}
	for _, c := range candidates {
		n, err := phonenumbers.Parse(c, region)
		if err == nil && phonenumbers.IsValidNumber(n) {
			return phonenumbers.Format(n, phonenumbers.E164), nil
		}
	}
	return "", ErrInvalidPhone
}

func onlyDigitsAndSeparators(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) && !strings.ContainsRune(" -().", r) {
			return false
		}
	}
	return true
}

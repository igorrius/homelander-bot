// Package i18n renders backend and bot texts in the person's language.
package i18n

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// Bundle holds the loaded messages of every supported locale.
type Bundle struct {
	bundle    *goi18n.Bundle
	def       string
	supported []string
}

// New loads <locale>.json for every supported locale from fsys.
func New(fsys fs.FS, defaultLocale string, supported []string) (*Bundle, error) {
	if !slices.Contains(supported, defaultLocale) {
		return nil, fmt.Errorf("default locale %q is not supported", defaultLocale)
	}
	b := goi18n.NewBundle(language.MustParse(defaultLocale))
	b.RegisterUnmarshalFunc("json", json.Unmarshal)
	for _, loc := range supported {
		if _, err := b.LoadMessageFileFS(fsys, loc+".json"); err != nil {
			return nil, fmt.Errorf("load locale %s: %w", loc, err)
		}
	}
	return &Bundle{bundle: b, def: defaultLocale, supported: slices.Clone(supported)}, nil
}

// Default returns the default locale.
func (b *Bundle) Default() string { return b.def }

// Pick returns the first supported locale among candidates ("en-GB" → "en"),
// else the default.
func (b *Bundle) Pick(candidates ...string) string {
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		tag, err := language.Parse(c)
		if err != nil {
			continue
		}
		base, _ := tag.Base()
		if slices.Contains(b.supported, base.String()) {
			return base.String()
		}
	}
	return b.def
}

// T renders message id in locale. data fills {{.Name}} templates; a non-nil
// count selects the plural form. A message missing in locale falls back to the
// default locale; missing everywhere, T returns id.
func (b *Bundle) T(locale, id string, data map[string]any, count any) string {
	cfg := &goi18n.LocalizeConfig{MessageID: id, TemplateData: data, PluralCount: count}
	// go-i18n picks one language per localizer and does not fall back per
	// message, so the default locale is tried on its own.
	for _, l := range []string{b.Pick(locale), b.def} {
		if s, err := goi18n.NewLocalizer(b.bundle, l).Localize(cfg); err == nil && s != "" {
			return s
		}
	}
	return id
}

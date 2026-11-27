// Package locales embeds the backend and bot texts, one JSON file per language.
package locales

import "embed"

// FS holds uk.json, en.json, ...
//
//go:embed *.json
var FS embed.FS

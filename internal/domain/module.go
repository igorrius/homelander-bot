package domain

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// ModuleDef is a built-in module in the registry: Go code, no dynamic loading.
type ModuleDef struct {
	Code string // also its data key prefix: module "feed" owns every "feed." key
	// Translation keys of the module's name and description (web locales).
	NameKey, DescriptionKey string
	DefaultEnabled          bool
	// Nav are the module's cabinet navigation entries.
	Nav []ModuleNav
	// Ensure creates the module's own data items when they are missing; it is
	// called when the module is turned on and at startup while it is on. May be nil.
	Ensure func(ctx context.Context) error
}

// ModuleNav is one cabinet navigation entry of a module.
type ModuleNav struct {
	Key, Path, LabelKey string
	Icon                string // an icon name the web app maps, e.g. "news"
}

// ModuleState is what an admin chose for a module; a missing row is the default.
type ModuleState struct {
	Code      string
	Enabled   bool
	Settings  json.RawMessage
	UpdatedBy *int64
	UpdatedAt time.Time
}

// ModuleRepo stores module states.
type ModuleRepo interface {
	// Get returns ErrNotFound when the module has no row yet.
	Get(ctx context.Context, code string) (ModuleState, error)
	Save(ctx context.Context, s ModuleState) error
}

// ModuleView is a registry entry with its current state.
type ModuleView struct {
	ModuleDef
	Enabled   bool
	Settings  json.RawMessage
	UpdatedBy *int64
	UpdatedAt *time.Time // nil while the module has its default
}

// ModuleOwner is the data item owner of a module's own items.
func ModuleOwner(code string) string { return "module:" + code }

// ModuleOf returns the module code of an owner ("module:feed" → "feed"), or "".
func ModuleOf(owner string) string {
	code, _ := strings.CutPrefix(owner, "module:")
	if code == owner {
		return ""
	}
	return code
}

// ModuleTarget is the audit target of a module.
func ModuleTarget(code string) string { return "module:" + code }

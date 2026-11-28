package domain

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// ModuleService knows the built-in modules and whether each is on.
type ModuleService struct {
	R    Repos
	Now  Clock
	Defs []ModuleDef
}

func (s *ModuleService) def(code string) (ModuleDef, bool) {
	for _, d := range s.Defs {
		if d.Code == code {
			return d, true
		}
	}
	return ModuleDef{}, false
}

func (s *ModuleService) view(ctx context.Context, d ModuleDef) (ModuleView, error) {
	v := ModuleView{ModuleDef: d, Enabled: d.DefaultEnabled, Settings: json.RawMessage("{}")}
	st, err := s.R.Modules.Get(ctx, d.Code)
	switch {
	case errors.Is(err, ErrNotFound):
		return v, nil
	case err != nil:
		return ModuleView{}, err
	}
	at := st.UpdatedAt
	v.Enabled, v.UpdatedBy, v.UpdatedAt = st.Enabled, st.UpdatedBy, &at
	if len(st.Settings) > 0 {
		v.Settings = st.Settings
	}
	return v, nil
}

// List returns every module in registry order.
func (s *ModuleService) List(ctx context.Context) ([]ModuleView, error) {
	out := make([]ModuleView, 0, len(s.Defs))
	for _, d := range s.Defs {
		v, err := s.view(ctx, d)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// Get returns one module; ErrNotFound for an unknown code.
func (s *ModuleService) Get(ctx context.Context, code string) (ModuleView, error) {
	d, ok := s.def(code)
	if !ok {
		return ModuleView{}, ErrNotFound
	}
	return s.view(ctx, d)
}

// Enabled reports whether a module is on; an unknown module is off.
func (s *ModuleService) Enabled(ctx context.Context, code string) (bool, error) {
	v, err := s.Get(ctx, code)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return v.Enabled, err
}

// Require returns ErrModuleOff unless the module is on.
func (s *ModuleService) Require(ctx context.Context, code string) error {
	on, err := s.Enabled(ctx, code)
	if err == nil && !on {
		err = ErrModuleOff
	}
	return err
}

func (s *ModuleService) save(ctx context.Context, v ModuleView, a Actor) error {
	return s.R.Modules.Save(ctx, ModuleState{Code: v.Code, Enabled: v.Enabled, Settings: v.Settings, UpdatedBy: a.UserID, UpdatedAt: s.Now()})
}

func (s *ModuleService) audit(ctx context.Context, a Actor, action, code string, details map[string]any) error {
	return s.R.Audit.Add(ctx, AuditEntry{At: s.Now(), ActorUserID: a.UserID, Action: action,
		Target: ModuleTarget(code), Details: a.AuditDetails(details), IP: a.IP})
}

// SetEnabled turns a module on or off. Turning it on creates its data items.
func (s *ModuleService) SetEnabled(ctx context.Context, code string, on bool, a Actor) (ModuleView, error) {
	var out ModuleView
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		v, err := s.Get(ctx, code)
		if err != nil {
			return err
		}
		was := v.Enabled
		v.Enabled = on
		if err := s.save(ctx, v, a); err != nil {
			return err
		}
		if on && v.Ensure != nil {
			if err := v.Ensure(ctx); err != nil {
				return err
			}
		}
		if was != on {
			action := ActModuleDisabled
			if on {
				action = ActModuleEnabled
			}
			if err := s.audit(ctx, a, action, code, nil); err != nil {
				return err
			}
		}
		out, err = s.Get(ctx, code)
		return err
	})
	return out, err
}

// SaveSettings stores a module's settings, already validated by the module.
// Call it inside the module's transaction.
func (s *ModuleService) SaveSettings(ctx context.Context, code string, raw json.RawMessage, a Actor) error {
	v, err := s.Get(ctx, code)
	if err != nil {
		return err
	}
	v.Settings = raw
	if err := s.save(ctx, v, a); err != nil {
		return err
	}
	var details map[string]any
	_ = json.Unmarshal(raw, &details)
	return s.audit(ctx, a, ActModuleSettingsSaved, code, details)
}

// EnsureAll creates the missing data items of every module that is on (at startup).
func (s *ModuleService) EnsureAll(ctx context.Context) error {
	for _, d := range s.Defs {
		on, err := s.Enabled(ctx, d.Code)
		if err != nil {
			return err
		}
		if on && d.Ensure != nil {
			if err := s.R.Tx.InTx(ctx, d.Ensure); err != nil {
				return err
			}
		}
	}
	return nil
}

// Reserved reports whether key lies under a module's prefix ("feed.…"),
// which only that module may create.
func (s *ModuleService) Reserved(key string) bool {
	if s == nil {
		return false
	}
	for _, d := range s.Defs {
		if strings.HasPrefix(key, d.Code+".") {
			return true
		}
	}
	return false
}

// DisabledOwners returns the data owners of modules that are off.
func (s *ModuleService) DisabledOwners(ctx context.Context) (map[string]bool, error) {
	out := map[string]bool{}
	if s == nil {
		return out, nil
	}
	for _, d := range s.Defs {
		on, err := s.Enabled(ctx, d.Code)
		if err != nil {
			return nil, err
		}
		if !on {
			out[ModuleOwner(d.Code)] = true
		}
	}
	return out, nil
}

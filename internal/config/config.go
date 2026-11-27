// Package config loads the process configuration from environment variables.
// Every secret and every per-deployment value comes from here; nothing about a
// particular deployment is kept in the repository.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"time"
)

// UpdatesMode selects how the bot receives Telegram updates.
type UpdatesMode string

// Supported update modes.
const (
	UpdatesPolling UpdatesMode = "polling"
	UpdatesWebhook UpdatesMode = "webhook"
)

// SupportedLocales lists the languages the system ships texts for.
var SupportedLocales = []string{"uk", "en"}

const minSecretLen = 32

// Config is the validated process configuration.
type Config struct {
	AppSecret             []byte
	PublicURL             *url.URL
	HTTPAddr              string
	DatabasePath          string
	TelegramBotToken      string
	TelegramUpdates       UpdatesMode
	TelegramWebhookSecret string
	BootstrapAdminPhone   string
	DefaultLocale         string
	DefaultPhoneRegion    string
	SessionIdleTTL        time.Duration
	SessionMaxTTL         time.Duration
	LogLevel              slog.Level
	// TrustProxy takes the client IP from the rightmost X-Forwarded-For entry
	// (set it behind Railway's proxy).
	TrustProxy bool
}

// Load reads and validates the configuration. All problems are reported
// together so a deployment can be fixed in one pass.
func Load(getenv func(string) string) (Config, error) {
	var errs []error
	get := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}
	need := func(key string) string {
		v := get(key, "")
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", key))
		}
		return v
	}
	duration := func(key, def string) time.Duration {
		d, err := time.ParseDuration(get(key, def))
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("%s must be a positive duration such as 12h", key))
		}
		return d
	}

	cfg := Config{
		DatabasePath:        get("DATABASE_PATH", "./data/homelander.db"),
		TelegramBotToken:    need("TELEGRAM_BOT_TOKEN"),
		BootstrapAdminPhone: get("BOOTSTRAP_ADMIN_PHONE", ""),
		DefaultLocale:       get("DEFAULT_LOCALE", "uk"),
		DefaultPhoneRegion:  strings.ToUpper(get("DEFAULT_PHONE_REGION", "UA")),
		SessionIdleTTL:      duration("SESSION_IDLE_TTL", "12h"),
		SessionMaxTTL:       duration("SESSION_MAX_TTL", "168h"),
	}

	if s := need("APP_SECRET"); s != "" {
		if len(s) < minSecretLen {
			errs = append(errs, fmt.Errorf("APP_SECRET must be at least %d bytes", minSecretLen))
		}
		cfg.AppSecret = []byte(s)
	}

	if raw := need("PUBLIC_URL"); raw != "" {
		u, err := parsePublicURL(raw)
		if err != nil {
			errs = append(errs, err)
		}
		cfg.PublicURL = u
	}

	cfg.HTTPAddr = get("HTTP_ADDR", "")
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
		if p := get("PORT", ""); p != "" {
			cfg.HTTPAddr = ":" + p
		}
	}

	switch mode := UpdatesMode(get("TELEGRAM_UPDATES", string(UpdatesPolling))); mode {
	case UpdatesPolling:
		cfg.TelegramUpdates = mode
	case UpdatesWebhook:
		cfg.TelegramUpdates = mode
		cfg.TelegramWebhookSecret = need("TELEGRAM_WEBHOOK_SECRET")
	default:
		errs = append(errs, fmt.Errorf("TELEGRAM_UPDATES must be %q or %q", UpdatesPolling, UpdatesWebhook))
	}

	if !slices.Contains(SupportedLocales, cfg.DefaultLocale) {
		errs = append(errs, fmt.Errorf("DEFAULT_LOCALE must be one of %v", SupportedLocales))
	}

	if cfg.SessionIdleTTL > 0 && cfg.SessionMaxTTL > 0 && cfg.SessionMaxTTL < cfg.SessionIdleTTL {
		errs = append(errs, errors.New("SESSION_MAX_TTL must not be shorter than SESSION_IDLE_TTL"))
	}

	switch v := strings.ToLower(get("TRUST_PROXY", "false")); v {
	case "true", "1", "yes":
		cfg.TrustProxy = true
	case "false", "0", "no":
	default:
		errs = append(errs, errors.New("TRUST_PROXY must be true or false"))
	}

	if err := cfg.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, errors.New("LOG_LEVEL must be debug, info, warn or error"))
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("configuration: %w", err)
	}
	return cfg, nil
}

func parsePublicURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, errors.New("PUBLIC_URL must be an absolute URL such as https://example.com")
	}
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"
	if u.Scheme != "https" && (u.Scheme != "http" || !local) {
		return nil, errors.New("PUBLIC_URL must use https (http is allowed only for localhost)")
	}
	return u, nil
}

// LogValue keeps secrets out of logs.
func (c Config) LogValue() slog.Value {
	const redacted = "[redacted]"
	publicURL := ""
	if c.PublicURL != nil {
		publicURL = c.PublicURL.String()
	}
	secret := func(v string) string {
		if v == "" {
			return ""
		}
		return redacted
	}
	return slog.GroupValue(
		slog.String("AppSecret", secret(string(c.AppSecret))),
		slog.String("PublicURL", publicURL),
		slog.String("HTTPAddr", c.HTTPAddr),
		slog.String("DatabasePath", c.DatabasePath),
		slog.String("TelegramBotToken", secret(c.TelegramBotToken)),
		slog.String("TelegramUpdates", string(c.TelegramUpdates)),
		slog.String("TelegramWebhookSecret", secret(c.TelegramWebhookSecret)),
		slog.Bool("BootstrapAdminPhoneSet", c.BootstrapAdminPhone != ""),
		slog.String("DefaultLocale", c.DefaultLocale),
		slog.String("DefaultPhoneRegion", c.DefaultPhoneRegion),
		slog.Duration("SessionIdleTTL", c.SessionIdleTTL),
		slog.Duration("SessionMaxTTL", c.SessionMaxTTL),
		slog.String("LogLevel", c.LogLevel.String()),
		slog.Bool("TrustProxy", c.TrustProxy),
	)
}

package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

const (
	testSecret  = "0123456789abcdef0123456789abcdef-secret"
	testToken   = "123456:test-bot-token-value"
	testWebhook = "webhook-secret-value"
)

func env(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func required(extra map[string]string) map[string]string {
	m := map[string]string{
		"APP_SECRET":         testSecret,
		"PUBLIC_URL":         "https://example.com",
		"TELEGRAM_BOT_TOKEN": testToken,
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func mustLoad(t *testing.T, kv map[string]string) Config {
	t.Helper()
	cfg, err := Load(env(kv))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func mustFail(t *testing.T, kv map[string]string, wants ...string) {
	t.Helper()
	_, err := Load(env(kv))
	if err == nil {
		t.Fatalf("Load succeeded, want error mentioning %v", wants)
	}
	for _, w := range wants {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error %q does not mention %q", err, w)
		}
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg := mustLoad(t, required(nil))
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.DatabasePath != "./data/homelander.db" {
		t.Errorf("DatabasePath = %q", cfg.DatabasePath)
	}
	if cfg.TelegramUpdates != UpdatesPolling {
		t.Errorf("TelegramUpdates = %q", cfg.TelegramUpdates)
	}
	if cfg.DefaultLocale != "uk" {
		t.Errorf("DefaultLocale = %q", cfg.DefaultLocale)
	}
	if cfg.DefaultPhoneRegion != "UA" {
		t.Errorf("DefaultPhoneRegion = %q", cfg.DefaultPhoneRegion)
	}
	if cfg.SessionIdleTTL != 12*time.Hour {
		t.Errorf("SessionIdleTTL = %v", cfg.SessionIdleTTL)
	}
	if cfg.SessionMaxTTL != 168*time.Hour {
		t.Errorf("SessionMaxTTL = %v", cfg.SessionMaxTTL)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v", cfg.LogLevel)
	}
	if string(cfg.AppSecret) != testSecret || cfg.TelegramBotToken != testToken {
		t.Errorf("secrets not loaded")
	}
	if cfg.PublicURL.String() != "https://example.com" {
		t.Errorf("PublicURL = %v", cfg.PublicURL)
	}
}

func TestLoadUsesPort(t *testing.T) {
	cfg := mustLoad(t, required(map[string]string{"PORT": "9000"}))
	if cfg.HTTPAddr != ":9000" {
		t.Errorf("HTTPAddr = %q, want :9000", cfg.HTTPAddr)
	}
	cfg = mustLoad(t, required(map[string]string{"PORT": "9000", "HTTP_ADDR": ":7000"}))
	if cfg.HTTPAddr != ":7000" {
		t.Errorf("HTTPAddr = %q, want :7000", cfg.HTTPAddr)
	}
}

func TestLoadReportsAllMissing(t *testing.T) {
	mustFail(t, map[string]string{}, "APP_SECRET", "PUBLIC_URL", "TELEGRAM_BOT_TOKEN")
}

func TestLoadRejectsShortSecret(t *testing.T) {
	mustFail(t, required(map[string]string{"APP_SECRET": strings.Repeat("x", 31)}), "APP_SECRET")
	mustLoad(t, required(map[string]string{"APP_SECRET": strings.Repeat("x", 32)}))
}

func TestLoadPublicURL(t *testing.T) {
	mustFail(t, required(map[string]string{"PUBLIC_URL": "http://example.com"}), "PUBLIC_URL")
	mustFail(t, required(map[string]string{"PUBLIC_URL": "example.com"}), "PUBLIC_URL")
	mustLoad(t, required(map[string]string{"PUBLIC_URL": "http://localhost:8080"}))
	mustLoad(t, required(map[string]string{"PUBLIC_URL": "http://127.0.0.1:8080"}))
	mustLoad(t, required(map[string]string{"PUBLIC_URL": "https://example.com"}))
}

func TestLoadWebhookNeedsSecret(t *testing.T) {
	mustFail(t, required(map[string]string{"TELEGRAM_UPDATES": "webhook"}), "TELEGRAM_WEBHOOK_SECRET")
	mustFail(t, required(map[string]string{"TELEGRAM_UPDATES": "push"}), "TELEGRAM_UPDATES")
	cfg := mustLoad(t, required(map[string]string{"TELEGRAM_UPDATES": "webhook", "TELEGRAM_WEBHOOK_SECRET": testWebhook}))
	if cfg.TelegramUpdates != UpdatesWebhook {
		t.Errorf("TelegramUpdates = %q", cfg.TelegramUpdates)
	}
}

func TestLoadRejectsUnsupportedLocale(t *testing.T) {
	mustFail(t, required(map[string]string{"DEFAULT_LOCALE": "de"}), "DEFAULT_LOCALE")
	cfg := mustLoad(t, required(map[string]string{"DEFAULT_LOCALE": "en"}))
	if cfg.DefaultLocale != "en" {
		t.Errorf("DefaultLocale = %q", cfg.DefaultLocale)
	}
}

func TestLoadRejectsMaxBelowIdle(t *testing.T) {
	mustFail(t, required(map[string]string{"SESSION_IDLE_TTL": "2h", "SESSION_MAX_TTL": "1h"}), "SESSION_MAX_TTL")
	mustFail(t, required(map[string]string{"SESSION_IDLE_TTL": "soon"}), "SESSION_IDLE_TTL")
}

func TestLoadRejectsBadLogLevel(t *testing.T) {
	mustFail(t, required(map[string]string{"LOG_LEVEL": "loud"}), "LOG_LEVEL")
	cfg := mustLoad(t, required(map[string]string{"LOG_LEVEL": "debug"}))
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v", cfg.LogLevel)
	}
}

func TestConfigLogValueRedactsSecrets(t *testing.T) {
	cfg := mustLoad(t, required(map[string]string{"TELEGRAM_UPDATES": "webhook", "TELEGRAM_WEBHOOK_SECRET": testWebhook}))
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("starting", "config", cfg)
	out := buf.String()
	for _, secret := range []string{testSecret, testToken, testWebhook} {
		if strings.Contains(out, secret) {
			t.Errorf("log contains secret %q: %s", secret, out)
		}
	}
	if !strings.Contains(out, "[redacted]") {
		t.Errorf("log has no [redacted] marker: %s", out)
	}
	if !strings.Contains(out, "PublicURL") {
		t.Errorf("log has no PublicURL: %s", out)
	}
}

func TestLoadTrustProxy(t *testing.T) {
	if mustLoad(t, required(nil)).TrustProxy {
		t.Error("TrustProxy defaults to true")
	}
	if !mustLoad(t, required(map[string]string{"TRUST_PROXY": "true"})).TrustProxy {
		t.Error("TRUST_PROXY=true ignored")
	}
	mustFail(t, required(map[string]string{"TRUST_PROXY": "maybe"}), "TRUST_PROXY")
}

func TestLoadWebhookSecretCharset(t *testing.T) {
	webhook := func(secret string) map[string]string {
		return required(map[string]string{"TELEGRAM_UPDATES": "webhook", "TELEGRAM_WEBHOOK_SECRET": secret})
	}
	mustFail(t, webhook("abc+/def=="), "TELEGRAM_WEBHOOK_SECRET") // base64 output: Telegram refuses it
	mustFail(t, webhook(strings.Repeat("a", 257)), "TELEGRAM_WEBHOOK_SECRET")
	mustLoad(t, webhook("Abc_123-xyz"))
}

func TestConfigMCPRedirects(t *testing.T) {
	cfg := mustLoad(t, required(nil))
	want := []string{"https://claude.ai/api/mcp/auth_callback", "https://claude.com/api/mcp/auth_callback",
		"https://chatgpt.com/connector_platform_oauth_redirect", "http://localhost:*", "http://127.0.0.1:*"}
	if strings.Join(cfg.MCPRedirectURIs, " ") != strings.Join(want, " ") {
		t.Errorf("default MCPRedirectURIs = %v", cfg.MCPRedirectURIs)
	}
	cfg = mustLoad(t, required(map[string]string{"MCP_REDIRECT_URIS": " https://cursor.example/cb , http://localhost:* ,"}))
	if strings.Join(cfg.MCPRedirectURIs, " ") != "https://cursor.example/cb http://localhost:*" {
		t.Errorf("override = %v", cfg.MCPRedirectURIs)
	}
	mustFail(t, required(map[string]string{"MCP_REDIRECT_URIS": "not a url"}), "MCP_REDIRECT_URIS")
}

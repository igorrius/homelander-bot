// Command homelander runs the Homelander service: HTTP API, web app and bot in one process.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/igorrius/homelander-bot/internal/config"
	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/httpapi"
	"github.com/igorrius/homelander-bot/internal/i18n"
	"github.com/igorrius/homelander-bot/internal/messenger/telegram"
	"github.com/igorrius/homelander-bot/internal/ratelimit"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite"
	"github.com/igorrius/homelander-bot/locales"
	"github.com/igorrius/homelander-bot/web"
)

const (
	pendingCap      = 200
	codeTTL         = 5 * time.Minute
	maxCodeAttempts = 5
	shutdownTimeout = 15 * time.Second
	cleanupEvery    = time.Hour
	leaveGrace      = 2 * time.Minute
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// After the first signal, restore default handling so a second one kills the process.
	go func() { <-ctx.Done(); stop() }()
	if err := run(ctx, os.Getenv, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		stop()
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, stderr io.Writer) error {
	return runWith(ctx, getenv, stderr, func(token string) (telegram.API, error) {
		return gotgbot.NewBot(token, nil)
	})
}

// runWith runs the service; newAPI builds the Telegram client (a fake in tests).
func runWith(ctx context.Context, getenv func(string) string, stderr io.Writer, newAPI func(token string) (telegram.API, error)) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	log.Info("starting", "config", cfg)

	rawDB, err := sqlite.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer func() { _ = rawDB.Close() }()
	if err := sqlite.Migrate(ctx, rawDB, sqlite.Migrations); err != nil {
		return err
	}
	db := sqlite.DB{DB: rawDB}

	tr, err := i18n.New(locales.FS, cfg.DefaultLocale, config.SupportedLocales)
	if err != nil {
		return err
	}

	repos := sqlite.NewRepos(db)
	now := domain.SystemClock
	limits := ratelimit.New(now)
	users := &domain.UserService{R: repos, Now: now, Limits: limits, Region: cfg.DefaultPhoneRegion,
		PendingCap: pendingCap, Locales: config.SupportedLocales}
	catalog := &domain.CatalogService{R: repos, Now: now, Locales: config.SupportedLocales, DefaultLocale: cfg.DefaultLocale}
	auth := &domain.AuthService{R: repos, Now: now, Limits: limits, Secret: cfg.AppSecret, Region: cfg.DefaultPhoneRegion,
		I18n: tr, BootstrapPhone: cfg.BootstrapAdminPhone, CodeTTL: codeTTL, MaxAttempts: maxCodeAttempts,
		IdleTTL: cfg.SessionIdleTTL, MaxTTL: cfg.SessionMaxTTL, Log: log}

	tgAPI, err := newAPI(cfg.TelegramBotToken)
	if err != nil {
		return fmt.Errorf("telegram client: %w", err)
	}
	messengers := domain.Messengers{}
	chats := &domain.ChatService{R: repos, Now: now, Messengers: messengers, Log: log, LeaveGrace: leaveGrace}
	bot, err := telegram.NewWithLocales(tgAPI, auth, chats, telegram.NopSink{}, tr, config.SupportedLocales, cfg.PublicURL.String(), log)
	if err != nil {
		return err
	}
	messengers[telegram.Type] = bot
	if err := bot.SetCommands(ctx); err != nil {
		log.Warn("telegram command menu not set", "err", err)
	}
	auth.Messenger = bot

	server := httpapi.New(cfg, httpapi.Deps{DB: rawDB, I18n: tr, Log: log, Static: web.Dist()})
	server.WithSession(auth)
	api := httpapi.API{Users: users, Catalog: catalog, Auth: auth, Messenger: bot, Cfg: cfg}
	httpapi.RegisterPublic(server, api)
	httpapi.RegisterAdmin(server, api)
	webhookPath := telegram.WebhookPath(cfg.AppSecret)
	if cfg.TelegramUpdates == config.UpdatesWebhook {
		server.Mount(webhookPath, bot.WebhookHandler(cfg.TelegramWebhookSecret))
	}

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.HTTPAddr, err)
	}
	srv := &http.Server{Handler: server.Handler(), ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second}
	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", ln.Addr().String())
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	switch cfg.TelegramUpdates {
	case config.UpdatesWebhook:
		if err := bot.StartWebhook(ctx, cfg.PublicURL.String()+webhookPath, cfg.TelegramWebhookSecret); err != nil {
			log.Error("telegram setWebhook failed; the bot will not receive updates", "err", err)
		}
	default:
		go func() {
			if err := bot.RunPolling(ctx); err != nil {
				log.Error("telegram polling stopped", "err", err)
			}
		}()
	}
	go cleanSessions(ctx, auth, log)

	select {
	case err := <-errc:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}
	log.Info("stopped")
	return nil
}

// cleanSessions deletes expired sessions every hour.
func cleanSessions(ctx context.Context, auth *domain.AuthService, log *slog.Logger) {
	t := time.NewTicker(cleanupEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := auth.CleanExpired(ctx); err != nil {
				log.Warn("session cleanup failed", "err", err)
			} else if n > 0 {
				log.Info("expired sessions removed", "count", n)
			}
		}
	}
}

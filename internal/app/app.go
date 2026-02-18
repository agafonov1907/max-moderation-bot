package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"max-moderation-bot/internal/config"
	"max-moderation-bot/internal/handler"
	"max-moderation-bot/internal/metrics"
	"max-moderation-bot/internal/repository"
	"max-moderation-bot/internal/service"
	"max-moderation-bot/internal/transport/polling"
	"max-moderation-bot/internal/transport/webhook"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

type App struct {
	cfg    *config.Config
	logger *slog.Logger
	bot    *maxbot.Api
	tracer trace.Tracer
}

func NewApp(cfg *config.Config, logger *slog.Logger) (*App, error) {

	bot, err := maxbot.New(cfg.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot client: %w", err)
	}

	return &App{
		cfg:    cfg,
		logger: logger,
		bot:    bot,
		tracer: otel.Tracer("max-moderation-bot"),
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	a.logger.Info("Starting Max Moderation Bot")

	botInfo, err := a.bot.Bots.GetBot(ctx)
	if err != nil {
		return fmt.Errorf("failed to get bot info: %w", err)
	}
	a.logger.Info("Bot connected", "username", botInfo.Username, "id", botInfo.UserId)

	db, err := repository.NewPostgresDB(a.cfg.GetDSN())
	if err != nil {
		return fmt.Errorf("failed to init db: %w", err)
	}

	settingsRepo := repository.NewSettingsRepository(db, a.cfg.EnableCache)
	chatAdminRepo := repository.NewChatAdminRepository(db)
	linkTokenRepo := repository.NewLinkTokenRepository(db)

	muteRepo := repository.NewMuteRepository(db)
	userStateRepo := repository.NewUserStateRepository(db)
	tempMessageRepo := repository.NewTemporaryMessageRepository(db)
	violationRepo := repository.NewViolationRepository(db)

	svc := service.NewModerationService(a.logger, settingsRepo, chatAdminRepo, linkTokenRepo, muteRepo, tempMessageRepo, violationRepo, a.bot)
	svc.StartMetricsUpdater(ctx)
	svc.StartCleanupTask(ctx, a.bot)
	h := handler.NewHandler(a.logger, svc, a.bot, userStateRepo, a.cfg)

	metricsSrv := metrics.NewServer(a.logger, a.cfg.MetricsAddr)
	go func() {
		if err := metricsSrv.Listen(); err != nil && err != http.ErrServerClosed {
			a.logger.Error("Metrics server failed", "error", err)
		}
	}()

	var updates <-chan schemes.UpdateInterface
	var cleanup func() error

	if a.cfg.WebhookHost != "" {

		a.logger.Info("Starting in Webhook mode", "host", a.cfg.WebhookHost)
		srv := webhook.NewServer(a.logger, a.bot, a.cfg.WebhookHost, a.cfg.Port)

		var err error
		updates, cleanup, err = srv.Start(ctx)
		if err != nil {
			return fmt.Errorf("failed to start webhook server: %w", err)
		}
		if cleanup != nil {
			defer func() {
				if err := cleanup(); err != nil {
					a.logger.Error("Cleanup failed", "error", err)
				}
			}()
		}

	} else {

		a.logger.Info("Starting in Long Polling mode")
		poller := polling.NewPoller(a.logger, a.bot)
		updates = poller.Start(ctx)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case upd, ok := <-updates:
				if !ok {
					return
				}
				h.HandleUpdate(ctx, upd)
			}
		}
	}()

	<-ctx.Done()
	a.logger.Info("Shutting down...")

	return nil
}

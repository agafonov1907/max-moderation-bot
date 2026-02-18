package webhook

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

type Server struct {
	logger *slog.Logger
	bot    *maxbot.Api
	host   string
	port   string
}

func NewServer(logger *slog.Logger, bot *maxbot.Api, host, port string) *Server {
	return &Server{
		logger: logger,
		bot:    bot,
		host:   host,
		port:   port,
	}
}

func (s *Server) Start(ctx context.Context) (<-chan schemes.UpdateInterface, func() error, error) {
	updates := make(chan schemes.UpdateInterface, 100)

	if subs, err := s.bot.Subscriptions.GetSubscriptions(ctx); err == nil {
		for _, sub := range subs.Subscriptions {
			if _, err := s.bot.Subscriptions.Unsubscribe(ctx, sub.Url); err != nil {
				s.logger.Warn("Failed to unsubscribe old webhook", "url", sub.Url, "error", err)
			}
		}
	}

	webhookURL := fmt.Sprintf("%s/webhook", s.host)

	if _, err := s.bot.Subscriptions.Subscribe(ctx, webhookURL, []string{}, "secret"); err != nil {
		return nil, nil, fmt.Errorf("failed to subscribe webhook: %w", err)
	}
	s.logger.Info("Subscribed to webhook", "url", webhookURL)

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", s.bot.GetHandler(updates))

	server := &http.Server{
		Addr:    ":" + s.port,
		Handler: mux,
	}

	go func() {
		s.logger.Info("Webhook server listening", "port", s.port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Webhook server failed", "error", err)
		}
	}()

	cleanup := func() error {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown error: %w", err)
		}
		return nil
	}

	return updates, cleanup, nil
}

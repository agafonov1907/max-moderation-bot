package polling

import (
	"context"
	"log/slog"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

type Poller struct {
	logger *slog.Logger
	bot    *maxbot.Api
}

func NewPoller(logger *slog.Logger, bot *maxbot.Api) *Poller {
	return &Poller{
		logger: logger,
		bot:    bot,
	}
}

func (p *Poller) Start(ctx context.Context) <-chan schemes.UpdateInterface {
	p.logger.Info("Starting Long Polling")
	return p.bot.GetUpdates(ctx)
}

package handler

import (
	"context"

	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

func (h *Handler) handleCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate) {
	h.callbackHandler.Handle(ctx, upd)
}

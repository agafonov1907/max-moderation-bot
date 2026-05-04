package handler

import (
	"context"
	"fmt"
	"os"

	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

func (h *Handler) handleCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate) {
	// 🔥 ЯДЕРНЫЙ ЛОГ + принудительная запись в stdout
	fmt.Printf("### CALLBACK_ENTRY: payload=%s user=%d chat=%d\n",
		upd.Callback.Payload, upd.Callback.GetUserID(), upd.Callback.GetChatID())
	os.Stdout.Sync() // ← КРИТИЧНО: сбрасывает буфер, лог появится сразу

	h.callbackHandler.Handle(ctx, upd)
}

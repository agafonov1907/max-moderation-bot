package callbacks

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

func (h *CallbackHandler) handleMonitoringStats(ctx context.Context, userID int64) {
	chatIDs, err := h.svc.GetManagedChats(ctx, userID)
	if err != nil {
		h.sendText(ctx, userID, "❌ Ошибка при получении списка чатов.")
		return
	}
	totalChats := len(chatIDs)
	if totalChats == 0 {
		h.sendText(ctx, userID, "📊 У вас пока нет подключённых чатов.\n\nДобавьте чат через кнопку «➕ Добавить чат».")
		return
	}
	chatMemberCache, _ := h.statsRepo.GetAllMemberCache(ctx, chatIDs)
	totalMembers := 0
	for _, cache := range chatMemberCache {
		if cache != nil {
			totalMembers += cache.MemberCount
		}
	}
	lastUpdated, _ := h.statsRepo.GetLastCacheUpdate(ctx, chatIDs)
	cacheStatus := "❌ Не обновлялся"
	if !lastUpdated.IsZero() {
		hoursAgo := time.Since(lastUpdated).Hours()
		if hoursAgo < 1 {
			cacheStatus = fmt.Sprintf("✅ %d мин. назад", int(hoursAgo*60))
		} else if hoursAgo < 24 {
			cacheStatus = fmt.Sprintf("⚠️ %.1f ч. назад", hoursAgo)
		} else {
			cacheStatus = fmt.Sprintf("⚠️ %.1f дн. назад", hoursAgo/24)
		}
	}
	report := fmt.Sprintf("📊 **Статистика модерации**\n\n"+
		"📋 **Всего чатов:** %d\n"+
		"👥 **Всего участников:** %d (из кэша)\n"+
		"🕐 **Кэш участников:** %s\n\n"+
		"_Подробная статистика по каждому чату доступна в разделе «Мои чаты»._",
		totalChats, totalMembers, cacheStatus)
	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(report)
	msg.SetFormat("markdown")
	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().
		AddCallback("🔄 Обновить кэш", schemes.DEFAULT, "monitoring_refresh_cache").
		AddCallback("📥 Экспорт", schemes.DEFAULT, "monitoring_export")
	kb.AddRow().
		AddCallback("🔙 В меню", schemes.NEGATIVE, "main_menu")
	msg.AddKeyboard(kb)
	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send monitoring stats", "error", err)
	}
}

func (h *CallbackHandler) handleMonitoringRefreshCache(ctx context.Context, userID int64) {
	h.sendText(ctx, userID, "🔄 Обновляю статистику участников...\n\n_Это может занять несколько минут._")
	go func() {
		bgCtx := context.Background()
		err := h.svc.UpdateMemberCache(bgCtx, userID)
		if err != nil {
			h.sendText(bgCtx, userID, "❌ Ошибка обновления кэша: "+err.Error())
		} else {
			h.sendText(bgCtx, userID, "✅ Статистика участников обновлена!")
		}
	}()
}

func (h *CallbackHandler) handleMonitoringExport(ctx context.Context, userID int64) {
	reportDir := "/root/logs/reports"
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		h.sendText(ctx, userID, "❌ Ошибка создания папки для отчётов.")
		return
	}
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("monitoring_report_%s.csv", timestamp)
	filepath := filepath.Join(reportDir, filename)
	chatIDs, err := h.svc.GetManagedChats(ctx, userID)
	if err != nil {
		h.sendText(ctx, userID, "❌ Ошибка при получении списка чатов.")
		return
	}
	file, err := os.Create(filepath)
	if err != nil {
		h.sendText(ctx, userID, "❌ Ошибка создания файла: "+err.Error())
		return
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	writer.Write([]string{"Chat ID", "Участников (кэш)"})
	chatMemberCache, _ := h.statsRepo.GetAllMemberCache(ctx, chatIDs)
	for _, chatID := range chatIDs {
		members := 0
		if cache, ok := chatMemberCache[chatID]; ok && cache != nil {
			members = cache.MemberCount
		}
		writer.Write([]string{fmt.Sprintf("%d", chatID), fmt.Sprintf("%d", members)})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		h.sendText(ctx, userID, "❌ Ошибка записи файла: "+err.Error())
		return
	}
	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(fmt.Sprintf("📥 **Отчёт готов!**\n\nФайл сохранён на сервере:\n`%s`\n\n_Скачайте его через файловый менеджер или по SFTP._", filepath))
	msg.SetFormat("markdown")
	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send export info", "error", err)
	}
}

func (h *CallbackHandler) handleMonitoringActivity(ctx context.Context, userID int64) {
	h.sendText(ctx, userID, "📈 **Активность**\n\n_Эта функция будет доступна в следующем обновлении._")
}

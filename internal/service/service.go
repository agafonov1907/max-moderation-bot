package service

import (
	"context"
	"fmt"
	"log/slog"
	"max-moderation-bot/internal/metrics"
	"max-moderation-bot/internal/pipeline"
	"max-moderation-bot/internal/pipeline/filters"
	"max-moderation-bot/internal/repository"
	"max-moderation-bot/internal/utils"
	"strings"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Service определяет интерфейс сервиса модерации
type Service interface {
	ModerateMessage(ctx context.Context, payload pipeline.Payload) (*pipeline.Result, error)
	GenerateLinkToken(ctx context.Context, userID int64) (string, error)
	GetManagedChats(ctx context.Context, userID int64) ([]int64, error)
	GetManagedChatsWithNames(ctx context.Context, userID int64) ([]repository.ChatInfo, error) // ✅ Новый метод
	GetManagedChatsPaginated(ctx context.Context, userID int64, page int) ([]int64, int64, error)
	GetChatSettings(ctx context.Context, chatID int64) (*repository.ChatSettings, error)
	ToggleSetting(ctx context.Context, chatID int64, setting string) (bool, error)
	AddBlockedWords(ctx context.Context, chatID int64, words []string) error
	SetBlockedWords(ctx context.Context, chatID int64, words []string) error
	AddBlockedDomains(ctx context.Context, chatID int64, domains []string) error
	SetBlockedDomains(ctx context.Context, chatID int64, domains []string) error
	InitializeChat(ctx context.Context, chatID int64) error
	LinkGroup(ctx context.Context, token string, chatID, userID int64) error
	MuteUser(ctx context.Context, chatID, adminID, userID int64, userName string, duration time.Duration) error
	SystemMuteUser(ctx context.Context, chatID, userID int64, userName string, duration time.Duration) error
	TrackViolation(ctx context.Context, chatID, userID int64, violationType string) (bool, time.Duration, error)
	GetActiveMutesPaginated(ctx context.Context, chatID int64, page int) ([]repository.Mute, int64, error)
	GetMute(ctx context.Context, chatID, userID int64) (*repository.Mute, error)
	UnmuteUser(ctx context.Context, chatID, adminID, userID int64) error
	GetChatStats(ctx context.Context, chatID int64) (*repository.ChatStats, error)
	StartMetricsUpdater(ctx context.Context)
	StartCleanupTask(ctx context.Context, bot *maxbot.Api)
	ScheduleDeletion(ctx context.Context, chatID int64, messageID string, duration time.Duration) error
	IsChatAdmin(ctx context.Context, chatID, userID int64) (bool, error)
	IsChatOwner(ctx context.Context, chatID, userID int64) (bool, error)
}

// ModerationService реализует сервис модерации
type ModerationService struct {
	logger          *slog.Logger
	settingsRepo    repository.SettingsRepository
	chatAdminRepo   repository.ChatAdminRepository
	linkTokenRepo   repository.LinkTokenRepository
	muteRepo        repository.MuteRepository
	tempMessageRepo repository.TemporaryMessageRepository
	violationRepo   repository.ViolationRepository
	chatRepo        repository.ChatRepository // ✅ Новое поле для работы с названиями чатов
	pipeline        *pipeline.Manager
	tracer          trace.Tracer
	bot             *maxbot.Api
}

// NewModerationService создаёт новый экземпляр сервиса
func NewModerationService(
	logger *slog.Logger,
	settingsRepo repository.SettingsRepository,
	chatAdminRepo repository.ChatAdminRepository,
	linkTokenRepo repository.LinkTokenRepository,
	muteRepo repository.MuteRepository,
	tempMessageRepo repository.TemporaryMessageRepository,
	violationRepo repository.ViolationRepository,
	chatRepo repository.ChatRepository, // ✅ Новый параметр
	bot *maxbot.Api,
) Service {

	linkFilter := filters.NewLinkFilter(settingsRepo, violationRepo)
	wordFilter := filters.NewWordFilter(settingsRepo, violationRepo)
	muteFilter := filters.NewMuteFilter(muteRepo, settingsRepo)
	attachmentFilter := filters.NewAttachmentFilter(settingsRepo, violationRepo)
	rateLimitFilter := filters.NewRateLimitFilter(5, 1*time.Second)

	pm := pipeline.NewManager(rateLimitFilter, muteFilter, linkFilter, wordFilter, attachmentFilter)

	return &ModerationService{
		logger:          logger,
		settingsRepo:    settingsRepo,
		chatAdminRepo:   chatAdminRepo,
		linkTokenRepo:   linkTokenRepo,
		muteRepo:        muteRepo,
		tempMessageRepo: tempMessageRepo,
		violationRepo:   violationRepo,
		chatRepo:        chatRepo, // ✅ Инициализация нового поля
		pipeline:        pm,
		tracer:          otel.Tracer("service"),
		bot:             bot,
	}
}

// StartMetricsUpdater запускает фоновую задачу обновления метрик
func (s *ModerationService) StartMetricsUpdater(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)

	update := func() {
		count, err := s.muteRepo.CountActiveMutes()
		if err != nil {
			s.logger.Error("Failed to count active mutes", "error", err)
			return
		}
		metrics.SetActiveMutes(float64(count))
	}

	go update()

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				update()
			}
		}
	}()
}

// ModerateMessage обрабатывает сообщение через пайплайн фильтров
func (s *ModerationService) ModerateMessage(ctx context.Context, payload pipeline.Payload) (*pipeline.Result, error) {
	ctx, span := s.tracer.Start(ctx, "ModerateMessage")
	defer span.End()

	s.logger.Debug("Moderating message", "chat_id", payload.ChatID, "user_id", payload.SenderID)
	return s.pipeline.Process(ctx, payload)
}

// GenerateLinkToken генерирует токен для привязки чата
func (s *ModerationService) GenerateLinkToken(ctx context.Context, userID int64) (string, error) {
	_, span := s.tracer.Start(ctx, "GenerateLinkToken")
	defer span.End()
	return s.linkTokenRepo.Create(userID, 24*time.Hour)
}

// GetManagedChats возвращает список ID чатов, которыми управляет пользователь
func (s *ModerationService) GetManagedChats(ctx context.Context, userID int64) ([]int64, error) {
	_, span := s.tracer.Start(ctx, "GetManagedChats")
	defer span.End()
	return s.chatAdminRepo.GetManagedChats(userID)
}

// ✅ GetManagedChatsWithNames возвращает список чатов с названиями из MAX API
func (s *ModerationService) GetManagedChatsWithNames(ctx context.Context, userID int64) ([]repository.ChatInfo, error) {
	ctx, span := s.tracer.Start(ctx, "GetManagedChatsWithNames")
	defer span.End()

	s.logger.Debug("GetManagedChatsWithNames: starting", "user_id", userID)

	// 1. Получаем ID чатов из репозитория
	chatIDs, err := s.chatAdminRepo.GetManagedChats(userID)
	if err != nil {
		s.logger.Error("GetManagedChatsWithNames: failed to get chat IDs", "user_id", userID, "error", err)
		return nil, fmt.Errorf("failed to get managed chat IDs: %w", err)
	}

	s.logger.Debug("GetManagedChatsWithNames: got chat IDs", "user_id", userID, "count", len(chatIDs), "ids", chatIDs)

	// 2. Обогащаем ID названиями через MAX API
	var chats []repository.ChatInfo
	for _, chatID := range chatIDs {
		// Fallback: если не сможем получить название — используем "Чат {ID}"
		chatName := fmt.Sprintf("Чат %d", chatID)
		nameSource := "fallback"

		// Пытаемся получить информацию о чате через MAX API
		if s.bot != nil {
			s.logger.Debug("GetManagedChatsWithNames: fetching chat info from API", "chat_id", chatID)
			
			chatInfo, err := s.bot.Chats.GetChat(ctx, chatID)
			if err != nil {
				s.logger.Warn("GetManagedChatsWithNames: failed to get chat info from API", 
					"chat_id", chatID, "error", err)
			} else if chatInfo != nil {
				// ✅ ОТЛАДКА: вывести полную структуру chatInfo в лог
				s.logger.Debug("GetManagedChatsWithNames: chat info from API", 
					"chat_id", chatID, 
					"info", fmt.Sprintf("%+v", chatInfo))
				
				// ✅ ИСПРАВЛЕНО: используем ТОЛЬКО поле Title (единственное доступное в schemes.Chat)
				if chatInfo.Title != "" {
					chatName = chatInfo.Title
					nameSource = "Title"
					s.logger.Debug("GetManagedChatsWithNames: got title from API", 
						"chat_id", chatID, "title", chatInfo.Title)
				} else {
					s.logger.Debug("GetManagedChatsWithNames: Title field is empty, using fallback", 
						"chat_id", chatID)
				}
			} else {
				s.logger.Debug("GetManagedChatsWithNames: chatInfo is nil", "chat_id", chatID)
			}
		} else {
			s.logger.Debug("GetManagedChatsWithNames: bot client is nil, using fallback", "chat_id", chatID)
		}

		s.logger.Debug("GetManagedChatsWithNames: enriched chat", 
			"chat_id", chatID, 
			"name", chatName, 
			"source", nameSource)

		chats = append(chats, repository.ChatInfo{
			ID:   chatID,
			Name: chatName,
		})
	}

	s.logger.Debug("GetManagedChatsWithNames: completed", "user_id", userID, "total_chats", len(chats))
	return chats, nil
}

// GetManagedChatsPaginated возвращает пагинированный список ID чатов
func (s *ModerationService) GetManagedChatsPaginated(ctx context.Context, userID int64, page int) ([]int64, int64, error) {
	_, span := s.tracer.Start(ctx, "GetManagedChatsPaginated")
	defer span.End()
	pageSize := 10
	offset := (page - 1) * pageSize
	return s.chatAdminRepo.GetManagedChatsPaginated(userID, offset, pageSize)
}

// GetChatSettings получает настройки чата
func (s *ModerationService) GetChatSettings(ctx context.Context, chatID int64) (*repository.ChatSettings, error) {
	_, span := s.tracer.Start(ctx, "GetChatSettings")
	defer span.End()
	return s.settingsRepo.GetSettings(chatID)
}

// ToggleSetting переключает настройку чата
func (s *ModerationService) ToggleSetting(ctx context.Context, chatID int64, setting string) (bool, error) {
	_, span := s.tracer.Start(ctx, "ToggleSetting")
	defer span.End()

	settings, err := s.settingsRepo.GetSettings(chatID)
	if err != nil {
		return false, err
	}
	var newValue bool
	switch setting {
	case "words", "word_filter":
		settings.EnableWordFilter = !settings.EnableWordFilter
		newValue = settings.EnableWordFilter
	case "links", "link_filter":
		settings.EnableLinkFilter = !settings.EnableLinkFilter
		newValue = settings.EnableLinkFilter
	case "mute":
		settings.EnableMute = !settings.EnableMute
		newValue = settings.EnableMute
	case "autodelete", "auto_delete":
		settings.EnableAutoDelete = !settings.EnableAutoDelete
		newValue = settings.EnableAutoDelete
	case "image":
		settings.RestrictImage = !settings.RestrictImage
		newValue = settings.RestrictImage
	case "video":
		settings.RestrictVideo = !settings.RestrictVideo
		newValue = settings.RestrictVideo
	case "audio":
		settings.RestrictAudio = !settings.RestrictAudio
		newValue = settings.RestrictAudio
	case "file", "document":
		settings.RestrictFile = !settings.RestrictFile
		newValue = settings.RestrictFile
	default:
		return false, fmt.Errorf("unknown setting: %s", setting)
	}
	if err := s.settingsRepo.UpdateSettings(settings); err != nil {
		return false, err
	}
	return newValue, nil
}

// AddBlockedWords добавляет слова в чёрный список чата
func (s *ModerationService) AddBlockedWords(ctx context.Context, chatID int64, words []string) error {
	_, span := s.tracer.Start(ctx, "AddBlockedWords")
	defer span.End()

	settings, err := s.settingsRepo.GetSettings(chatID)
	if err != nil {
		return err
	}
	existing := make(map[string]struct{})
	for _, w := range settings.BlockedWords {
		existing[w] = struct{}{}
	}
	for _, w := range words {
		norm := strings.ToLower(strings.TrimSpace(w))
		if norm == "" {
			continue
		}
		if _, exists := existing[norm]; !exists {
			settings.BlockedWords = append(settings.BlockedWords, norm)
			existing[norm] = struct{}{}
		}
	}
	return s.settingsRepo.UpdateSettings(settings)
}

// SetBlockedWords полностью заменяет список заблокированных слов
func (s *ModerationService) SetBlockedWords(ctx context.Context, chatID int64, words []string) error {
	_, span := s.tracer.Start(ctx, "SetBlockedWords")
	defer span.End()

	settings, err := s.settingsRepo.GetSettings(chatID)
	if err != nil {
		return err
	}

	unique := make(map[string]struct{})
	var normalized []string
	for _, w := range words {
		norm := strings.ToLower(strings.TrimSpace(w))
		if norm == "" {
			continue
		}
		if _, exists := unique[norm]; !exists {
			unique[norm] = struct{}{}
			normalized = append(normalized, norm)
		}
	}
	settings.BlockedWords = normalized
	return s.settingsRepo.UpdateSettings(settings)
}

// AddBlockedDomains добавляет домены в чёрный список чата
func (s *ModerationService) AddBlockedDomains(ctx context.Context, chatID int64, domains []string) error {
	_, span := s.tracer.Start(ctx, "AddBlockedDomains")
	defer span.End()

	settings, err := s.settingsRepo.GetSettings(chatID)
	if err != nil {
		return err
	}
	existing := make(map[string]struct{})
	for _, d := range settings.BlockedDomains {
		existing[d] = struct{}{}
	}
	for _, d := range domains {
		norm := utils.NormalizeDomain(d)
		if norm == "" {
			continue
		}
		if _, exists := existing[norm]; !exists {
			settings.BlockedDomains = append(settings.BlockedDomains, norm)
			existing[norm] = struct{}{}
		}
	}
	return s.settingsRepo.UpdateSettings(settings)
}

// SetBlockedDomains полностью заменяет список заблокированных доменов
func (s *ModerationService) SetBlockedDomains(ctx context.Context, chatID int64, domains []string) error {
	_, span := s.tracer.Start(ctx, "SetBlockedDomains")
	defer span.End()

	settings, err := s.settingsRepo.GetSettings(chatID)
	if err != nil {
		return err
	}

	unique := make(map[string]struct{})
	var normalized []string
	for _, d := range domains {
		norm := utils.NormalizeDomain(d)
		if norm == "" {
			continue
		}
		if _, exists := unique[norm]; !exists {
			unique[norm] = struct{}{}
			normalized = append(normalized, norm)
		}
	}
	settings.BlockedDomains = normalized
	return s.settingsRepo.UpdateSettings(settings)
}

// InitializeChat инициализирует настройки чата по умолчанию
func (s *ModerationService) InitializeChat(ctx context.Context, chatID int64) error {
	_, span := s.tracer.Start(ctx, "InitializeChat")
	defer span.End()
	return s.settingsRepo.InitSettings(chatID)
}

// LinkGroup привязывает чат к пользователю по токену
func (s *ModerationService) LinkGroup(ctx context.Context, token string, chatID, userID int64) error {
	_, span := s.tracer.Start(ctx, "LinkGroup")
	defer span.End()

	linkToken, err := s.linkTokenRepo.Get(token)
	if err != nil {
		return fmt.Errorf("invalid or expired token: %w", err)
	}
	if linkToken.UserID != userID {
		return fmt.Errorf("token does not belong to user")
	}
	if err := s.chatAdminRepo.AddAdmin(chatID, userID); err != nil {
		return fmt.Errorf("failed to add admin: %w", err)
	}
	return s.linkTokenRepo.Delete(token)
}

// MuteUser блокирует пользователя в чате (по запросу админа)
func (s *ModerationService) MuteUser(ctx context.Context, chatID, adminID, userID int64, userName string, duration time.Duration) error {
	_, span := s.tracer.Start(ctx, "MuteUser")
	defer span.End()
	isAdmin, err := s.chatAdminRepo.IsAdmin(chatID, adminID)
	if err != nil {
		return fmt.Errorf("failed to check admin status: %w", err)
	}
	if !isAdmin {
		return fmt.Errorf("user %d is not a bot admin in chat %d", adminID, chatID)
	}
	if err := s.muteRepo.MuteUser(chatID, userID, userName, duration); err != nil {
		return err
	}
	return nil
}

// SystemMuteUser блокирует пользователя системно (автоматически за нарушения)
func (s *ModerationService) SystemMuteUser(ctx context.Context, chatID, userID int64, userName string, duration time.Duration) error {
	_, span := s.tracer.Start(ctx, "SystemMuteUser")
	defer span.End()
	if err := s.muteRepo.MuteUser(chatID, userID, userName, duration); err != nil {
		return err
	}
	go func() {
		_ = s.violationRepo.IncrementChatStat(context.Background(), chatID, "mute_count")
	}()
	return nil
}

// TrackViolation отслеживает нарушение и возвращает, нужно ли мутить пользователя
func (s *ModerationService) TrackViolation(ctx context.Context, chatID, userID int64, violationType string) (bool, time.Duration, error) {
	_, span := s.tracer.Start(ctx, "TrackViolation")
	defer span.End()

	if err := s.violationRepo.AddViolation(ctx, chatID, userID, violationType); err != nil {
		return false, 0, err
	}

	since := time.Now().Add(-24 * time.Hour)
	count, err := s.violationRepo.CountViolationsSince(ctx, chatID, userID, since)
	if err != nil {
		return false, 0, err
	}

	if count >= 5 {
		muteDuration := 24 * time.Hour
		return true, muteDuration, nil
	}

	return false, 0, nil
}

// GetActiveMutesPaginated возвращает пагинированный список активных мутов
func (s *ModerationService) GetActiveMutesPaginated(ctx context.Context, chatID int64, page int) ([]repository.Mute, int64, error) {
	_, span := s.tracer.Start(ctx, "GetActiveMutesPaginated")
	defer span.End()
	pageSize := 10
	offset := (page - 1) * pageSize
	return s.muteRepo.GetActiveMutesPaginated(chatID, offset, pageSize)
}

// GetMute получает информацию о муте пользователя в чате
func (s *ModerationService) GetMute(ctx context.Context, chatID, userID int64) (*repository.Mute, error) {
	_, span := s.tracer.Start(ctx, "GetMute")
	defer span.End()
	return s.muteRepo.GetMute(chatID, userID)
}

// UnmuteUser разблокирует пользователя в чате
func (s *ModerationService) UnmuteUser(ctx context.Context, chatID, adminID, userID int64) error {
	_, span := s.tracer.Start(ctx, "UnmuteUser")
	defer span.End()
	isAdmin, err := s.chatAdminRepo.IsAdmin(chatID, adminID)
	if err != nil {
		return fmt.Errorf("failed to check admin status: %w", err)
	}
	if !isAdmin {
		return fmt.Errorf("user %d is not a bot admin in chat %d", adminID, chatID)
	}
	return s.muteRepo.UnmuteUser(chatID, userID)
}

// GetChatStats получает статистику нарушений по чату
func (s *ModerationService) GetChatStats(ctx context.Context, chatID int64) (*repository.ChatStats, error) {
	_, span := s.tracer.Start(ctx, "GetChatStats")
	defer span.End()
	return s.violationRepo.GetChatTotalStats(ctx, chatID)
}

// IsChatAdmin проверяет, является ли пользователь админом чата через MAX API
func (s *ModerationService) IsChatAdmin(ctx context.Context, chatID, userID int64) (bool, error) {
	_, span := s.tracer.Start(ctx, "IsChatAdmin")
	defer span.End()

	if s.bot == nil {
		return false, fmt.Errorf("bot client not initialized in service")
	}

	adminList, err := s.bot.Chats.GetChatAdmins(ctx, chatID)
	if err != nil {
		return false, fmt.Errorf("failed to get chat admins: %w", err)
	}

	for _, member := range adminList.Members {
		if member.UserId == userID {
			return true, nil
		}
	}

	return false, nil
}

// IsChatOwner проверяет, является ли пользователь владельцем чата через MAX API
func (s *ModerationService) IsChatOwner(ctx context.Context, chatID, userID int64) (bool, error) {
	_, span := s.tracer.Start(ctx, "IsChatOwner")
	defer span.End()

	if s.bot == nil {
		return false, fmt.Errorf("bot client not initialized in service")
	}

	adminList, err := s.bot.Chats.GetChatAdmins(ctx, chatID)
	if err != nil {
		return false, fmt.Errorf("failed to get chat admins: %w", err)
	}

	for _, member := range adminList.Members {
		if member.UserId == userID && member.IsOwner {
			return true, nil
		}
	}

	return false, nil
}
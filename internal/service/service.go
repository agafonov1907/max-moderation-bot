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

type Service interface {
	ModerateMessage(ctx context.Context, payload pipeline.Payload) (*pipeline.Result, error)
	GenerateLinkToken(ctx context.Context, userID int64) (string, error)
	GetManagedChats(ctx context.Context, userID int64) ([]int64, error)
	GetManagedChatsWithNames(ctx context.Context, userID int64) ([]repository.ChatInfo, error)
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
	UpdateMemberCache(ctx context.Context, userID int64) error
}

type ModerationService struct {
	logger          *slog.Logger
	settingsRepo    repository.SettingsRepository
	chatAdminRepo   repository.ChatAdminRepository
	linkTokenRepo   repository.LinkTokenRepository
	muteRepo        repository.MuteRepository
	tempMessageRepo repository.TemporaryMessageRepository
	violationRepo   repository.ViolationRepository
	chatRepo        repository.ChatRepository
	pipeline        *pipeline.Manager
	tracer          trace.Tracer
	bot             *maxbot.Api
	statsRepo       *repository.PostgresRepository
}

func NewModerationService(
	logger *slog.Logger,
	settingsRepo repository.SettingsRepository,
	chatAdminRepo repository.ChatAdminRepository,
	linkTokenRepo repository.LinkTokenRepository,
	muteRepo repository.MuteRepository,
	tempMessageRepo repository.TemporaryMessageRepository,
	violationRepo repository.ViolationRepository,
	chatRepo repository.ChatRepository,
	bot *maxbot.Api,
	statsRepo *repository.PostgresRepository,
) *ModerationService {
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
		chatRepo:        chatRepo,
		pipeline:        pm,
		tracer:          otel.Tracer("service"),
		bot:             bot,
		statsRepo:       statsRepo,
	}
}

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

func (s *ModerationService) ModerateMessage(ctx context.Context, payload pipeline.Payload) (*pipeline.Result, error) {
	ctx, span := s.tracer.Start(ctx, "ModerateMessage")
	defer span.End()
	s.logger.Debug("Moderating message", "chat_id", payload.ChatID, "user_id", payload.SenderID)
	return s.pipeline.Process(ctx, payload)
}

func (s *ModerationService) GenerateLinkToken(ctx context.Context, userID int64) (string, error) {
	_, span := s.tracer.Start(ctx, "GenerateLinkToken")
	defer span.End()
	return s.linkTokenRepo.Create(userID, 24*time.Hour)
}

func (s *ModerationService) GetManagedChats(ctx context.Context, userID int64) ([]int64, error) {
	_, span := s.tracer.Start(ctx, "GetManagedChats")
	defer span.End()
	return s.chatAdminRepo.GetManagedChats(userID)
}

func (s *ModerationService) GetManagedChatsWithNames(ctx context.Context, userID int64) ([]repository.ChatInfo, error) {
	ctx, span := s.tracer.Start(ctx, "GetManagedChatsWithNames")
	defer span.End()
	chatIDs, err := s.chatAdminRepo.GetManagedChats(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get managed chat IDs: %w", err)
	}
	var chats []repository.ChatInfo
	for _, chatID := range chatIDs {
		chatName := fmt.Sprintf("Чат %d", chatID)
		if s.bot != nil {
			chatInfo, err := s.bot.Chats.GetChat(ctx, chatID)
			if err == nil && chatInfo != nil && chatInfo.Title != "" {
				chatName = chatInfo.Title
			}
		}
		chats = append(chats, repository.ChatInfo{ID: chatID, Name: chatName})
	}
	return chats, nil
}

func (s *ModerationService) GetManagedChatsPaginated(ctx context.Context, userID int64, page int) ([]int64, int64, error) {
	pageSize := 10
	offset := (page - 1) * pageSize
	return s.chatAdminRepo.GetManagedChatsPaginated(userID, offset, pageSize)
}

func (s *ModerationService) GetChatSettings(ctx context.Context, chatID int64) (*repository.ChatSettings, error) {
	return s.settingsRepo.GetSettings(chatID)
}

func (s *ModerationService) ToggleSetting(ctx context.Context, chatID int64, setting string) (bool, error) {
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

func (s *ModerationService) AddBlockedWords(ctx context.Context, chatID int64, words []string) error {
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

func (s *ModerationService) SetBlockedWords(ctx context.Context, chatID int64, words []string) error {
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

func (s *ModerationService) AddBlockedDomains(ctx context.Context, chatID int64, domains []string) error {
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

func (s *ModerationService) SetBlockedDomains(ctx context.Context, chatID int64, domains []string) error {
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

func (s *ModerationService) InitializeChat(ctx context.Context, chatID int64) error {
	return s.settingsRepo.InitSettings(chatID)
}

func (s *ModerationService) LinkGroup(ctx context.Context, token string, chatID, userID int64) error {
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

func (s *ModerationService) MuteUser(ctx context.Context, chatID, adminID, userID int64, userName string, duration time.Duration) error {
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

func (s *ModerationService) SystemMuteUser(ctx context.Context, chatID, userID int64, userName string, duration time.Duration) error {
	if err := s.muteRepo.MuteUser(chatID, userID, userName, duration); err != nil {
		return err
	}
	go func() {
		_ = s.violationRepo.IncrementChatStat(context.Background(), chatID, "mute_count")
	}()
	return nil
}

func (s *ModerationService) TrackViolation(ctx context.Context, chatID, userID int64, violationType string) (bool, time.Duration, error) {
	if err := s.violationRepo.AddViolation(ctx, chatID, userID, violationType); err != nil {
		return false, 0, err
	}
	since := time.Now().Add(-24 * time.Hour)
	count, err := s.violationRepo.CountViolationsSince(ctx, chatID, userID, since)
	if err != nil {
		return false, 0, err
	}
	if count >= 5 {
		return true, 24 * time.Hour, nil
	}
	return false, 0, nil
}

func (s *ModerationService) GetActiveMutesPaginated(ctx context.Context, chatID int64, page int) ([]repository.Mute, int64, error) {
	pageSize := 10
	offset := (page - 1) * pageSize
	return s.muteRepo.GetActiveMutesPaginated(chatID, offset, pageSize)
}

func (s *ModerationService) GetMute(ctx context.Context, chatID, userID int64) (*repository.Mute, error) {
	return s.muteRepo.GetMute(chatID, userID)
}

func (s *ModerationService) UnmuteUser(ctx context.Context, chatID, adminID, userID int64) error {
	isAdmin, err := s.chatAdminRepo.IsAdmin(chatID, adminID)
	if err != nil {
		return fmt.Errorf("failed to check admin status: %w", err)
	}
	if !isAdmin {
		return fmt.Errorf("user %d is not a bot admin in chat %d", adminID, chatID)
	}
	return s.muteRepo.UnmuteUser(chatID, userID)
}

func (s *ModerationService) GetChatStats(ctx context.Context, chatID int64) (*repository.ChatStats, error) {
	return s.violationRepo.GetChatTotalStats(ctx, chatID)
}

// ✅ ИСПРАВЛЕНО: используем ParticipantsCount из schemes.Chat
func (s *ModerationService) UpdateMemberCache(ctx context.Context, userID int64) error {
	if s.statsRepo == nil {
		return fmt.Errorf("statsRepo not initialized")
	}
	chatIDs, err := s.GetManagedChats(ctx, userID)
	if err != nil {
		return err
	}
	if len(chatIDs) == 0 {
		return nil
	}
	s.logger.Info("Updating member cache", "user_id", userID, "chats_count", len(chatIDs))
	for i, chatID := range chatIDs {
		memberCount := 0
		if s.bot != nil {
			chatInfo, err := s.bot.Chats.GetChat(ctx, chatID)
			if err == nil && chatInfo != nil {
				// ✅ ИСПРАВЛЕНО: ParticipantsCount вместо MemberCount
				memberCount = chatInfo.ParticipantsCount
				s.logger.Debug("Got member count from API", "chat_id", chatID, "members", memberCount)
			}
		}
		err = s.statsRepo.UpdateMemberCache(ctx, chatID, memberCount)
		if err != nil {
			s.logger.Warn("Failed to update member cache", "chat_id", chatID, "error", err)
			continue
		}
		s.logger.Debug("Updated member cache", "chat_id", chatID, "members", memberCount)
		if i < len(chatIDs)-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}
	s.logger.Info("Member cache updated", "user_id", userID)
	return nil
}

func (s *ModerationService) IsChatAdmin(ctx context.Context, chatID, userID int64) (bool, error) {
	if s.bot == nil {
		return false, fmt.Errorf("bot client not initialized")
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

func (s *ModerationService) IsChatOwner(ctx context.Context, chatID, userID int64) (bool, error) {
	if s.bot == nil {
		return false, fmt.Errorf("bot client not initialized")
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

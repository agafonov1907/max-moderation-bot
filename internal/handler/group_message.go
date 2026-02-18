package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"max-moderation-bot/internal/messages"
	"max-moderation-bot/internal/pipeline"
	"strings"
	"time"

	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

func (h *Handler) handleGroupMessage(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	if strings.HasPrefix(upd.Message.Body.Text, "/link") {
		h.handleLinkCommand(ctx, upd)
		return
	}
	if strings.HasPrefix(upd.Message.Body.Text, "/mute") {
		h.handleMuteCommand(ctx, upd)
		return
	}
	var attachmentTypes []string
	if len(upd.Message.Body.RawAttachments) > 0 {
		for _, raw := range upd.Message.Body.RawAttachments {
			var attMap map[string]interface{}
			if err := json.Unmarshal(raw, &attMap); err == nil {
				if typeVal, ok := attMap["type"].(string); ok {
					attachmentTypes = append(attachmentTypes, typeVal)
				}
			} else {
				h.logger.Error("Failed to unmarshal attachment", "error", err)
			}
		}
	}
	h.logger.Info("Received group message",
		"text", upd.Message.Body.Text,
		"sender", upd.Message.Sender.UserId,
		"attachment_types", attachmentTypes,
		"chat_id", upd.Message.Recipient.ChatId,
	)
	payload := pipeline.Payload{
		ChatID:          upd.Message.Recipient.ChatId,
		SenderID:        upd.Message.Sender.UserId,
		Text:            upd.Message.Body.Text,
		AttachmentTypes: attachmentTypes,
	}
	res, err := h.svc.ModerateMessage(ctx, payload)
	if err != nil {
		h.logger.Error("Failed to moderate message", "error", err)
	}
	if res != nil && !res.IsAllowed {
		h.logger.Info("Message blocked", "reason", res.Reason, "filter", res.FilterName)

		go func() {
			if res.ShouldMute {
				h.logger.Info("Muting user for rate limit", "user_id", upd.Message.Sender.UserId, "duration", res.MuteDuration)
				if err := h.svc.SystemMuteUser(context.Background(), upd.Message.Recipient.ChatId, upd.Message.Sender.UserId, upd.Message.Sender.Name, res.MuteDuration); err != nil {
					h.logger.Error("Failed to system mute user", "error", err)
				}
				h.sendWarningWithMention(context.Background(), upd.Message.Recipient.ChatId, upd.Message.Sender, res.Reason)
				return
			}
			if res.FilterName != "mute_filter" {
				shouldMute, duration, err := h.svc.TrackViolation(context.Background(), upd.Message.Recipient.ChatId, upd.Message.Sender.UserId, res.FilterName)
				if err != nil {
					h.logger.Error("Failed to track violation", "error", err)
				}
				if shouldMute {
					h.logger.Info("Muting user for persistent violations", "user_id", upd.Message.Sender.UserId)
					if err := h.svc.SystemMuteUser(context.Background(), upd.Message.Recipient.ChatId, upd.Message.Sender.UserId, upd.Message.Sender.Name, duration); err != nil {
						h.logger.Error("Failed to system mute user", "error", err)
					}
					h.sendWarningWithMention(context.Background(), upd.Message.Recipient.ChatId, upd.Message.Sender, messages.MsgReasonPersistentViolation)
					return
				}

				h.sendWarningWithMention(context.Background(), upd.Message.Recipient.ChatId, upd.Message.Sender, res.Reason)
			}
		}()
		go func() {
			bgCtx := context.Background()

			if res.ShouldDelete {
				h.logger.Info("Deleting message as requested by filter", "mid", upd.Message.Body.Mid, "filter", res.FilterName)
				_ = h.deleteMessage(bgCtx, upd.Message.Body.Mid, res.FilterName)

				return
			}

			settings, err := h.svc.GetChatSettings(bgCtx, upd.Message.Recipient.ChatId)
			if err != nil {
				h.logger.Error("Failed to get settings for auto-delete check", "error", err)
			} else {
				h.logger.Info("Checking auto-delete setting", "enabled", settings.EnableAutoDelete, "chat_id", settings.ChatID)
				if settings.EnableAutoDelete {
					h.logger.Info("Attempting to delete message", "message_id", upd.Message.Body.Mid)
					_ = h.deleteMessage(bgCtx, upd.Message.Body.Mid, res.FilterName)

				} else {
					h.logger.Info("Auto-delete is disabled for this chat")
				}
			}
		}()
		return
	}
	h.logger.Debug("Message allowed")
}
func (h *Handler) handleLinkCommand(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	parts := strings.Fields(upd.Message.Body.Text)
	if len(parts) < 2 {
		h.logger.Info("Invalid link command format")
		h.SendAutoDeleteMessage(ctx, upd.Message.Recipient.ChatId, messages.MsgLinkCommandInvalid)
		_ = h.deleteMessage(ctx, upd.Message.Body.Mid, "invalid_link_command")

		return
	}
	token := parts[1]

	if err := h.deleteMessage(ctx, upd.Message.Body.Mid, "admin_check"); err != nil {
		h.logger.Warn("Failed to delete link command (admin rights missing?)", "error", err)
		h.SendAutoDeleteMessage(ctx, upd.Message.Recipient.ChatId, messages.MsgLinkAdminError)
		return
	}

	isOwner, err := h.svc.IsChatOwner(ctx, upd.Message.Recipient.ChatId, upd.Message.Sender.UserId)
	if err != nil {
		h.logger.Error("Failed to check owner status for link", "error", err)
		h.SendAutoDeleteMessage(ctx, upd.Message.Recipient.ChatId, fmt.Sprintf(messages.MsgLinkGroupFail, "could not verify owner status"))
		return
	}
	if !isOwner {
		h.logger.Info("Non-owner user tried to link group", "user_id", upd.Message.Sender.UserId)
		h.SendAutoDeleteMessage(ctx, upd.Message.Recipient.ChatId, messages.MsgLinkUserNotOwner)
		return
	}

	if err := h.svc.LinkGroup(ctx, token, upd.Message.Recipient.ChatId, upd.Message.Sender.UserId); err != nil {
		h.logger.Error("Failed to link group", "error", err)
		h.SendAutoDeleteMessage(ctx, upd.Message.Recipient.ChatId, fmt.Sprintf(messages.MsgLinkGroupFail, err))
		return
	}

	successMsg := messages.MsgGroupLinkedSuccess
	if h.config.GroupLinkedSuccessText != "" {
		successMsg = h.config.GroupLinkedSuccessText
	}
	h.sendGroupMessage(ctx, upd.Message.Recipient.ChatId, successMsg)

	h.sendMainMenu(ctx, upd.Message.Sender.UserId)
}

func (h *Handler) handleMuteCommand(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	h.logger.Info("Mute command received",
		"reply_to", upd.Message.Body.ReplyTo,
		"text", upd.Message.Body.Text,
		"mid", upd.Message.Body.Mid,
	)
	if upd.Message.Link == nil {

		h.logger.Info("Mute command used without reply (Link is nil)")
		h.SendAutoDeleteMessage(ctx, upd.Message.Recipient.ChatId, messages.MsgMuteCommandInvalid)
		_ = h.deleteMessage(ctx, upd.Message.Body.Mid, "invalid_mute_command_cleanup")

		return
	}

	parts := strings.Fields(upd.Message.Body.Text)
	duration, err := time.ParseDuration(h.config.DefaultMuteDuration)
	if err != nil {
		h.logger.Error("Invalid default mute duration in config, using fallback", "error", err)
		duration = 30 * time.Minute
	}
	if len(parts) > 1 {
		var err error
		duration, err = time.ParseDuration(parts[1])
		if err != nil {
			h.logger.Info("Invalid mute duration format", "input", parts[1])
			h.SendAutoDeleteMessage(ctx, upd.Message.Recipient.ChatId, messages.MsgMuteDurationInvalid)
			_ = h.deleteMessage(ctx, upd.Message.Body.Mid, "invalid_mute_duration_cleanup")

			return
		}
	}

	targetUserID := upd.Message.Link.Sender.UserId

	targetUserName := upd.Message.Link.Sender.Name
	adminID := upd.Message.Sender.UserId
	chatID := upd.Message.Recipient.ChatId

	isAdmin, err := h.svc.IsChatAdmin(ctx, chatID, adminID)
	if err != nil {
		h.logger.Error("Failed to check real-time admin status for mute", "error", err)
	} else if !isAdmin {
		h.logger.Info("Non-admin user tried to use mute command", "user_id", adminID)
		h.SendAutoDeleteMessage(ctx, chatID, messages.MsgMuteAdminError)
		_ = h.deleteMessage(ctx, upd.Message.Body.Mid, "non_admin_mute_cleanup")
		return
	}

	err = h.svc.MuteUser(ctx, chatID, adminID, targetUserID, targetUserName, duration)

	if err != nil {
		h.logger.Error("Failed to mute user", "error", err)
		if strings.Contains(err.Error(), "not a bot admin") {
			h.SendAutoDeleteMessage(ctx, chatID, messages.MsgMuteAdminError)
			_ = h.deleteMessage(ctx, upd.Message.Body.Mid, "start_mute_admin_cleanup")

		}
		return
	}

	humanDuration := duration.String()
	h.SendAutoDeleteMessage(ctx, chatID, fmt.Sprintf(messages.MsgUserMuted, targetUserName, humanDuration))

	_ = h.deleteMessage(ctx, upd.Message.Body.Mid, "mute_command_cleanup")

}

package pipeline

import (
	"fmt"
)

type Payload struct {
	ChatID          int64
	SenderID        int64
	Text            string
	AttachmentTypes []string
}

func (p Payload) SenderIDUserKey(chatID int64) string {
	return fmt.Sprintf("%d:%d", chatID, p.SenderID)
}

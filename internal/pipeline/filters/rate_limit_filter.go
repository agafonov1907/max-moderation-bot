package filters

import (
	"context"
	"max-moderation-bot/internal/messages"
	"max-moderation-bot/internal/pipeline"
	"sync"
	"time"
)

type RateLimitFilter struct {
	mu            sync.Mutex
	msgTimestamps map[string][]time.Time
	limit         int
	window        time.Duration
}

func NewRateLimitFilter(limit int, window time.Duration) *RateLimitFilter {
	return &RateLimitFilter{
		msgTimestamps: make(map[string][]time.Time),
		limit:         limit,
		window:        window,
	}
}

func (f *RateLimitFilter) Name() string {
	return "rate_limit_filter"
}

func (f *RateLimitFilter) Process(_ context.Context, payload pipeline.Payload) (*pipeline.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := payload.SenderIDUserKey(payload.ChatID)
	now := time.Now()

	timestamps := f.msgTimestamps[key]

	var validTimestamps []time.Time
	for _, t := range timestamps {
		if now.Sub(t) <= f.window {
			validTimestamps = append(validTimestamps, t)
		}
	}

	validTimestamps = append(validTimestamps, now)
	f.msgTimestamps[key] = validTimestamps

	if len(validTimestamps) > f.limit {
		return &pipeline.Result{
			IsAllowed:    false,
			Reason:       messages.MsgReasonRateLimit,
			FilterName:   f.Name(),
			ShouldDelete: true,
			ShouldMute:   true,
			MuteDuration: 1 * time.Hour,
		}, nil
	}

	return &pipeline.Result{IsAllowed: true}, nil
}

package filters

import (
	"context"
	"max-moderation-bot/internal/pipeline"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimitFilter_Process(t *testing.T) {
	filter := NewRateLimitFilter(5, 100*time.Millisecond)

	ctx := context.Background()
	payload := pipeline.Payload{
		ChatID:   -100,
		SenderID: 123,
		Text:     "text",
	}

	for i := 0; i < 5; i++ {
		res, err := filter.Process(ctx, payload)
		assert.NoError(t, err)
		assert.True(t, res.IsAllowed, "Message %d should be allowed", i+1)
	}

	res, err := filter.Process(ctx, payload)
	assert.NoError(t, err)
	assert.False(t, res.IsAllowed, "6th message should be blocked")
	assert.True(t, res.ShouldMute, "Should trigger mute")
	assert.True(t, res.ShouldDelete, "Should trigger delete")

	payload2 := pipeline.Payload{
		ChatID:   -100,
		SenderID: 456,
		Text:     "text",
	}
	res, err = filter.Process(ctx, payload2)
	assert.NoError(t, err)
	assert.True(t, res.IsAllowed, "Different user should be allowed")

	time.Sleep(150 * time.Millisecond)

	res, err = filter.Process(ctx, payload)
	assert.NoError(t, err)
	assert.True(t, res.IsAllowed, "Message after window should be allowed")
}

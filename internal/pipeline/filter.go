package pipeline

import (
	"context"
	"time"
)

type Result struct {
	IsAllowed    bool
	Reason       string
	FilterName   string
	ShouldDelete bool
	ShouldMute   bool
	MuteDuration time.Duration
}
type Filter interface {
	Name() string
	Process(ctx context.Context, payload Payload) (*Result, error)
}

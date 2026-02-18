package pipeline

import (
	"context"
	"testing"
)

type mockFilter struct {
	name      string
	shouldErr bool
	allow     bool
	reason    string
}

func (f *mockFilter) Name() string { return f.name }
func (f *mockFilter) Process(_ context.Context, _ Payload) (*Result, error) {
	if f.shouldErr {
		return nil, context.DeadlineExceeded
	}
	if !f.allow {
		return &Result{
			IsAllowed:  false,
			Reason:     f.reason,
			FilterName: f.name,
		}, nil
	}
	return &Result{IsAllowed: true}, nil
}
func TestManager_Process(t *testing.T) {
	tests := []struct {
		name        string
		filters     []Filter
		message     string
		wantAllowed bool
		wantFilter  string
		wantErr     bool
	}{
		{
			name:        "No filters",
			filters:     []Filter{},
			message:     "hello",
			wantAllowed: true,
		},
		{
			name: "All pass",
			filters: []Filter{
				&mockFilter{name: "f1", allow: true},
				&mockFilter{name: "f2", allow: true},
			},
			message:     "hello",
			wantAllowed: true,
		},
		{
			name: "First fails",
			filters: []Filter{
				&mockFilter{name: "f1", allow: false, reason: "fail1"},
				&mockFilter{name: "f2", allow: true},
			},
			message:     "hello",
			wantAllowed: false,
			wantFilter:  "f1",
		},
		{
			name: "Second fails",
			filters: []Filter{
				&mockFilter{name: "f1", allow: true},
				&mockFilter{name: "f2", allow: false, reason: "fail2"},
			},
			message:     "hello",
			wantAllowed: false,
			wantFilter:  "f2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(tt.filters...)
			res, err := m.Process(context.Background(), Payload{ChatID: 123, Text: tt.message})
			if (err != nil) != tt.wantErr {
				t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if res.IsAllowed != tt.wantAllowed {
				t.Errorf("Process() allowed = %v, want %v", res.IsAllowed, tt.wantAllowed)
			}
			if !tt.wantAllowed && res.FilterName != tt.wantFilter {
				t.Errorf("Process() filter = %v, want %v", res.FilterName, tt.wantFilter)
			}
		})
	}
}

package pipeline

import "context"

type Manager struct {
	filters []Filter
}

func NewManager(filters ...Filter) *Manager {
	return &Manager{filters: filters}
}
func (m *Manager) Process(ctx context.Context, payload Payload) (*Result, error) {
	for _, f := range m.filters {
		res, err := f.Process(ctx, payload)
		if err != nil {
			return nil, err
		}
		if !res.IsAllowed {
			return res, nil
		}
	}
	return &Result{IsAllowed: true}, nil
}

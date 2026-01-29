package tests

import (
	"context"

	"github.com/mydehq/autotitle/internal/types"
)

// MockDB satisfies types.DatabaseRepository
type MockDB struct {
	path string
}

func (m *MockDB) Path() string                                       { return m.path }
func (m *MockDB) Save(ctx context.Context, media *types.Media) error { return nil }
func (m *MockDB) Load(ctx context.Context, provider, id string) (*types.Media, error) {
	return nil, nil
}
func (m *MockDB) Delete(ctx context.Context, provider, id string) error { return nil }
func (m *MockDB) DeleteAll(ctx context.Context) error                   { return nil }
func (m *MockDB) Exists(provider, id string) bool                       { return false }
func (m *MockDB) List(ctx context.Context, providerFilter string) ([]types.MediaSummary, error) {
	return nil, nil
}
func (m *MockDB) Search(ctx context.Context, query string) ([]types.MediaSummary, error) {
	return nil, nil
}

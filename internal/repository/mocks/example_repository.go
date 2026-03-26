package mocks

import (
	"context"
	"database/sql"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockExampleRepository is a mock implementation of ExampleRepository for testing.
type MockExampleRepository struct {
	mock.Mock
}

func (m *MockExampleRepository) FetchAll(ctx context.Context, params pagination.Params) ([]entity.Example, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.Example), args.Error(1)
}

func (m *MockExampleRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockExampleRepository) FetchByID(ctx context.Context, id uuid.UUID) (*entity.Example, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Example), args.Error(1)
}

func (m *MockExampleRepository) Store(ctx context.Context, tx *sql.Tx, example *entity.Example) error {
	args := m.Called(ctx, tx, example)
	return args.Error(0)
}

func (m *MockExampleRepository) Remove(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

package usecase

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/repository/mocks"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestExampleUseCase_GetAll(t *testing.T) {
	mockRepo := new(mocks.MockExampleRepository)
	logger := zerolog.Nop()
	mockTracer := tracer.NewNoopTracer()

	uc := NewExampleUseCase(mockRepo, nil, logger, mockTracer)

	expectedExamples := []entity.Example{
		{
			ID:          uuid.New(),
			Name:        "Test Example 1",
			Description: "Description 1",
			Status:      entity.ExampleStatusActive,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New(),
			Name:        "Test Example 2",
			Description: "Description 2",
			Status:      entity.ExampleStatusInactive,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	params := pagination.Params{Offset: 0, Limit: 10}

	mockRepo.On("Count", mock.Anything).Return(int64(2), nil)
	mockRepo.On("FetchAll", mock.Anything, params).Return(expectedExamples, nil)

	result, err := uc.GetAll(context.Background(), params)

	assert.NoError(t, err)
	assert.Len(t, result.Examples, 2)
	assert.Equal(t, expectedExamples[0].Name, result.Examples[0].Name)
	assert.Equal(t, int64(2), result.Meta.Total)
	assert.Equal(t, 0, result.Meta.Offset)
	assert.Equal(t, 10, result.Meta.Limit)
	mockRepo.AssertExpectations(t)
}

func TestExampleUseCase_GetAll_Error(t *testing.T) {
	mockRepo := new(mocks.MockExampleRepository)
	logger := zerolog.Nop()
	mockTracer := tracer.NewNoopTracer()

	uc := NewExampleUseCase(mockRepo, nil, logger, mockTracer)

	params := pagination.Params{Offset: 0, Limit: 10}

	mockRepo.On("Count", mock.Anything).Return(int64(0), nil)
	mockRepo.On("FetchAll", mock.Anything, params).Return(nil, errors.New("database error"))

	result, err := uc.GetAll(context.Background(), params)

	assert.Error(t, err)
	assert.Nil(t, result)

	// Verify it's wrapped as an AppError
	appErr := apperror.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, apperror.CodeInternal, appErr.Code)
	mockRepo.AssertExpectations(t)
}

func TestExampleUseCase_GetByID(t *testing.T) {
	mockRepo := new(mocks.MockExampleRepository)
	logger := zerolog.Nop()
	mockTracer := tracer.NewNoopTracer()

	uc := NewExampleUseCase(mockRepo, nil, logger, mockTracer)

	expectedID := uuid.New()
	expectedExample := &entity.Example{
		ID:          expectedID,
		Name:        "Test Example",
		Description: "Test Description",
		Status:      entity.ExampleStatusActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mockRepo.On("FetchByID", mock.Anything, expectedID).Return(expectedExample, nil)

	result, err := uc.GetByID(context.Background(), expectedID)

	assert.NoError(t, err)
	assert.Equal(t, expectedExample.ID, result.ID)
	assert.Equal(t, expectedExample.Name, result.Name)
	mockRepo.AssertExpectations(t)
}

func TestExampleUseCase_GetByID_NotFound(t *testing.T) {
	mockRepo := new(mocks.MockExampleRepository)
	logger := zerolog.Nop()
	mockTracer := tracer.NewNoopTracer()

	uc := NewExampleUseCase(mockRepo, nil, logger, mockTracer)

	nonExistentID := uuid.New()

	mockRepo.On("FetchByID", mock.Anything, nonExistentID).Return(nil, sql.ErrNoRows)

	result, err := uc.GetByID(context.Background(), nonExistentID)

	assert.Error(t, err)
	assert.Nil(t, result)

	// Verify it's a NotFound AppError
	appErr := apperror.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, apperror.CodeNotFound, appErr.Code)
	assert.True(t, apperror.IsNotFound(err))
	mockRepo.AssertExpectations(t)
}

func TestExampleUseCase_Create(t *testing.T) {
	mockRepo := new(mocks.MockExampleRepository)
	logger := zerolog.Nop()
	mockTracer := tracer.NewNoopTracer()

	db, mockDB, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	uc := NewExampleUseCase(mockRepo, db, logger, mockTracer)

	newExample := entity.Example{
		Name:        "New Example",
		Description: "New Description",
	}

	mockDB.ExpectBegin()
	mockRepo.On("Store", mock.Anything, mock.AnythingOfType("*sql.Tx"), mock.AnythingOfType("*entity.Example")).
		Run(func(args mock.Arguments) {
			example := args.Get(2).(*entity.Example)
			example.ID = uuid.New()
			example.CreatedAt = time.Now()
			example.UpdatedAt = time.Now()
		}).
		Return(nil)
	mockDB.ExpectCommit()

	result, err := uc.Create(context.Background(), newExample)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, entity.ExampleStatusActive, result.Status) // Default status should be set
	mockRepo.AssertExpectations(t)
}

func TestExampleUseCase_Create_StoreError(t *testing.T) {
	mockRepo := new(mocks.MockExampleRepository)
	logger := zerolog.Nop()
	mockTracer := tracer.NewNoopTracer()

	db, mockDB, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	uc := NewExampleUseCase(mockRepo, db, logger, mockTracer)

	newExample := entity.Example{
		Name:        "New Example",
		Description: "New Description",
	}

	mockDB.ExpectBegin()
	mockRepo.On("Store", mock.Anything, mock.AnythingOfType("*sql.Tx"), mock.AnythingOfType("*entity.Example")).
		Return(errors.New("store error"))
	mockDB.ExpectRollback()

	result, err := uc.Create(context.Background(), newExample)

	assert.Error(t, err)
	assert.Nil(t, result)

	// Verify it's wrapped as an AppError
	appErr := apperror.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, apperror.CodeInternal, appErr.Code)
	mockRepo.AssertExpectations(t)
}

func TestExampleUseCase_Delete(t *testing.T) {
	mockRepo := new(mocks.MockExampleRepository)
	logger := zerolog.Nop()
	mockTracer := tracer.NewNoopTracer()

	uc := NewExampleUseCase(mockRepo, nil, logger, mockTracer)

	exampleID := uuid.New()

	mockRepo.On("Remove", mock.Anything, exampleID).Return(nil)

	err := uc.Delete(context.Background(), exampleID)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestExampleUseCase_Delete_NotFound(t *testing.T) {
	mockRepo := new(mocks.MockExampleRepository)
	logger := zerolog.Nop()
	mockTracer := tracer.NewNoopTracer()

	uc := NewExampleUseCase(mockRepo, nil, logger, mockTracer)

	nonExistentID := uuid.New()

	mockRepo.On("Remove", mock.Anything, nonExistentID).Return(sql.ErrNoRows)

	err := uc.Delete(context.Background(), nonExistentID)

	assert.Error(t, err)

	// Verify it's a NotFound AppError
	appErr := apperror.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, apperror.CodeNotFound, appErr.Code)
	mockRepo.AssertExpectations(t)
}

func TestExampleUseCase_Delete_Error(t *testing.T) {
	mockRepo := new(mocks.MockExampleRepository)
	logger := zerolog.Nop()
	mockTracer := tracer.NewNoopTracer()

	uc := NewExampleUseCase(mockRepo, nil, logger, mockTracer)

	exampleID := uuid.New()

	mockRepo.On("Remove", mock.Anything, exampleID).Return(errors.New("delete error"))

	err := uc.Delete(context.Background(), exampleID)

	assert.Error(t, err)

	// Verify it's wrapped as an AppError
	appErr := apperror.GetAppError(err)
	assert.NotNil(t, appErr)
	assert.Equal(t, apperror.CodeInternal, appErr.Code)
	mockRepo.AssertExpectations(t)
}

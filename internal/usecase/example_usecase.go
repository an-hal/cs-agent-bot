package usecase

import (
	"context"
	"database/sql"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// GetAllResult contains the paginated result for GetAll operation
type GetAllResult struct {
	Examples []entity.Example
	Meta     pagination.Meta
}

// ExampleUseCase defines the business logic interface for example operations.
type ExampleUseCase interface {
	GetAll(ctx context.Context, params pagination.Params) (*GetAllResult, error)
	GetByID(ctx context.Context, id uuid.UUID) (*entity.Example, error)
	Create(ctx context.Context, example entity.Example) (*entity.Example, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type exampleUseCase struct {
	exampleRepo repository.ExampleRepository
	db          *sql.DB
	logger      zerolog.Logger
	tracer      tracer.Tracer
}

// NewExampleUseCase creates a new instance of ExampleUseCase.
func NewExampleUseCase(exampleRepo repository.ExampleRepository, db *sql.DB, logger zerolog.Logger, tracer tracer.Tracer) ExampleUseCase {
	return &exampleUseCase{
		exampleRepo: exampleRepo,
		db:          db,
		logger:      logger,
		tracer:      tracer,
	}
}

// GetAll retrieves all examples with pagination.
func (uc *exampleUseCase) GetAll(ctx context.Context, params pagination.Params) (*GetAllResult, error) {
	ctx, span := uc.tracer.Start(ctx, "example.usecase.GetAll")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, uc.logger)
	logger.Info().
		Str("usecase", "GetAll").
		Int("offset", params.Offset).
		Int("limit", params.Limit).
		Msg("Fetching all examples")

	// Get total count
	total, err := uc.exampleRepo.Count(ctx)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to count examples")
	}

	// Get paginated examples
	examples, err := uc.exampleRepo.FetchAll(ctx, params)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to fetch examples")
	}

	return &GetAllResult{
		Examples: examples,
		Meta:     pagination.NewMeta(params, total),
	}, nil
}

// GetByID retrieves an example by its UUID.
func (uc *exampleUseCase) GetByID(ctx context.Context, id uuid.UUID) (*entity.Example, error) {
	ctx, span := uc.tracer.Start(ctx, "example.usecase.GetByID")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, uc.logger)
	logger.Info().Str("usecase", "GetByID").Str("id", id.String()).Msg("Fetching example by ID")

	example, err := uc.exampleRepo.FetchByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperror.WrapNotFound(logger, err, "example", "Example not found")
		}
		return nil, apperror.WrapInternal(logger, err, "Failed to fetch example by ID")
	}

	return example, nil
}

// Create stores a new example in the database.
func (uc *exampleUseCase) Create(ctx context.Context, example entity.Example) (*entity.Example, error) {
	ctx, span := uc.tracer.Start(ctx, "example.usecase.Create")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, uc.logger)
	logger.Info().Str("usecase", "Create").Msg("Creating new example")

	// Set default status if not provided
	if example.Status == "" {
		example.Status = entity.ExampleStatusActive
	}

	tx, err := uc.db.Begin()
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to begin transaction")
	}

	err = uc.exampleRepo.Store(ctx, tx, &example)
	if err != nil {
		_ = tx.Rollback()
		return nil, apperror.WrapInternal(logger, err, "Failed to store example")
	}

	err = tx.Commit()
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to commit transaction")
	}

	logger.Info().Str("example_id", example.ID.String()).Msg("Example created successfully")
	return &example, nil
}

// Delete removes an example by its UUID (soft delete).
func (uc *exampleUseCase) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := uc.tracer.Start(ctx, "example.usecase.Delete")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, uc.logger)
	logger.Info().Str("usecase", "Delete").Str("id", id.String()).Msg("Deleting example")

	err := uc.exampleRepo.Remove(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return apperror.WrapNotFound(logger, err, "example", "Example not found")
		}
		return apperror.WrapInternal(logger, err, "Failed to delete example")
	}

	return nil
}

package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// RevenueTargetHandler handles revenue target CRUD endpoints.
type RevenueTargetHandler struct {
	repo   repository.RevenueTargetRepository
	logger zerolog.Logger
	tracer tracer.Tracer
}

// NewRevenueTargetHandler creates a new RevenueTargetHandler.
func NewRevenueTargetHandler(repo repository.RevenueTargetRepository, logger zerolog.Logger, tr tracer.Tracer) *RevenueTargetHandler {
	return &RevenueTargetHandler{repo: repo, logger: logger, tracer: tr}
}

// List godoc
// @Summary      List revenue targets
// @Description  Get all revenue targets for a workspace.
// @Tags         Revenue Targets
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=[]entity.RevenueTarget}
// @Failure      500  {object}  response.StandardResponse
// @Router       /revenue-targets [get]
func (h *RevenueTargetHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "revenue_target.handler.List")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)

	targets, err := h.repo.List(ctx, wsID)
	if err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Revenue targets retrieved", targets)
}

type upsertTargetRequest struct {
	Year         int   `json:"year"`
	Month        int   `json:"month"`
	TargetAmount int64 `json:"target_amount"`
}

// Upsert godoc
// @Summary      Upsert revenue target
// @Description  Create or update a revenue target for a workspace.
// @Tags         Revenue Targets
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        body  body  upsertTargetRequest  true  "Revenue target"
// @Success      200  {object}  response.StandardResponse
// @Failure      400  {object}  response.StandardResponse
// @Router       /revenue-targets [put]
func (h *RevenueTargetHandler) Upsert(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "revenue_target.handler.Upsert")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)
	var actor string
	if u, ok := middleware.GetJWTUser(ctx); ok {
		actor = u.Email
	}

	var req upsertTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid request body")
	}

	if req.Year < 2020 || req.Year > 2100 {
		return apperror.BadRequest("year must be between 2020 and 2100")
	}
	if req.Month < 1 || req.Month > 12 {
		return apperror.BadRequest("month must be between 1 and 12")
	}
	if req.TargetAmount < 0 {
		return apperror.BadRequest("target_amount must be non-negative")
	}

	target := entity.RevenueTarget{
		WorkspaceID:  wsID,
		Year:         req.Year,
		Month:        req.Month,
		TargetAmount: req.TargetAmount,
		CreatedBy:    actor,
	}

	if err := h.repo.Upsert(ctx, target); err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Revenue target saved", nil)
}

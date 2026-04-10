package dashboard

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"

	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	"github.com/rs/zerolog"
)

// actorFromCtx extracts the authenticated user's email from the JWT context.
func actorFromCtx(r *http.Request) string {
	if u, ok := middleware.GetJWTUser(r.Context()); ok {
		return u.Email
	}
	return "unknown"
}

type ClientHandler struct {
	uc     dashboard.DashboardUsecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewClientHandler(uc dashboard.DashboardUsecase, logger zerolog.Logger, tr tracer.Tracer) *ClientHandler {
	return &ClientHandler{uc: uc, logger: logger, tracer: tr}
}

// List godoc
// @Summary      List clients
// @Description  Returns paginated clients for the workspace specified in the X-Workspace-ID header.
// @Tags         Dashboard
// @Param        X-Workspace-ID  header    string  true   "Workspace ID"
// @Param        search          query     string  false  "Search across company_id, company_name, etc."
// @Param        segment         query     string  false  "Filter by segment"
// @Param        payment_status  query     string  false  "Filter by payment status"
// @Param        sequence_cs     query     string  false  "Filter by CS sequence"
// @Param        plan_type       query     string  false  "Filter by plan type"
// @Param        bot_active      query     bool    false  "Filter by bot active status"
// @Param        offset          query     int     false  "Pagination offset (default 0)"
// @Param        limit           query     int     false  "Limit per page (default 10, max 100)"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.Client}
// @Failure      400  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/data-master/clients [get]
func (h *ClientHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ClientList")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	wsID := ctxutil.GetWorkspaceID(ctx)
	params := pagination.FromRequest(r)
	q := r.URL.Query()

	filter := entity.ClientFilter{
		WorkspaceIDs:  []string{wsID},
		Search:        q.Get("search"),
		Segment:       q.Get("segment"),
		PaymentStatus: q.Get("payment_status"),
		SequenceCS:    q.Get("sequence_cs"),
		PlanType:      q.Get("plan_type"),
	}
	if activeStr := q.Get("bot_active"); activeStr != "" {
		v, err := strconv.ParseBool(activeStr)
		if err == nil {
			filter.BotActive = &v
		}
	}

	logger.Info().Str("workspace_id", wsID).Str("search", filter.Search).Int("offset", params.Offset).Int("limit", params.Limit).Msg("Incoming list clients request")

	result, err := h.uc.GetClientsByWorkspaceID(ctx, filter, params)
	if err != nil {
		return err
	}

	clients := result.Clients
	if clients == nil {
		clients = []entity.Client{}
	}

	logger.Info().Int("count", len(clients)).Int64("total", result.Meta.Total).Msg("Successfully fetched clients")

	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Clients", result.Meta, clients)
}

// Get godoc
// @Summary      Get client by ID
// @Description  Returns a single client by company_id.
// @Tags         Dashboard
// @Param        company_id  path      string  true  "Company ID"
// @Success      200  {object}  response.StandardResponse{data=entity.Client}
// @Failure      404  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/clients/{company_id} [get]
func (h *ClientHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ClientGet")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	companyID := router.GetParam(r, "company_id")

	logger.Info().Str("company_id", companyID).Msg("Incoming get client request")

	client, err := h.uc.GetClient(ctx, companyID)
	if err != nil {
		return err
	}
	if client == nil {
		return apperror.NotFound("client", "Client not found")
	}

	logger.Info().Str("company_id", companyID).Msg("Successfully fetched client")

	return response.StandardSuccess(w, r, http.StatusOK, "Client", client)
}

// Create godoc
// @Summary      Create client
// @Description  Creates a new client in the workspace specified in the X-Workspace-ID header.
// @Tags         Dashboard
// @Param        X-Workspace-ID  header    string                true   "Workspace ID"
// @Param        body            body      CreateClientRequest   true   "Client payload"
// @Success      201  {object}  response.StandardResponse{data=entity.Client}
// @Failure      400  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/clients [post]
func (h *ClientHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ClientCreate")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	logger.Info().Msg("Incoming create client request")

	wsID := ctxutil.GetWorkspaceID(ctx)

	var client entity.Client
	if err := json.NewDecoder(r.Body).Decode(&client); err != nil {
		return apperror.ValidationError("Invalid request body")
	}

	if client.CompanyID == "" || client.CompanyName == "" {
		return apperror.BadRequest("company_id and company_name are required")
	}

	client.WorkspaceID = wsID

	if err := h.uc.CreateClient(ctx, client); err != nil {
		return err
	}

	if err := h.uc.RecordActivity(ctx, entity.ActivityLog{
		WorkspaceID:  wsID,
		Category:     entity.ActivityCategoryData,
		ActorType:    entity.ActivityActorHuman,
		Actor:        actorFromCtx(r),
		Action:       "add_client",
		Target:       client.CompanyName,
		RefID:        client.CompanyID,
		ResourceType: entity.ActivityResourceClient,
		Detail:       "Company ID: " + client.CompanyID,
	}); err != nil {
		return err
	}

	logger.Info().Str("company_id", client.CompanyID).Msg("Successfully created client")

	return response.StandardSuccess(w, r, http.StatusCreated, "Client created", client)
}

type CreateClientRequest struct {
	CompanyID       string `json:"company_id"`
	CompanyName     string `json:"company_name"`
	PICName         string `json:"pic_name"`
	PICWA           string `json:"pic_wa"`
	PICEmail        string `json:"pic_email"`
	PICRole         string `json:"pic_role"`
	OwnerName       string `json:"owner_name"`
	OwnerWA         string `json:"owner_wa"`
	OwnerTelegramID string `json:"owner_telegram_id"`
	Segment         string `json:"segment"`
	PlanType        string `json:"plan_type"`
	HCSize          string `json:"hc_size"`
}

// Update godoc
// @Summary      Update client
// @Description  Partially updates a client by company_id.
// @Tags         Dashboard
// @Param        company_id  path      string  true  "Company ID"
// @Param        body        body      map[string]interface{}  true  "Fields to update"
// @Success      200  {object}  response.StandardResponse
// @Failure      400  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/clients/{company_id} [put]
func (h *ClientHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ClientUpdate")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	companyID := router.GetParam(r, "company_id")

	logger.Info().Str("company_id", companyID).Msg("Incoming update client request")

	var patch map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		return apperror.ValidationError("Invalid request body")
	}

	if err := h.uc.UpdateClient(ctx, companyID, patch); err != nil {
		return err
	}

	patchKeys := make([]string, 0, len(patch))
	for k := range patch {
		patchKeys = append(patchKeys, k)
	}
	sort.Strings(patchKeys)

	if err := h.uc.RecordActivity(ctx, entity.ActivityLog{
		WorkspaceID:  ctxutil.GetWorkspaceID(ctx),
		Category:     entity.ActivityCategoryData,
		ActorType:    entity.ActivityActorHuman,
		Actor:        actorFromCtx(r),
		Action:       "edit_client",
		Target:       companyID,
		RefID:        companyID,
		ResourceType: entity.ActivityResourceClient,
		Detail:       "Ubah: " + strings.Join(patchKeys, ", "),
	}); err != nil {
		return err
	}

	logger.Info().Str("company_id", companyID).Msg("Successfully updated client")

	return response.StandardSuccess(w, r, http.StatusOK, "Client updated", nil)
}

// Delete godoc
// @Summary      Delete client
// @Description  Soft-deletes a client by company_id (sets blacklisted=true).
// @Tags         Dashboard
// @Param        company_id  path      string  true  "Company ID"
// @Success      200  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/clients/{company_id} [delete]
func (h *ClientHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ClientDelete")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	companyID := router.GetParam(r, "company_id")

	logger.Info().Str("company_id", companyID).Msg("Incoming delete client request")

	if err := h.uc.DeleteClient(ctx, companyID); err != nil {
		return err
	}

	if err := h.uc.RecordActivity(ctx, entity.ActivityLog{
		WorkspaceID:  ctxutil.GetWorkspaceID(ctx),
		Category:     entity.ActivityCategoryData,
		ActorType:    entity.ActivityActorHuman,
		Actor:        actorFromCtx(r),
		Action:       "delete_client",
		Target:       companyID,
		RefID:        companyID,
		ResourceType: entity.ActivityResourceClient,
		Detail:       "Company ID: " + companyID,
	}); err != nil {
		return err
	}

	logger.Info().Str("company_id", companyID).Msg("Successfully deleted client")

	return response.StandardSuccess(w, r, http.StatusOK, "Client deleted", nil)
}


package dashboard

import (
	"encoding/json"
	"net/http"

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
// @Description  Returns paginated clients for a given workspace slug.
// @Tags         Dashboard
// @Param        workspace  query     string  false  "Workspace slug"  default(dealls)
// @Param        offset     query     int     false  "Pagination offset (default 0)"
// @Param        limit      query     int     false  "Limit per page (default 10, max 100)"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.Client}
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/clients [get]
func (h *ClientHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ClientList")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	workspace := r.URL.Query().Get("workspace")
	params := pagination.FromRequest(r)

	logger.Info().Str("workspace", workspace).Int("offset", params.Offset).Int("limit", params.Limit).Msg("Incoming list clients request")

	clients, total, err := h.uc.GetClients(ctx, workspace, params)
	if err != nil {
		return err
	}
	if clients == nil {
		clients = []entity.Client{}
	}

	logger.Info().Int("count", len(clients)).Int64("total", total).Msg("Successfully fetched clients")

	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Clients", pagination.NewMeta(params, total), clients)
}

// ListByWorkspaceID godoc
// @Summary      List clients by workspace ID
// @Description  Returns paginated clients for a given workspace ID.
// @Tags         Dashboard
// @Param        workspace_id  path      string  true   "Workspace ID"
// @Param        offset        query     int     false  "Pagination offset (default 0)"
// @Param        limit         query     int     false  "Limit per page (default 10, max 100)"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.Client}
// @Failure      400  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/workspaces/{workspace_id}/clients [get]
func (h *ClientHandler) ListByWorkspaceID(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ClientListByWorkspaceID")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	workspaceID := router.GetParam(r, "workspace_id")
	if workspaceID == "" {
		return apperror.BadRequest("workspace_id is required")
	}

	params := pagination.FromRequest(r)

	logger.Info().Str("workspace_id", workspaceID).Int("offset", params.Offset).Int("limit", params.Limit).Msg("Incoming list clients by workspace ID request")

	result, err := h.uc.GetClientsByWorkspaceID(ctx, workspaceID, params)
	if err != nil {
		return err
	}

	clients := result.Clients
	if clients == nil {
		clients = []entity.Client{}
	}

	logger.Info().Int("count", len(clients)).Int64("total", result.Meta.Total).Msg("Successfully fetched clients by workspace ID")

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
// @Description  Creates a new client in the specified workspace.
// @Tags         Dashboard
// @Param        workspace  query     string                false  "Workspace slug"  default(dealls)
// @Param        body       body      CreateClientRequest   true   "Client payload"
// @Success      201  {object}  response.StandardResponse{data=entity.Client}
// @Failure      400  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/clients [post]
func (h *ClientHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ClientCreate")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	logger.Info().Msg("Incoming create client request")

	workspace := r.URL.Query().Get("workspace")
	if workspace == "" {
		workspace = "dealls"
	}

	var client entity.Client
	if err := json.NewDecoder(r.Body).Decode(&client); err != nil {
		return apperror.ValidationError("Invalid request body")
	}

	if client.CompanyID == "" || client.CompanyName == "" {
		return apperror.BadRequest("company_id and company_name are required")
	}

	ws, err := h.uc.GetWorkspaceBySlug(ctx, workspace)
	if err != nil {
		return err
	}
	if ws == nil {
		return apperror.BadRequest("Invalid workspace")
	}
	client.WorkspaceID = ws.ID

	if err := h.uc.CreateClient(ctx, client); err != nil {
		return err
	}

	if err := h.uc.RecordActivity(ctx, entity.ActivityLog{
		WorkspaceID: ws.Slug,
		Category:    entity.ActivityCategoryData,
		ActorType:   entity.ActivityActorHuman,
		Actor:       actorFromCtx(r),
		Action:      "add_client",
		Target:      client.CompanyName,
		RefID:       client.CompanyID,
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

	if err := h.uc.RecordActivity(ctx, entity.ActivityLog{
		Category:  entity.ActivityCategoryData,
		ActorType: entity.ActivityActorHuman,
		Actor:     actorFromCtx(r),
		Action:    "edit_client",
		Target:    companyID,
		RefID:     companyID,
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
		Category:  entity.ActivityCategoryData,
		ActorType: entity.ActivityActorHuman,
		Actor:     actorFromCtx(r),
		Action:    "delete_client",
		Target:    companyID,
		RefID:     companyID,
	}); err != nil {
		return err
	}

	logger.Info().Str("company_id", companyID).Msg("Successfully deleted client")

	return response.StandardSuccess(w, r, http.StatusOK, "Client deleted", nil)
}

// GetInvoices godoc
// @Summary      Get client invoices
// @Description  Returns paginated invoices for a given client.
// @Tags         Dashboard
// @Param        company_id  path      string  true  "Company ID"
// @Param        offset      query     int     false  "Pagination offset (default 0)"
// @Param        limit       query     int     false  "Limit per page (default 10, max 100)"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.Invoice}
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/clients/{company_id}/invoices [get]
func (h *ClientHandler) GetInvoices(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ClientGetInvoices")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	companyID := router.GetParam(r, "company_id")
	params := pagination.FromRequest(r)

	logger.Info().Str("company_id", companyID).Msg("Incoming get invoices request")

	invoices, total, err := h.uc.GetClientInvoices(ctx, companyID, params)
	if err != nil {
		return err
	}
	if invoices == nil {
		invoices = []entity.Invoice{}
	}

	logger.Info().Str("company_id", companyID).Int("count", len(invoices)).Msg("Successfully fetched invoices")

	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Invoices", pagination.NewMeta(params, total), invoices)
}

// GetEscalations godoc
// @Summary      Get client escalations
// @Description  Returns paginated escalation records for a given client.
// @Tags         Dashboard
// @Param        company_id  path      string  true  "Company ID"
// @Param        offset      query     int     false  "Pagination offset (default 0)"
// @Param        limit       query     int     false  "Limit per page (default 10, max 100)"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.Escalation}
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/clients/{company_id}/escalations [get]
func (h *ClientHandler) GetEscalations(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ClientGetEscalations")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	companyID := router.GetParam(r, "company_id")
	params := pagination.FromRequest(r)

	logger.Info().Str("company_id", companyID).Msg("Incoming get escalations request")

	escalations, total, err := h.uc.GetClientEscalations(ctx, companyID, params)
	if err != nil {
		return err
	}
	if escalations == nil {
		escalations = []entity.Escalation{}
	}

	logger.Info().Str("company_id", companyID).Int("count", len(escalations)).Msg("Successfully fetched escalations")

	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Escalations", pagination.NewMeta(params, total), escalations)
}

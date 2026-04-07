package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
)

// actorFromCtx extracts the authenticated user's email from the JWT context.
func actorFromCtx(r *http.Request) string {
	if u, ok := middleware.GetJWTUser(r.Context()); ok {
		return u.Email
	}
	return "unknown"
}

type ClientHandler struct {
	uc dashboard.DashboardUsecase
}

func NewClientHandler(uc dashboard.DashboardUsecase) *ClientHandler {
	return &ClientHandler{uc: uc}
}

// List godoc
// @Summary      List clients
// @Description  Returns all clients for a given workspace. Use "holding" to aggregate across member workspaces. Defaults to "dealls" if omitted.
// @Tags         Dashboard
// @Param        workspace  query     string  false  "Workspace slug"  default(dealls)
// @Success      200  {object}  response.StandardResponse{data=[]entity.Client}
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/clients [get]
func (h *ClientHandler) List(w http.ResponseWriter, r *http.Request) error {
	workspace := r.URL.Query().Get("workspace")
	params := pagination.FromRequest(r)

	clients, total, err := h.uc.GetClients(r.Context(), workspace, params)
	if err != nil {
		return err
	}
	if clients == nil {
		clients = []entity.Client{}
	}
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
	workspaceID := router.GetParam(r, "workspace_id")
	if workspaceID == "" {
		response.StandardError(w, r, http.StatusBadRequest, "workspace_id is required", "VALIDATION_ERROR", nil, "")
		return nil
	}

	params := pagination.FromRequest(r)

	result, err := h.uc.GetClientsByWorkspaceID(r.Context(), workspaceID, params)
	if err != nil {
		return err
	}

	clients := result.Clients
	if clients == nil {
		clients = []entity.Client{}
	}

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
	companyID := router.GetParam(r, "company_id")

	client, err := h.uc.GetClient(r.Context(), companyID)
	if err != nil {
		return err
	}
	if client == nil {
		response.StandardError(w, r, http.StatusNotFound, "Client not found", "CLIENT_NOT_FOUND", nil, "")
		return nil
	}
	response.StandardSuccess(w, r, http.StatusOK, "Client", client)
	return nil
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

// Create godoc
// @Summary      Create client
// @Description  Creates a new client in the specified workspace. Defaults to "dealls" workspace if omitted.
// @Tags         Dashboard
// @Param        workspace  query     string                false  "Workspace slug"  default(dealls)
// @Param        body       body      CreateClientRequest   true   "Client payload"
// @Success      201  {object}  response.StandardResponse{data=entity.Client}
// @Failure      400  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/clients [post]
func (h *ClientHandler) Create(w http.ResponseWriter, r *http.Request) error {
	workspace := r.URL.Query().Get("workspace")
	if workspace == "" {
		workspace = "dealls"
	}

	var client entity.Client
	if err := json.NewDecoder(r.Body).Decode(&client); err != nil {
		response.StandardError(w, r, http.StatusBadRequest, "Invalid JSON body", "INVALID_BODY", nil, "")
		return nil
	}

	if client.CompanyID == "" || client.CompanyName == "" {
		response.StandardError(w, r, http.StatusBadRequest, "Company_ID and Company_Name are required", "VALIDATION_ERROR", nil, "")
		return nil
	}

	// Resolve workspace slug to ID
	ws, err := h.uc.GetWorkspaceBySlug(r.Context(), workspace)
	if err != nil {
		return err
	}
	if ws == nil {
		response.StandardError(w, r, http.StatusBadRequest, "Invalid workspace", "INVALID_WORKSPACE", nil, "")
		return nil
	}
	client.WorkspaceID = ws.ID

	if err := h.uc.CreateClient(r.Context(), client); err != nil {
		return err
	}

	if err := h.uc.RecordActivity(r.Context(), entity.ActivityLog{
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

	response.StandardSuccess(w, r, http.StatusCreated, "Client created", client)
	return nil
}

// Update godoc
// @Summary      Update client
// @Description  Partially updates a client by company_id. Accepts a JSON object with fields to update.
// @Tags         Dashboard
// @Param        company_id  path      string  true  "Company ID"
// @Param        body        body      map[string]interface{}  true  "Fields to update"
// @Success      200  {object}  response.StandardResponse
// @Failure      400  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/clients/{company_id} [put]
func (h *ClientHandler) Update(w http.ResponseWriter, r *http.Request) error {
	companyID := router.GetParam(r, "company_id")

	var patch map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		response.StandardError(w, r, http.StatusBadRequest, "Invalid JSON body", "INVALID_BODY", nil, "")
		return nil
	}

	if err := h.uc.UpdateClient(r.Context(), companyID, patch); err != nil {
		return err
	}

	if err := h.uc.RecordActivity(r.Context(), entity.ActivityLog{
		Category:  entity.ActivityCategoryData,
		ActorType: entity.ActivityActorHuman,
		Actor:     actorFromCtx(r),
		Action:    "edit_client",
		Target:    companyID,
		RefID:     companyID,
	}); err != nil {
		return err
	}

	response.StandardSuccess(w, r, http.StatusOK, "Client updated", nil)
	return nil
}

// Delete godoc
// @Summary      Delete client
// @Description  Soft-deletes a client by company_id (sets blacklisted=true, bot_active=false).
// @Tags         Dashboard
// @Param        company_id  path      string  true  "Company ID"
// @Success      200  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/clients/{company_id} [delete]
func (h *ClientHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	companyID := router.GetParam(r, "company_id")

	if err := h.uc.DeleteClient(r.Context(), companyID); err != nil {
		return err
	}

	if err := h.uc.RecordActivity(r.Context(), entity.ActivityLog{
		Category:  entity.ActivityCategoryData,
		ActorType: entity.ActivityActorHuman,
		Actor:     actorFromCtx(r),
		Action:    "delete_client",
		Target:    companyID,
		RefID:     companyID,
	}); err != nil {
		return err
	}

	response.StandardSuccess(w, r, http.StatusOK, "Client deleted", nil)
	return nil
}

// GetInvoices godoc
// @Summary      Get client invoices
// @Description  Returns all invoices for a given client.
// @Tags         Dashboard
// @Param        company_id  path      string  true  "Company ID"
// @Success      200  {object}  response.StandardResponse{data=[]entity.Invoice}
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/clients/{company_id}/invoices [get]
func (h *ClientHandler) GetInvoices(w http.ResponseWriter, r *http.Request) error {
	companyID := router.GetParam(r, "company_id")
	params := pagination.FromRequest(r)

	invoices, total, err := h.uc.GetClientInvoices(r.Context(), companyID, params)
	if err != nil {
		return err
	}
	if invoices == nil {
		invoices = []entity.Invoice{}
	}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Invoices", pagination.NewMeta(params, total), invoices)
}

// GetEscalations godoc
// @Summary      Get client escalations
// @Description  Returns all escalation records for a given client.
// @Tags         Dashboard
// @Param        company_id  path      string  true  "Company ID"
// @Success      200  {object}  response.StandardResponse{data=[]entity.Escalation}
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/clients/{company_id}/escalations [get]
func (h *ClientHandler) GetEscalations(w http.ResponseWriter, r *http.Request) error {
	companyID := router.GetParam(r, "company_id")
	params := pagination.FromRequest(r)

	escalations, total, err := h.uc.GetClientEscalations(r.Context(), companyID, params)
	if err != nil {
		return err
	}
	if escalations == nil {
		escalations = []entity.Escalation{}
	}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Escalations", pagination.NewMeta(params, total), escalations)
}

package dashboard

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
)

type WorkspaceHandler struct {
	uc dashboard.DashboardUsecase
}

func NewWorkspaceHandler(uc dashboard.DashboardUsecase) *WorkspaceHandler {
	return &WorkspaceHandler{uc: uc}
}

// List godoc
// @Summary      List workspaces
// @Description  Returns all workspaces. Holding workspace aggregates data from member workspaces.
// @Tags         Dashboard
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.Workspace}
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/workspaces [get]
func (h *WorkspaceHandler) List(w http.ResponseWriter, r *http.Request) error {
	workspaces, err := h.uc.GetWorkspaces(r.Context())
	if err != nil {
		return err
	}
	if workspaces == nil {
		workspaces = []entity.Workspace{}
	}
	meta := pagination.Meta{Total: int64(len(workspaces)), Offset: 0, Limit: len(workspaces)}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Workspaces", meta, workspaces)
}

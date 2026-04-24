package dashboard

import (
	"net/http"
	"strconv"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/fireflies"
	"github.com/rs/zerolog"
)

type FirefliesHandler struct {
	uc     fireflies.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewFirefliesHandler(uc fireflies.Usecase, logger zerolog.Logger, tr tracer.Tracer) *FirefliesHandler {
	return &FirefliesHandler{uc: uc, logger: logger, tracer: tr}
}

// List godoc
// @Summary      List ingested Fireflies transcripts
// @Tags         Fireflies
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true   "Workspace ID"
// @Param        status          query   string  false  "pending|running|succeeded|failed"
// @Param        limit           query   int     false  "Max items (default 50)"
// @Param        offset          query   int     false  "Offset"
// @Router       /api/fireflies/transcripts [get]
func (h *FirefliesHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.FirefliesList")
	defer span.End()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	out, total, err := h.uc.List(ctx, ctxutil.GetWorkspaceID(ctx), r.URL.Query().Get("status"), limit, offset)
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.FirefliesTranscript{}
	}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Fireflies transcripts",
		pagination.Meta{Total: total, Offset: offset, Limit: limit}, out)
}

// Get godoc
// @Summary      Get a Fireflies transcript by id
// @Tags         Fireflies
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        id              path    string  true  "Transcript ID"
// @Router       /api/fireflies/transcripts/{id} [get]
func (h *FirefliesHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.FirefliesGet")
	defer span.End()

	out, err := h.uc.Get(ctx, ctxutil.GetWorkspaceID(ctx), router.GetParam(r, "id"))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Fireflies transcript", out)
}

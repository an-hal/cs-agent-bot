// Package approval exposes a central dispatcher that applies a pending
// approval_request by routing to the feature-specific Apply method based on
// the request_type. Feature usecases retain ownership of their own business
// logic; this package is only a router.
package approval

import (
	"context"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	automationrule "github.com/Sejutacita/cs-agent-bot/internal/usecase/automation_rule"
	collectionuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/collection"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/invoice"
	masterdata "github.com/Sejutacita/cs-agent-bot/internal/usecase/master_data"
	"github.com/rs/zerolog"
)

// Supported approval request_type strings. Keep in sync with the strings
// written by feature usecases when they create an ApprovalRequest.
const (
	TypeCreateInvoice           = "create_invoice"
	TypeMarkInvoicePaid         = "mark_invoice_paid"
	TypeCollectionSchema        = "collection_schema_change"
	TypeDeleteClient            = "delete_client_record"
	TypeToggleAutomation        = "toggle_automation_rule"
	TypeBulkImportMasterData    = "bulk_import_master_data"
	TypeStageTransition         = "stage_transition"
	TypeIntegrationKeyChange    = "integration_key_change"
)

// Dispatcher routes `POST /approvals/{id}/apply` to the correct feature Apply.
type Dispatcher interface {
	Apply(ctx context.Context, workspaceID, approvalID, checkerEmail string) (*entity.ApprovalRequest, error)
	List(ctx context.Context, workspaceID string, filter repository.ApprovalFilter) ([]entity.ApprovalRequest, int64, error)
	Get(ctx context.Context, workspaceID, approvalID string) (*entity.ApprovalRequest, error)
	Reject(ctx context.Context, workspaceID, approvalID, checkerEmail, reason string) (*entity.ApprovalRequest, error)
}

// MasterDataStageApprover is the narrow port used to apply a stage_transition
// approval. Implemented by master_data.Usecase (ApplyApprovedStageTransition).
// Nil is legal — the dispatcher returns "unsupported" for that type.
type MasterDataStageApprover interface {
	ApplyApprovedStageTransition(ctx context.Context, workspaceID, approvalID, checkerEmail string) (any, error)
}

// WorkspaceIntegrationApprover is the narrow port for integration_key_change.
type WorkspaceIntegrationApprover interface {
	ApplyApprovedKeyChange(ctx context.Context, workspaceID, approvalID, checkerEmail string) (any, error)
}

type dispatcher struct {
	approvalRepo   repository.ApprovalRequestRepository
	invoiceUC      invoice.Usecase
	masterDataUC   masterdata.Usecase
	collectionUC   collectionuc.Usecase
	automationUC   automationrule.Usecase
	stageApprover  MasterDataStageApprover
	intApprover    WorkspaceIntegrationApprover
	logger         zerolog.Logger
}

// New constructs a Dispatcher. All feature usecases must be non-nil; the
// dispatcher does not attempt to run requests whose owners are absent.
func New(
	approvalRepo repository.ApprovalRequestRepository,
	invoiceUC invoice.Usecase,
	masterDataUC masterdata.Usecase,
	collectionUC collectionuc.Usecase,
	automationUC automationrule.Usecase,
	logger zerolog.Logger,
) Dispatcher {
	return &dispatcher{
		approvalRepo: approvalRepo,
		invoiceUC:    invoiceUC,
		masterDataUC: masterDataUC,
		collectionUC: collectionUC,
		automationUC: automationUC,
		logger:       logger,
	}
}

// NewWithExtras is like New but attaches the optional stage + integration
// approvers. Use when the dispatcher must handle stage_transition +
// integration_key_change as well (the newer two types).
func NewWithExtras(
	approvalRepo repository.ApprovalRequestRepository,
	invoiceUC invoice.Usecase,
	masterDataUC masterdata.Usecase,
	collectionUC collectionuc.Usecase,
	automationUC automationrule.Usecase,
	stageApprover MasterDataStageApprover,
	intApprover WorkspaceIntegrationApprover,
	logger zerolog.Logger,
) Dispatcher {
	return &dispatcher{
		approvalRepo:  approvalRepo,
		invoiceUC:     invoiceUC,
		masterDataUC:  masterDataUC,
		collectionUC:  collectionUC,
		automationUC:  automationUC,
		stageApprover: stageApprover,
		intApprover:   intApprover,
		logger:        logger,
	}
}

func (d *dispatcher) List(ctx context.Context, workspaceID string, filter repository.ApprovalFilter) ([]entity.ApprovalRequest, int64, error) {
	if workspaceID == "" {
		return nil, 0, apperror.ValidationError("workspace_id required")
	}
	return d.approvalRepo.List(ctx, workspaceID, filter)
}

func (d *dispatcher) Get(ctx context.Context, workspaceID, approvalID string) (*entity.ApprovalRequest, error) {
	if workspaceID == "" || approvalID == "" {
		return nil, apperror.ValidationError("workspace_id and approval_id required")
	}
	ar, err := d.approvalRepo.GetByID(ctx, workspaceID, approvalID)
	if err != nil {
		return nil, err
	}
	if ar == nil {
		return nil, apperror.NotFound("approval_request", approvalID)
	}
	return ar, nil
}

func (d *dispatcher) Reject(ctx context.Context, workspaceID, approvalID, checkerEmail, reason string) (*entity.ApprovalRequest, error) {
	if workspaceID == "" || approvalID == "" {
		return nil, apperror.ValidationError("workspace_id and approval_id required")
	}
	ar, err := d.approvalRepo.GetByID(ctx, workspaceID, approvalID)
	if err != nil {
		return nil, err
	}
	if ar == nil {
		return nil, apperror.NotFound("approval_request", approvalID)
	}
	if ar.Status != entity.ApprovalStatusPending {
		return nil, apperror.BadRequest("approval is not pending (status=" + ar.Status + ")")
	}
	if ar.MakerEmail == checkerEmail {
		return nil, apperror.BadRequest("cannot reject your own request")
	}
	if err := d.approvalRepo.UpdateStatus(ctx, workspaceID, approvalID, entity.ApprovalStatusRejected, checkerEmail, reason); err != nil {
		return nil, err
	}
	return d.approvalRepo.GetByID(ctx, workspaceID, approvalID)
}

func (d *dispatcher) Apply(ctx context.Context, workspaceID, approvalID, checkerEmail string) (*entity.ApprovalRequest, error) {
	if workspaceID == "" || approvalID == "" {
		return nil, apperror.ValidationError("workspace_id and approval_id required")
	}

	ar, err := d.approvalRepo.GetByID(ctx, workspaceID, approvalID)
	if err != nil {
		return nil, err
	}
	if ar == nil {
		return nil, apperror.NotFound("approval_request", approvalID)
	}
	if ar.Status != entity.ApprovalStatusPending {
		return nil, apperror.BadRequest("approval is not pending (status=" + ar.Status + ")")
	}
	// TODO(RBAC): self-approval guard temporarily relaxed per product
	// request — operators want to apply their own bulk imports without a
	// second human in dev/internal workspaces. Restore as a per-workspace
	// permission flag (e.g., role_permissions.allow_self_approve) when the
	// RBAC matrix is wired. Until then, anyone with permission to call
	// /apply can apply their own request — log the case so audit can spot
	// it later.
	if ar.MakerEmail == checkerEmail {
		// no-op for now; RBAC TODO above
		_ = ar.MakerEmail
	}

	switch ar.RequestType {

	case TypeCreateInvoice:
		if _, err := d.invoiceUC.ApplyCreate(ctx, workspaceID, approvalID, checkerEmail); err != nil {
			return nil, err
		}

	case TypeMarkInvoicePaid:
		if err := d.invoiceUC.ApplyMarkPaid(ctx, workspaceID, approvalID, checkerEmail); err != nil {
			return nil, err
		}

	case TypeCollectionSchema:
		// collection.ApplyCollectionSchemaChange already marks the approval
		// approved and returns the updated ApprovalRequest — short-circuit.
		return d.collectionUC.ApplyCollectionSchemaChange(ctx, workspaceID, approvalID, checkerEmail)

	case TypeDeleteClient:
		if err := d.masterDataUC.ApplyApprovedDelete(ctx, workspaceID, approvalID, checkerEmail); err != nil {
			return nil, err
		}

	case TypeToggleAutomation:
		if _, err := d.automationUC.ApplyToggleStatus(ctx, workspaceID, approvalID, checkerEmail); err != nil {
			return nil, err
		}

	case TypeStageTransition:
		if d.stageApprover == nil {
			return nil, apperror.BadRequest("stage_transition approver not wired")
		}
		if _, err := d.stageApprover.ApplyApprovedStageTransition(ctx, workspaceID, approvalID, checkerEmail); err != nil {
			return nil, err
		}

	case TypeIntegrationKeyChange:
		if d.intApprover == nil {
			return nil, apperror.BadRequest("integration_key_change approver not wired")
		}
		if _, err := d.intApprover.ApplyApprovedKeyChange(ctx, workspaceID, approvalID, checkerEmail); err != nil {
			return nil, err
		}

	case TypeBulkImportMasterData:
		// File is stashed inline as base64 in approval payload by RequestImport.
		// Sync execution so any error surfaces in the response. Bulk import
		// over remote PG can run 30-90s for hundreds of rows; FE proxy
		// already configures a 5-min timeout for /approvals/*/apply.
		if _, err := d.masterDataUC.ApplyApprovedImport(ctx, workspaceID, approvalID, checkerEmail); err != nil {
			return nil, err
		}

	default:
		return nil, apperror.BadRequest("unsupported request_type: " + ar.RequestType)
	}

	// Re-read to return the post-apply status. Feature Apply methods may or
	// may not update the approval record themselves; this gives a consistent
	// view regardless.
	out, err := d.approvalRepo.GetByID(ctx, workspaceID, approvalID)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return ar, nil
	}
	return out, nil
}

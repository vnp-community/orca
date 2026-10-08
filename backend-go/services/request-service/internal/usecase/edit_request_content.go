package usecase

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// EditInput is a user edit. Patch members that are nil stay as they are.
type EditInput struct {
	RequestID        string
	ExpectedRevision int
	Patch            domain.ContentPatch
}

func ErrContentNotEditable(status domain.RequestStatus) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_CONTENT_NOT_EDITABLE",
		"request content can no longer be edited in status "+string(status)+"; answer a clarification or change the type instead", nil)
}

// editableStatus: the content is still being shaped (before analysis starts) or the request was sent back.
func editableStatus(s domain.RequestStatus) bool {
	switch s {
	case domain.RequestStatusNew, domain.RequestStatusClassifying, domain.RequestStatusAwaitingTypeConfirmation, domain.RequestStatusRequestBacklog:
		return true
	}
	return false
}

// EditRequestContent is the user-facing edit: guarded by expected_revision and by the request status.
type EditRequestContent struct {
	repo   RequestRepository
	append *AppendRequestRevision
	tx     TxRunner
}

func NewEditRequestContent(repo RequestRepository, appendRevision *AppendRequestRevision, tx TxRunner) *EditRequestContent {
	return &EditRequestContent{repo: repo, append: appendRevision, tx: tx}
}

func (uc *EditRequestContent) Execute(ctx context.Context, in EditInput) (domain.Request, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Request{}, domain.ErrRequestTenantRequired()
	}
	var out domain.Request
	err := uc.tx.InTx(ctx, func(txCtx context.Context) error {
		r, err := loadReadableRequest(txCtx, uc.repo, in.RequestID)
		if err != nil {
			return err
		}
		// Interim write rule until README v6 section 8 settles request-level permissions.
		if !canWriteRequest(txCtx, r) {
			return errRequestForbidden("only the reporter or an admin may edit request content")
		}
		if !editableStatus(r.Status) {
			return ErrContentNotEditable(r.Status)
		}
		if in.ExpectedRevision != r.ContentRevision {
			return domain.ErrRequestVersionConflict(r.ID, int64(in.ExpectedRevision))
		}
		c, err := domain.ContentFromRequest(r)
		if err != nil {
			return err
		}
		edited, err := c.Apply(in.Patch)
		if err != nil {
			return err
		}
		res, err := uc.append.AppendWithinTx(txCtx, AppendInput{
			RequestID: r.ID, Content: edited, Cause: domain.RevisionCauseEdited,
			ActorID: callerID(txCtx), ActorKind: domain.ActorKindUser, ExpectedVersion: r.Version,
		})
		out = res.Request
		return err
	})
	return out, err
}

// PatchFromJSON builds a patch from the RPC members: empty strings mean "unchanged" for the JSON
// members, and title/body are applied only when the caller sent them (hasTitle/hasBody).
func PatchFromJSON(title, body string, hasTitle, hasBody bool, acJSON, typeFieldsJSON string) (domain.ContentPatch, error) {
	var p domain.ContentPatch
	if hasTitle {
		p.Title = &title
	}
	if hasBody {
		p.Body = &body
	}
	if strings.TrimSpace(acJSON) != "" {
		var items []domain.ACInput
		if err := json.Unmarshal([]byte(acJSON), &items); err != nil {
			return p, domain.ErrRequestContentInvalid("acceptance_criteria_json: " + err.Error())
		}
		p.AcceptanceCriteria = &items
	}
	if strings.TrimSpace(typeFieldsJSON) != "" {
		var m map[string]any
		if err := json.Unmarshal([]byte(typeFieldsJSON), &m); err != nil {
			return p, domain.ErrRequestContentInvalid("type_fields_json: " + err.Error())
		}
		p.TypeFields = m
	}
	return p, nil
}

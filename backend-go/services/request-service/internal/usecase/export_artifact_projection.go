package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func ErrExportUnsupported(reason string) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ARTIFACT_EXPORT_UNSUPPORTED", reason, nil)
}

func ErrExportFormatUnsupported(format string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_ARTIFACT_FORMAT_UNSUPPORTED", fmt.Sprintf("format %q is not supported; use markdown", format), nil)
}

type ExportResult struct {
	Filename string
	Content  string
	Digest   string
}

// ExportArtifactProjection renders a stored artifact as Markdown/YAML. It only reads.
type ExportArtifactProjection struct {
	requests  RequestRepository
	revisions RequestRevisionRepository
	solutions SolutionCoreRepository
	index     ArtifactIndexRepository
	schemas   *domain.SchemaRegistry
}

func NewExportArtifactProjection(requests RequestRepository, revisions RequestRevisionRepository, solutions SolutionCoreRepository,
	index ArtifactIndexRepository, schemas *domain.SchemaRegistry) *ExportArtifactProjection {
	return &ExportArtifactProjection{requests: requests, revisions: revisions, solutions: solutions, index: index, schemas: schemas}
}

// Execute exports artifactRef of requestID; an empty ref exports the Request itself. A ref that belongs to another
// request is reported as not found.
func (uc *ExportArtifactProjection) Execute(ctx context.Context, requestID, artifactRef, format string) (ExportResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return ExportResult{}, domain.ErrRequestTenantRequired()
	}
	if format != "" && format != "markdown" {
		return ExportResult{}, ErrExportFormatUnsupported(format)
	}
	req, err := loadReadableRequest(ctx, uc.requests, requestID)
	if err != nil {
		return ExportResult{}, err
	}
	if artifactRef == "" {
		artifactRef = domain.FormatRequestID(req.Number)
	}
	id, err := domain.ParseDisplayID(artifactRef)
	if err != nil {
		return ExportResult{}, err
	}
	var md []byte
	switch id.Kind {
	case domain.DisplayKindRequest:
		if id.ReqNum != req.Number {
			return ExportResult{}, domain.ErrArtifactNotFound(artifactRef)
		}
		md, err = uc.exportRequest(req)
	case domain.DisplayKindSolution:
		md, err = uc.exportSolution(ctx, req, artifactRef)
	case domain.DisplayKindPlan:
		return ExportResult{}, ErrExportUnsupported("plan export needs the task-service tree reader, which is not wired yet")
	default:
		return ExportResult{}, ErrExportUnsupported("only request, solution and plan artifacts can be exported")
	}
	if err != nil {
		return ExportResult{}, err
	}
	if len(md) > domain.MaxProjectionBytes {
		return ExportResult{}, apperrors.New(apperrors.KindFailedPrecondition, domain.CodeArtifactLimitExceeded, "the projection is larger than 1 MB", nil)
	}
	return ExportResult{
		Filename: strings.ReplaceAll(artifactRef, "/", "_") + ".md",
		Content:  string(md),
		Digest:   "sha256:" + domain.DigestOfCanonical(md),
	}, nil
}

func (uc *ExportArtifactProjection) exportRequest(req domain.Request) ([]byte, error) {
	content, err := domain.ContentFromRequest(req)
	if err != nil {
		return nil, err
	}
	snap, err := content.Snapshot(nil)
	if err != nil {
		return nil, err
	}
	return domain.RenderArtifact(uc.schemas, domain.ArtifactKindRequest, snap, domain.ProjectionMeta{
		ID: domain.FormatRequestID(req.Number), Status: string(req.Status), Title: req.Title,
		Request: fmt.Sprintf("%s@r%d", domain.FormatRequestID(req.Number), req.ContentRevision),
	})
}

func (uc *ExportArtifactProjection) exportSolution(ctx context.Context, req domain.Request, ref string) ([]byte, error) {
	entry, err := uc.index.Resolve(ctx, ref)
	if err != nil || entry.RequestID != req.ID {
		return nil, domain.ErrArtifactNotFound(ref)
	}
	sol, err := uc.solutions.Get(ctx, entry.ArtifactID)
	if err != nil {
		return nil, domain.ErrArtifactNotFound(ref)
	}
	meta := domain.ProjectionMeta{ID: ref, Title: req.Title, Status: string(sol.Status), Request: domain.FormatRequestID(req.Number)}
	if sol.InputRequestRevision > 0 {
		meta.Request = fmt.Sprintf("%s@r%d", domain.FormatRequestID(req.Number), sol.InputRequestRevision)
	}
	return domain.RenderArtifact(uc.schemas, domain.ArtifactKindSolution, sol.OptionsJSON, meta)
}

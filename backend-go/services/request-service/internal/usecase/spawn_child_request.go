package usecase

import (
	"context"
	"strings"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type SpawnInput struct {
	ParentRequestID string
	LinkReason      domain.LinkReason
	Title           string
	Body            string
	TypeHint        domain.RequestType
	ClientRequestID string
	ActorID         string
	Provider        domain.SourceProvider // manual or mcp
}

type SpawnResult struct {
	Child   domain.Request
	Created bool
}

type SpawnChildRequest struct {
	repo    RequestRepository
	links   RequestLinkRepository
	creator ChildRequestCreator
	tx      TxRunner
}

func NewSpawnChildRequest(repo RequestRepository, links RequestLinkRepository, creator ChildRequestCreator, tx TxRunner) *SpawnChildRequest {
	return &SpawnChildRequest{repo: repo, links: links, creator: creator, tx: tx}
}

func (uc *SpawnChildRequest) Execute(ctx context.Context, in SpawnInput) (SpawnResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return SpawnResult{}, domain.ErrRequestTenantRequired()
	}
	if strings.TrimSpace(in.ClientRequestID) == "" {
		return SpawnResult{}, domain.ErrClientRequestIDRequired()
	}
	if in.Provider != domain.SourceProviderManual && in.Provider != domain.SourceProviderMCP {
		return SpawnResult{}, domain.ErrRequestInvalidSourceProvider(string(in.Provider))
	}
	parent, err := uc.repo.Get(ctx, in.ParentRequestID)
	if err != nil {
		if isNotFound(err) {
			return SpawnResult{}, domain.ErrParentRequestNotFound(in.ParentRequestID)
		}
		return SpawnResult{}, err
	}
	if err := domain.ValidateChild(parent, in.LinkReason, in.TypeHint); err != nil {
		return SpawnResult{}, err
	}
	// Limits are checked before the transaction, so they are soft under concurrency (a few over is accepted).
	children, err := uc.links.ListChildren(ctx, parent.ID)
	if err != nil {
		return SpawnResult{}, err
	}
	if len(children) >= domain.MaxChildrenPerParent {
		return SpawnResult{}, domain.ErrChildLimit(parent.ID, domain.MaxChildrenPerParent)
	}
	depth, err := uc.nestingLevel(ctx, parent.ID, map[string]bool{})
	if err != nil {
		return SpawnResult{}, err
	}
	if depth >= domain.MaxAncestorDepth {
		return SpawnResult{}, domain.ErrChildDepthExceeded(domain.MaxAncestorDepth)
	}

	var out SpawnResult
	err = uc.tx.InTx(ctx, func(ctx context.Context) error {
		res, err := uc.creator.CreateChild(ctx, ChildRequestInput{
			ProjectID: parent.ProjectID, Title: in.Title, Body: in.Body, Provider: in.Provider, ReporterID: in.ActorID,
			// Parent prefix: the same client id under two parents must give two children.
			IdempotencyKey: parent.ID + ":" + in.ClientRequestID,
			TypeHint:       in.TypeHint, ParentRequestID: parent.ID, LinkReason: in.LinkReason,
		})
		if err != nil {
			return err
		}
		out = SpawnResult{Child: res.Request, Created: res.Created}
		if !res.Created {
			return nil
		}
		link, err := domain.NewRequestLink(parent.ID, res.Request.ID, in.LinkReason, in.ActorID)
		if err != nil {
			return err
		}
		return uc.links.Insert(ctx, link)
	})
	if err != nil {
		return SpawnResult{}, err
	}
	return out, nil
}

// nestingLevel is the length of the longest ancestor chain ending at id (a root is level 1),
// walking at most MaxAncestorDepth steps so a cycle or deep chain cannot loop.
func (uc *SpawnChildRequest) nestingLevel(ctx context.Context, id string, seen map[string]bool) (int, error) {
	if seen[id] || len(seen) >= domain.MaxAncestorDepth {
		return len(seen) + 1, nil
	}
	seen[id] = true
	defer delete(seen, id)
	parents, err := uc.links.ListParents(ctx, id)
	if err != nil {
		return 0, err
	}
	best := 1
	for _, p := range parents {
		d, err := uc.nestingLevel(ctx, p.ParentRequestID, seen)
		if err != nil {
			return 0, err
		}
		if d+1 > best {
			best = d + 1
		}
	}
	return best, nil
}

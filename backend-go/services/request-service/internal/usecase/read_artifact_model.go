package usecase

import (
	"context"
	"fmt"
	"sort"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const (
	defaultRevisionPage = 50
	maxRevisionPage     = 200
)

// ArtifactReader holds the read-only RPC use cases of the artifact model. Reads follow GetRequest's rule
// (tenant scope; an id of another tenant or a malformed one is simply not found).
type ArtifactReader struct {
	requests  RequestRepository
	revisions RequestRevisionRepository
	coverage  RequestCoverageRepository
	relations ArtifactRelationRepository
	links     RequestLinkRepository
	index     ArtifactIndexRepository
	tree      PlanTreeReader
}

func NewArtifactReader(requests RequestRepository, revisions RequestRevisionRepository, coverage RequestCoverageRepository,
	relations ArtifactRelationRepository, links RequestLinkRepository, index ArtifactIndexRepository) *ArtifactReader {
	return &ArtifactReader{requests: requests, revisions: revisions, coverage: coverage, relations: relations, links: links, index: index}
}

// WithPlanTree adds the task-service reader that supplies contains and depends_on edges.
func (r *ArtifactReader) WithPlanTree(t PlanTreeReader) *ArtifactReader {
	r.tree = t
	return r
}

type RevisionPage struct {
	Revisions []domain.RequestRevision
	NextAfter int // 0 when there is no further page
}

// ListRevisions pages by revision number; afterRevision is the last number the caller saw.
func (r *ArtifactReader) ListRevisions(ctx context.Context, requestID string, afterRevision, pageSize int) (RevisionPage, error) {
	if _, err := loadReadableRequest(ctx, r.requests, requestID); err != nil {
		return RevisionPage{}, err
	}
	if pageSize <= 0 {
		pageSize = defaultRevisionPage
	}
	if pageSize > maxRevisionPage {
		pageSize = maxRevisionPage
	}
	got, err := r.revisions.List(ctx, requestID, afterRevision, pageSize+1)
	if err != nil {
		return RevisionPage{}, err
	}
	page := RevisionPage{Revisions: got}
	if len(got) > pageSize {
		page.Revisions = got[:pageSize]
		page.NextAfter = got[pageSize-1].Revision
	}
	return page, nil
}

func (r *ArtifactReader) GetRevision(ctx context.Context, requestID string, revision int) (domain.RequestRevision, error) {
	if _, err := loadReadableRequest(ctx, r.requests, requestID); err != nil {
		return domain.RequestRevision{}, err
	}
	return r.revisions.Get(ctx, requestID, revision)
}

type CoverageView struct {
	Rows           []domain.CoverageRow
	UncoveredACIDs []string
}

func (r *ArtifactReader) Coverage(ctx context.Context, requestID string) (CoverageView, error) {
	req, err := loadReadableRequest(ctx, r.requests, requestID)
	if err != nil {
		return CoverageView{}, err
	}
	rows, err := r.coverage.ListByRequest(ctx, requestID)
	if err != nil {
		return CoverageView{}, err
	}
	content, err := domain.ContentFromRequest(req)
	if err != nil {
		return CoverageView{}, err
	}
	view := CoverageView{Rows: rows, UncoveredACIDs: domain.UncoveredACs(content.AcceptanceCriteria, rows)}
	return view, nil
}

type ArtifactEdge struct {
	Rel      domain.Relation
	FromKind domain.NodeKind
	FromID   string
	ToKind   domain.NodeKind
	ToID     string
}

type ArtifactGraph struct {
	Edges   []ArtifactEdge
	Partial bool
}

// Graph merges artifact_relations, request_links (as spawned_by) and the task tree (contains, depends_on) into one
// ordered edge list. When the task tree cannot be read the rest is returned with Partial=true.
func (r *ArtifactReader) Graph(ctx context.Context, requestID string) (ArtifactGraph, error) {
	req, err := loadReadableRequest(ctx, r.requests, requestID)
	if err != nil {
		return ArtifactGraph{}, err
	}
	var g ArtifactGraph
	rels, err := r.relations.ListByRequest(ctx, requestID)
	if err != nil {
		return ArtifactGraph{}, err
	}
	for _, e := range rels {
		g.Edges = append(g.Edges, ArtifactEdge{Rel: e.Rel, FromKind: e.FromKind, FromID: e.FromID, ToKind: e.ToKind, ToID: e.ToID})
	}
	parents, err := r.links.ListParents(ctx, requestID)
	if err != nil {
		return ArtifactGraph{}, err
	}
	for _, l := range parents {
		g.Edges = append(g.Edges, ArtifactEdge{Rel: domain.RelationSpawnedBy, FromKind: domain.NodeRequest, FromID: requestID, ToKind: domain.NodeRequest, ToID: l.ParentRequestID})
	}
	if req.PlanTaskID != "" {
		sub, terr := r.readTree(ctx, req.PlanTaskID)
		if terr != nil {
			g.Partial = true
		} else {
			g.Edges = append(g.Edges, treeEdges(req.PlanTaskID, sub)...)
		}
	}
	sort.SliceStable(g.Edges, func(i, j int) bool {
		a, b := g.Edges[i], g.Edges[j]
		if a.Rel != b.Rel {
			return a.Rel < b.Rel
		}
		if a.FromID != b.FromID {
			return a.FromID < b.FromID
		}
		return a.ToID < b.ToID
	})
	return g, nil
}

func (r *ArtifactReader) readTree(ctx context.Context, planTaskID string) (PlanSubtree, error) {
	if r.tree == nil {
		return PlanSubtree{}, errNoPlanTree
	}
	return r.tree.GetSubtree(ctx, planTaskID)
}

var errNoPlanTree = domain.ErrArtifactNotFound("plan tree reader is not configured")

func treeEdges(planTaskID string, sub PlanSubtree) []ArtifactEdge {
	kinds := map[string]domain.NodeKind{planTaskID: domain.NodePlan}
	for _, n := range sub.Nodes {
		switch n.Kind {
		case "phase":
			kinds[n.ID] = domain.NodePhase
		case "task":
			kinds[n.ID] = domain.NodeTask
		case "plan":
			kinds[n.ID] = domain.NodePlan
		}
	}
	var out []ArtifactEdge
	for _, n := range sub.Nodes {
		if n.ParentID == "" || n.ID == planTaskID {
			continue
		}
		from, to := kinds[n.ParentID], kinds[n.ID]
		if domain.AllowedRelation(domain.RelationContains, from, to) {
			out = append(out, ArtifactEdge{Rel: domain.RelationContains, FromKind: from, FromID: n.ParentID, ToKind: to, ToID: n.ID})
		}
	}
	for _, e := range sub.DependsOn {
		from, to := kinds[e.From], kinds[e.To]
		if domain.AllowedRelation(domain.RelationDependsOn, from, to) {
			out = append(out, ArtifactEdge{Rel: domain.RelationDependsOn, FromKind: from, FromID: e.From, ToKind: to, ToID: e.To})
		}
	}
	return out
}

// Resolve turns a display id (SOL-142.2, PLN-142.1, ...) into the key behind it, scoped to the caller's tenant.
func (r *ArtifactReader) Resolve(ctx context.Context, ref string) (domain.IndexEntry, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.IndexEntry{}, domain.ErrRequestTenantRequired()
	}
	id, err := domain.ParseDisplayID(ref)
	if err != nil {
		return domain.IndexEntry{}, err
	}
	// Requests and their criteria resolve through the request number; the index holds the other entities.
	if id.Kind == domain.DisplayKindRequest || id.Kind == domain.DisplayKindAC {
		req, err := r.requests.GetByNumber(ctx, id.ReqNum)
		if err != nil {
			return domain.IndexEntry{}, domain.ErrArtifactNotFound(ref)
		}
		if id.Kind == domain.DisplayKindAC {
			c, cerr := domain.ContentFromRequest(req)
			if _, ok := c.AcceptanceCriteria.Find(fmt.Sprintf("AC-%d", id.Index)); cerr != nil || !ok {
				return domain.IndexEntry{}, domain.ErrArtifactNotFound(ref)
			}
		}
		return domain.IndexEntry{TenantID: req.TenantID, DisplayID: ref, Kind: id.Kind, RequestID: req.ID, ArtifactID: req.ID, CreatedAt: req.CreatedAt}, nil
	}
	if _, ok := domain.IndexKindFor(id.Kind); !ok {
		return domain.IndexEntry{}, domain.ErrArtifactNotFound(ref)
	}
	entry, err := r.index.Resolve(ctx, ref)
	if err != nil {
		return domain.IndexEntry{}, err
	}
	// Same read rule as GetRequest: the request behind the id must be visible.
	if _, err := loadReadableRequest(ctx, r.requests, entry.RequestID); err != nil {
		return domain.IndexEntry{}, domain.ErrArtifactNotFound(ref)
	}
	return entry, nil
}

package domain

import (
	"fmt"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

// Relation is one of the eight ontology relations (CR-REQ-027 section 2.3).
type Relation string

const (
	RelationDerivedFrom Relation = "derived_from"
	RelationImplements  Relation = "implements"
	RelationContains    Relation = "contains"
	RelationDependsOn   Relation = "depends_on"
	RelationVerifies    Relation = "verifies"
	RelationSupersedes  Relation = "supersedes"
	RelationSpawnedBy   Relation = "spawned_by"
	RelationEvidencedBy Relation = "evidenced_by"
)

func AllRelations() []Relation {
	return []Relation{
		RelationDerivedFrom, RelationImplements, RelationContains, RelationDependsOn,
		RelationVerifies, RelationSupersedes, RelationSpawnedBy, RelationEvidencedBy,
	}
}

// NodeKind is an end of a relation; wider than the index kinds because verifies
// links Checks to ACs and Tasks, and evidenced_by points at Evidence (CR-REQ-030, not built yet).
type NodeKind string

const (
	NodeRequest  NodeKind = "request"
	NodeSolution NodeKind = "solution"
	NodeOption   NodeKind = "option"
	NodePlan     NodeKind = "plan"
	NodePhase    NodeKind = "phase"
	NodeTask     NodeKind = "task"
	NodeAC       NodeKind = "ac"
	NodeCheck    NodeKind = "check"
	NodeEvidence NodeKind = "evidence"
)

func AllNodeKinds() []NodeKind {
	return []NodeKind{NodeRequest, NodeSolution, NodeOption, NodePlan, NodePhase, NodeTask, NodeAC, NodeCheck, NodeEvidence}
}

type relationTriple struct {
	rel      Relation
	from, to NodeKind
}

var allowedRelations = map[relationTriple]bool{
	{RelationDerivedFrom, NodeSolution, NodeRequest}:  true,
	{RelationDerivedFrom, NodePlan, NodeSolution}:     true,
	{RelationImplements, NodePlan, NodeOption}:        true,
	{RelationContains, NodePlan, NodePhase}:           true,
	{RelationContains, NodePlan, NodeTask}:            true,
	{RelationContains, NodePhase, NodeTask}:           true,
	{RelationDependsOn, NodeTask, NodeTask}:           true,
	{RelationDependsOn, NodePhase, NodePhase}:         true,
	{RelationVerifies, NodeCheck, NodeAC}:             true,
	{RelationVerifies, NodeCheck, NodeTask}:           true,
	{RelationSupersedes, NodeSolution, NodeSolution}:  true,
	{RelationSupersedes, NodePlan, NodePlan}:          true,
	{RelationSpawnedBy, NodeRequest, NodeRequest}:     true,
	{RelationEvidencedBy, NodeSolution, NodeEvidence}: true,
}

func AllowedRelation(rel Relation, from, to NodeKind) bool {
	return allowedRelations[relationTriple{rel, from, to}]
}

// StoredInRelationsTable is false for relations whose source of truth lives elsewhere:
// contains/depends_on in task-service, verifies in request_coverage, spawned_by in request_links.
func StoredInRelationsTable(rel Relation) bool {
	switch rel {
	case RelationDerivedFrom, RelationImplements, RelationSupersedes, RelationEvidencedBy:
		return true
	}
	return false
}

// ArtifactRelation is an append-only edge in artifact_relations.
type ArtifactRelation struct {
	ID             string
	TenantID       string
	RequestID      string
	Rel            Relation
	FromKind       NodeKind
	FromID         string
	ToKind         NodeKind
	ToID           string
	CreatedByRunID string
	CreatedAt      time.Time
}

func ErrArtifactRelationNotAllowed(rel Relation, from, to NodeKind) error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ARTIFACT_RELATION_NOT_ALLOWED",
		fmt.Sprintf("relation %s from %s to %s is not allowed", rel, from, to), nil)
}

// NewArtifactRelation refuses triples outside the ontology and relations that are not stored here.
func NewArtifactRelation(id, tenantID, requestID string, rel Relation, fromKind NodeKind, fromID string, toKind NodeKind, toID string) (ArtifactRelation, error) {
	if !AllowedRelation(rel, fromKind, toKind) || !StoredInRelationsTable(rel) {
		return ArtifactRelation{}, ErrArtifactRelationNotAllowed(rel, fromKind, toKind)
	}
	if fromID == "" || toID == "" {
		return ArtifactRelation{}, apperrors.New(apperrors.KindInvalidArgument, "REQUEST_ARTIFACT_ID_INVALID", "relation ends need an id", nil)
	}
	return ArtifactRelation{
		ID: id, TenantID: tenantID, RequestID: requestID, Rel: rel,
		FromKind: fromKind, FromID: fromID, ToKind: toKind, ToID: toID,
	}, nil
}

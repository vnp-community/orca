package contracttest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// notFoundProbe records one lookup/mutation aimed at a row that does not exist (well-formed unknown id)
// or that cannot exist (malformed id). TASK-E2E-017: it must surface as KindNotFound (or a quiet nil/false
// for "find"-style methods), never as an untyped error that the gRPC layer would turn into Internal.
type notFoundProbe struct {
	name string
	// quiet marks methods whose contract is "no row is not an error" (nil, false or empty result).
	quiet bool
	err   error
}

func (p notFoundProbe) check(t *testing.T) {
	t.Helper()
	if p.err == nil {
		if !p.quiet {
			t.Errorf("%s: want a NotFound error, got nil", p.name)
		}
		return
	}
	var ae *apperrors.AppError
	if !errors.As(p.err, &ae) || ae.Kind != apperrors.KindNotFound {
		t.Errorf("%s: want KindNotFound, got %T: %v", p.name, p.err, p.err)
	}
}

// missingIDs are an unknown well-formed uuid and a value no uuid column can hold.
func missingIDs() []string { return []string{uuid.NewString(), "abc"} }

// RunArtifactNotFoundContract probes every id-taking repository of the artifact/clarification/solution family.
func RunArtifactNotFoundContract(t *testing.T, newEnv func(t *testing.T) ArtifactEnv) {
	env := newEnv(t)
	ctx := CtxForTenant(newTenant())
	var probes []notFoundProbe
	add := func(name string, quiet bool, err error) { probes = append(probes, notFoundProbe{name, quiet, err}) }
	// A real row's shape with another id keeps unrelated column checks from masking the not-found path.
	template := createRequest(t, env.Env, ctx, nil)

	for _, id := range missingIDs() {
		ghost := template
		ghost.ID = id
		_, err := env.Requests.Get(ctx, id)
		add("Requests.Get "+id, false, err)
		_, err = env.Requests.List(ctx, usecase.ListFilter{ProjectID: id, PageSize: 10})
		add("Requests.List(project) "+id, true, err)
		_, err = env.Requests.List(ctx, usecase.ListFilter{ReporterID: id, PageSize: 10})
		add("Requests.List(reporter) "+id, true, err)
		_, err = env.Requests.Update(ctx, ghost, 1)
		add("Requests.Update "+id, false, err)
		add("Requests.UpdateSolutionEngine "+id, false, env.Requests.UpdateSolutionEngine(ctx, id, domain.EngineName("native"), 1))
		_, err = env.Content.UpdateContent(ctx, ghost, 1)
		add("Content.UpdateContent "+id, false, err)

		_, err = env.Revisions.Get(ctx, id, 1)
		add("Revisions.Get "+id, false, err)
		_, err = env.Index.Resolve(ctx, id)
		add("Index.Resolve "+id, false, err)
		add("Index.SetSolutionSeq "+id, false, env.Index.SetSolutionSeq(ctx, id, 1))

		_, err = env.Clarifications.Get(ctx, id)
		add("Clarifications.Get "+id, false, err)
		_, err = env.Clarifications.GetOpenByRequest(ctx, id)
		add("Clarifications.GetOpenByRequest "+id, true, err)
		add("Clarifications.UpdateAnswers "+id, false, env.Clarifications.UpdateAnswers(ctx, id, nil, 0))
		add("Clarifications.MarkAnswered "+id, false, env.Clarifications.MarkAnswered(ctx, id, 1, time.Now(), 0))
		add("Clarifications.MarkCancelled "+id, false, env.Clarifications.MarkCancelled(ctx, id, "x", time.Now(), 0))
		add("Clarifications.MarkExpired "+id, false, env.Clarifications.MarkExpired(ctx, id, time.Now()))
		_, err = env.Clarifications.List(ctx, usecase.ClarificationListFilter{RequestID: id})
		add("Clarifications.List "+id, true, err)

		_, err = env.Decisions.Get(ctx, id)
		add("Decisions.Get "+id, false, err)
		_, err = env.Decisions.GetLiveBySubject(ctx, domain.DecisionSubjectKind("solution"), id)
		add("Decisions.GetLiveBySubject "+id, true, err)
		_, err = env.Decisions.Update(ctx, domain.Decision{ID: id}, 1)
		add("Decisions.Update "+id, false, err)
		_, err = env.Decisions.ListHistory(ctx, id)
		add("Decisions.ListHistory "+id, true, err)
		_, err = env.Decisions.List(ctx, usecase.DecisionListFilter{RequestID: id})
		add("Decisions.List "+id, true, err)

		_, err = env.Solutions.Get(ctx, id)
		add("Solutions.Get "+id, false, err)
		_, err = env.Solutions.Update(ctx, domain.Solution{ID: id}, 1)
		add("Solutions.Update "+id, false, err)
		_, err = env.Solutions.ListByRequestID(ctx, id)
		add("Solutions.ListByRequestID "+id, true, err)
		if store, ok := env.Solutions.(usecase.SolutionStore); ok {
			_, err = store.Choose(ctx, id, 0, 1)
			add("Solutions.Choose "+id, true, err)
			add("Solutions.DeleteDraft "+id, true, store.DeleteDraft(ctx, id))
		}

		_, err = env.Runs.Get(ctx, id)
		add("ClassificationRuns.Get "+id, false, err)
		_, err = env.Links.ListParents(ctx, id)
		add("Links.ListParents "+id, true, err)
		_, err = env.Links.ListChildren(ctx, id)
		add("Links.ListChildren "+id, true, err)
	}
	for _, p := range probes {
		p.check(t)
	}
}

// RunApprovalNotFoundContract probes the approval, policy and request-lock repositories.
func RunApprovalNotFoundContract(t *testing.T, newEnv func(t *testing.T) ApprovalEnv) {
	env := newEnv(t)
	tenantID := newTenant()
	ctx := CtxForTenant(tenantID)
	var probes []notFoundProbe
	add := func(name string, quiet bool, err error) { probes = append(probes, notFoundProbe{name, quiet, err}) }

	for _, id := range missingIDs() {
		_, err := env.Approvals.Get(ctx, tenantID, id)
		add("Approvals.Get "+id, false, err)
		_, err = env.Approvals.GetForUpdate(ctx, tenantID, id)
		add("Approvals.GetForUpdate "+id, false, err)
		_, err = env.Approvals.FindPendingBySubject(ctx, tenantID, domain.SubjectPlan, id)
		add("Approvals.FindPendingBySubject "+id, true, err)
		_, err = env.Approvals.FindByIdempotencyKey(ctx, tenantID, id)
		add("Approvals.FindByIdempotencyKey "+id, true, err)
		_, err = env.Approvals.UpdateDecision(ctx, domain.Approval{ID: id, TenantID: tenantID}, 1)
		add("Approvals.UpdateDecision "+id, true, err)
		_, err = env.Approvals.UpdateDue(ctx, domain.Approval{ID: id, TenantID: tenantID}, 1)
		add("Approvals.UpdateDue "+id, true, err)
		_, err = env.Approvals.MarkReminded(ctx, tenantID, id, time.Now())
		add("Approvals.MarkReminded "+id, true, err)
		_, err = env.Approvals.CancelPendingForRequest(ctx, tenantID, id, "x", time.Now())
		add("Approvals.CancelPendingForRequest "+id, true, err)
		_, err = env.Approvals.UpdatePendingDigest(ctx, tenantID, domain.SubjectPlan, id, "d")
		add("Approvals.UpdatePendingDigest "+id, true, err)
		_, err = env.Approvers.ListForApproval(ctx, tenantID, id)
		add("Approvers.ListForApproval "+id, true, err)

		_, err = env.Policies.Get(ctx, tenantID, id)
		add("Policies.Get "+id, false, err)
		add("Policies.Delete "+id, false, env.Policies.Delete(ctx, tenantID, id, 1))
		_, err = env.Policies.Upsert(ctx, domain.ApprovalPolicy{ID: id, TenantID: tenantID, SubjectType: domain.SubjectPlan, Enabled: true}, 1)
		add("Policies.Upsert(update) "+id, false, err)

		add("Locker.LockRequest "+id, false, lockRequest(ctx, env, id))
	}
	for _, p := range probes {
		p.check(t)
	}
}

// lockRequest runs LockRequest in a transaction, as production does.
func lockRequest(ctx context.Context, env ApprovalEnv, id string) error {
	return env.Tx.InTx(ctx, func(ctx context.Context) error {
		_, err := env.Locker.LockRequest(ctx, id)
		return err
	})
}

// RunRetentionNotFoundContract: erasing a request that does not exist (or whose id is no uuid) is a NotFound,
// and only a miss may look like one.
func RunRetentionNotFoundContract(t *testing.T, newEnv func(t *testing.T) SecurityEnv) {
	env := newEnv(t)
	ctx := CtxForTenant(newTenant())
	pseudo := func(string) string { return uuid.NewString() }
	for _, id := range missingIDs() {
		_, err := env.Retention.Anonymize(ctx, id, pseudo, time.Now().UTC(), "")
		notFoundProbe{name: "Retention.Anonymize " + id, err: err}.check(t)
	}
}

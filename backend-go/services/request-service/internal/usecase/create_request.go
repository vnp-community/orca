package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type CreateRequestInput struct {
	ProjectID       string
	Title           string
	Body            string
	Source          domain.SourceRef
	Hints           domain.SourceHints
	ClientRequestID string
	// AllowTypeHint is set only by SpawnChildRequest; public callers cannot steer classification.
	AllowTypeHint bool
	// ActorKind is the kind recorded on the start_classification transition (default user).
	ActorKind domain.ActorKind
	// ParentRequestID and LinkReason are set only by SpawnChildRequest and ride on the created event.
	ParentRequestID string
	LinkReason      string
	// AcceptanceCriteria and TypeFields are optional initial content (CR-REQ-027); Jira and GitHub requests arrive without them.
	AcceptanceCriteria []domain.ACInput
	TypeFields         map[string]any
}

type CreateRequestResult struct {
	Request domain.Request
	Created bool
}

type CreateRequest struct {
	repo       RequestRepository
	idem       RequestIdempotencyRepository
	tx         TxRunner
	outbox     OutboxWriter
	transition RequestTransitioner
	issues     IssueFetcher
	recorder   RequestCreationRecorder
	flags      SecurityFlagStore
}

func NewCreateRequest(repo RequestRepository, idem RequestIdempotencyRepository, tx TxRunner, outbox OutboxWriter, transition RequestTransitioner, issues IssueFetcher) *CreateRequest {
	return &CreateRequest{repo: repo, idem: idem, tx: tx, outbox: outbox, transition: transition, issues: issues, recorder: NoopRequestCreationRecorder{}}
}

// WithCreationRecorder plugs the content-revision writer that must run in the creation transaction.
func (uc *CreateRequest) WithCreationRecorder(r RequestCreationRecorder) *CreateRequest {
	uc.recorder = r
	return uc
}

// WithSecurityFlags records contains_secret_suspected in the creation transaction; without it the text is
// still masked, only the flag is not kept.
func (uc *CreateRequest) WithSecurityFlags(f SecurityFlagStore) *CreateRequest {
	uc.flags = f
	return uc
}

// claimLost aborts the creation transaction (so no number is burnt) when another caller owns the key.
type claimLost struct{ existingID string }

func (claimLost) Error() string { return "idempotency key claimed by another request" }

func (uc *CreateRequest) Execute(ctx context.Context, in CreateRequestInput) (CreateRequestResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return CreateRequestResult{}, domain.ErrRequestTenantRequired()
	}
	reporter, _ := tenant.UserID(ctx)
	if reporter == "" {
		return CreateRequestResult{}, domain.ErrRequestReporterRequired()
	}
	if in.ProjectID == "" {
		return CreateRequestResult{}, domain.ErrRequestProjectRequired()
	}
	if _, err := uuid.Parse(in.ProjectID); err != nil {
		return CreateRequestResult{}, domain.ErrRequestProjectInvalid()
	}
	src, err := domain.NormalizeSourceRef(in.Source)
	if err != nil {
		return CreateRequestResult{}, err
	}
	key, keyed, err := domain.BuildIdempotencyKey(src, reporter, in.ClientRequestID)
	if err != nil {
		return CreateRequestResult{}, err
	}
	if keyed {
		if res, found, err := uc.existing(ctx, key); err != nil || found {
			return res, err
		}
	}

	in, err = uc.enrich(ctx, in, src)
	if err != nil {
		return CreateRequestResult{}, err
	}
	title, err := domain.NormalizeTitle(in.Title)
	if err != nil {
		return CreateRequestResult{}, err
	}
	body, err := domain.NormalizeBody(in.Body)
	if err != nil {
		return CreateRequestResult{}, err
	}
	in.Title, in.Body = title, body

	var created domain.Request
	err = uc.tx.InTx(ctx, func(txCtx context.Context) error {
		r, err := uc.CreateWithinTx(txCtx, in, src, tenantID, reporter, key, keyed)
		created = r
		return err
	})
	var lost claimLost
	if errors.As(err, &lost) {
		r, gerr := uc.repo.Get(ctx, lost.existingID)
		if gerr != nil {
			return CreateRequestResult{}, gerr
		}
		return CreateRequestResult{Request: r}, nil
	}
	if err != nil {
		return CreateRequestResult{}, err
	}
	return CreateRequestResult{Request: created, Created: true}, nil
}

func (uc *CreateRequest) existing(ctx context.Context, key domain.IdempotencyKey) (CreateRequestResult, bool, error) {
	id, err := uc.idem.Find(ctx, string(key.Provider), key.Site, key.Ref)
	if err != nil || id == "" {
		return CreateRequestResult{}, false, err
	}
	// A closed Request is returned as is: reopening is a deliberate lifecycle action, not a side effect of a retry.
	r, err := uc.repo.Get(ctx, id)
	if err != nil {
		return CreateRequestResult{}, false, err
	}
	return CreateRequestResult{Request: r}, true, nil
}

// enrich reads title/body/hints from the tracker outside any transaction so the
// request_counters row lock is never held across a network call.
func (uc *CreateRequest) enrich(ctx context.Context, in CreateRequestInput, src domain.SourceRef) (CreateRequestInput, error) {
	canFetch := (src.Provider == domain.SourceProviderJira || src.Provider == domain.SourceProviderLinear) && src.Ref != "" && uc.issues != nil
	needs := in.Title == "" || in.Body == "" || in.Hints.IsZero()
	if !canFetch || !needs {
		return in, nil
	}
	snap, err := uc.issues.GetIssue(ctx, src.Provider, src.Ref, src.Site)
	if err != nil {
		if in.Title != "" {
			slog.WarnContext(ctx, "request intake: source enrichment failed, using caller content", slog.String("provider", string(src.Provider)))
			return in, nil
		}
		if errors.Is(err, ErrIssueNotFound) {
			return in, domain.ErrRequestSourceNotFound(src.Provider, src.Ref)
		}
		return in, domain.ErrRequestSourceFetchFailed(src.Provider, err)
	}
	if in.Title == "" {
		in.Title = snap.Title
	}
	if in.Body == "" {
		in.Body = snap.Body
	}
	if in.Source.URL == "" {
		in.Source.URL = snap.URL
	}
	if in.Hints.IsZero() {
		in.Hints = snap.Hints
	}
	return in, nil
}

// CreateWithinTx is the creation core shared with SpawnChildRequest. The caller owns the
// transaction; src must already be normalised and title/body validated.
func (uc *CreateRequest) CreateWithinTx(ctx context.Context, in CreateRequestInput, src domain.SourceRef, tenantID, reporter string, key domain.IdempotencyKey, keyed bool) (domain.Request, error) {
	// Every entry path (manual, webhook, MCP, child) masks pasted credentials before anything is stored or published.
	title, body, suspected, _, err := SecretIngressGuard{}.Apply(in.Title, in.Body)
	if err != nil {
		return domain.Request{}, err
	}
	in.Title, in.Body = title, body
	hints := in.Hints
	if !in.AllowTypeHint {
		hints.TypeHint = ""
	}
	r, err := domain.NewRequest(domain.NewRequestInput{
		TenantID: tenantID, ProjectID: in.ProjectID, Title: in.Title, Body: in.Body,
		SourceProvider: string(src.Provider), SourceRef: src.Ref, SourceURL: firstNonEmpty(src.URL, in.Source.URL), SourceSite: src.Site,
		ReporterID: reporter, SourceHints: hints,
	})
	if err != nil {
		return domain.Request{}, err
	}
	if len(in.AcceptanceCriteria) > 0 || len(in.TypeFields) > 0 {
		c, err := domain.InitialContent(r.Title, r.Body, in.AcceptanceCriteria, in.TypeFields)
		if err != nil {
			return domain.Request{}, err
		}
		if vs := domain.ValidateRequestContent("", c, domain.ValidationLevelDraft); len(vs) > 0 {
			return domain.Request{}, domain.ErrArtifactSchemaInvalid(vs)
		}
		if r, err = r.WithContent(c); err != nil {
			return domain.Request{}, err
		}
	}
	if keyed {
		existingID, claimed, err := uc.idem.Claim(ctx, string(key.Provider), key.Site, key.Ref, r.ID)
		if err != nil {
			return domain.Request{}, err
		}
		if !claimed {
			return domain.Request{}, claimLost{existingID: existingID}
		}
	}
	if r.Number, err = uc.repo.NextNumber(ctx); err != nil {
		return domain.Request{}, err
	}
	if err := uc.repo.Create(ctx, r); err != nil {
		return domain.Request{}, err
	}
	if suspected && uc.flags != nil {
		if err := uc.flags.MarkSecretSuspected(ctx, r.ID); err != nil {
			return domain.Request{}, err
		}
	}
	if err := uc.recorder.RecordCreated(ctx, r); err != nil {
		return domain.Request{}, err
	}
	// created is written before the transition so outbox order is created, status_changed.
	payload := map[string]any{
		"request_id": r.ID, "project_id": r.ProjectID, "number": r.Number,
		"source_provider": r.SourceProvider, "source_site": r.SourceSite, "source_ref": r.SourceRef,
		"reporter_id": r.ReporterID, "title": r.Title,
	}
	if in.ParentRequestID != "" {
		payload["parent_request_id"], payload["link_reason"] = in.ParentRequestID, in.LinkReason
	}
	ev, err := NewOutboxEvent(ctx, domain.SubjectRequestCreated, payload)
	if err != nil {
		return domain.Request{}, err
	}
	if err := uc.outbox.InsertOutboxEvent(ctx, ev); err != nil {
		return domain.Request{}, err
	}
	kind := in.ActorKind
	if kind == "" {
		kind = domain.ActorKindUser
	}
	from := domain.RequestStatusNew
	res, err := uc.transition.Execute(ctx, TransitionInput{
		RequestID: r.ID, Trigger: domain.TriggerStartClassification, ExpectedFrom: &from, ActorID: reporter, ActorKind: kind,
	})
	if err != nil {
		return domain.Request{}, err
	}
	return res.Request, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

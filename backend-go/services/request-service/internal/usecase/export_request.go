package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/secretscan"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const (
	// ExportMaxBytes is the cap of one ExportRequest bundle; larger fails instead of being cut silently.
	ExportMaxBytes = 5 << 20
	// ExportChunkBytes is the cap of one NDJSON chunk on the tenant export stream.
	ExportChunkBytes = 64 << 10
	exportPageSize   = 100
	exportBodyPart   = 40 << 10
)

// ExportSources are the read ports an export gathers from.
type ExportSources struct {
	Requests  RequestRepository
	History   RequestTypeHistoryRepository
	Solutions SolutionCoreRepository
	Approvals ApprovalRepository
	Links     RequestLinkRepository
	Flags     SecurityFlagStore
}

type ExportBundle struct {
	JSON  string
	Bytes int64
}

type ExportRequest struct {
	src   ExportSources
	audit *AuditRecorder
	clock Clock
}

func NewExportRequest(src ExportSources, audit *AuditRecorder, clock Clock) *ExportRequest {
	if clock == nil {
		clock = systemClock{}
	}
	return &ExportRequest{src: src, audit: audit, clock: clock}
}

// Execute builds one redacted JSON bundle. The audit entry is queued before the data is handed out, so an
// export that cannot be audited does not happen.
func (uc *ExportRequest) Execute(ctx context.Context, requestID string) (ExportBundle, error) {
	if err := requireTenantAdmin(ctx); err != nil {
		return ExportBundle{}, err
	}
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ExportBundle{}, domain.ErrRequestTenantRequired()
	}
	req, err := uc.src.Requests.Get(ctx, requestID)
	if err != nil {
		return ExportBundle{}, err
	}
	flags, err := uc.src.Flags.Get(ctx, requestID)
	if err != nil {
		return ExportBundle{}, err
	}
	doc := map[string]any{"request": requestSkeleton(req), "exported_at": uc.clock.Now().UTC().Format(time.RFC3339), "patterns_version": secretscan.PatternsVersion}
	if flags.ErasedAt != nil {
		doc["erased"] = true
	} else if err := uc.addDetails(ctx, tenantID, req, doc); err != nil {
		return ExportBundle{}, err
	}
	out, err := redactedJSON(doc)
	if err != nil {
		return ExportBundle{}, err
	}
	if len(out) > ExportMaxBytes {
		return ExportBundle{}, domain.ErrExportTooLarge()
	}
	if err := uc.audit.RecordDurable(ctx, AuditEvent{
		Action: domain.AuditRequestExport, TargetType: "request", TargetID: requestID, Outcome: "allowed", RequestID: requestID,
		Metadata: map[string]any{"bytes": len(out)},
	}); err != nil {
		return ExportBundle{}, err
	}
	return ExportBundle{JSON: string(out), Bytes: int64(len(out))}, nil
}

func (uc *ExportRequest) addDetails(ctx context.Context, tenantID string, req domain.Request, doc map[string]any) error {
	r := doc["request"].(map[string]any)
	r["title"], r["body"], r["classification_reason"], r["return_reason"] = req.Title, req.Body, req.ClassificationReason, req.ReturnReason

	hist, err := uc.src.History.List(ctx, req.ID)
	if err != nil {
		return err
	}
	th := make([]map[string]any, 0, len(hist))
	for _, h := range hist {
		th = append(th, map[string]any{"from_type": string(h.FromType), "to_type": string(h.ToType), "actor_kind": string(h.ActorKind), "reason": h.Reason, "at": h.At.UTC().Format(time.RFC3339)})
	}
	doc["type_history"] = th

	sols, err := uc.src.Solutions.ListByRequestID(ctx, req.ID)
	if err != nil {
		return err
	}
	sj := make([]map[string]any, 0, len(sols))
	for _, s := range sols {
		var opts any
		if len(s.OptionsJSON) > 0 {
			_ = json.Unmarshal(s.OptionsJSON, &opts)
		}
		sj = append(sj, map[string]any{"id": s.ID, "kind": string(s.Kind), "status": string(s.Status), "options": opts, "chosen_option": s.ChosenOption})
	}
	doc["solutions"] = sj

	appr, _, err := uc.src.Approvals.List(ctx, tenantID, ApprovalListFilter{RequestID: req.ID, PageSize: 200})
	if err != nil {
		return err
	}
	aj := make([]map[string]any, 0, len(appr))
	for _, a := range appr {
		aj = append(aj, map[string]any{"id": a.ID, "subject_type": string(a.SubjectType), "status": string(a.Status), "comment": a.Comment})
	}
	doc["approvals"] = aj

	links := []map[string]any{}
	if parents, err := uc.src.Links.ListParents(ctx, req.ID); err != nil {
		return err
	} else {
		for _, l := range parents {
			links = append(links, map[string]any{"parent_request_id": l.ParentRequestID, "child_request_id": l.ChildRequestID, "reason": string(l.Reason)})
		}
	}
	if children, err := uc.src.Links.ListChildren(ctx, req.ID); err != nil {
		return err
	} else {
		for _, l := range children {
			links = append(links, map[string]any{"parent_request_id": l.ParentRequestID, "child_request_id": l.ChildRequestID, "reason": string(l.Reason)})
		}
	}
	doc["links"] = links
	return nil
}

func requestSkeleton(r domain.Request) map[string]any {
	return map[string]any{
		"id": r.ID, "number": r.Number, "project_id": r.ProjectID, "type": string(r.Type), "status": string(r.Status),
		"source_provider": string(r.SourceProvider), "created_at": r.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// redactedJSON masks secrets in every string of the document (decode, walk, encode) so masking can never
// break the JSON structure.
func redactedJSON(doc map[string]any) ([]byte, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // keeps chunk sizes close to the text size
	if err := enc.Encode(redactStrings(generic)); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func redactStrings(v any) any {
	switch x := v.(type) {
	case string:
		out, _ := secretscan.Redact(x)
		return out
	case []any:
		for i := range x {
			x[i] = redactStrings(x[i])
		}
		return x
	case map[string]any:
		for k, child := range x {
			x[k] = redactStrings(child)
		}
		return x
	}
	return v
}

func requireTenantAdmin(ctx context.Context) error {
	if role, _ := tenant.Role(ctx); role != "admin" || tenant.ActorType(ctx) == tenant.ActorAgent {
		return domain.ErrRequestForbidden()
	}
	return nil
}

// ExportTenantRequests streams every Request of the tenant (or one project) as NDJSON chunks of at most 64 KB.
type ExportTenantRequests struct {
	requests RequestRepository
	flags    SecurityFlagStore
	audit    *AuditRecorder

	mu     sync.Mutex
	active map[string]bool
}

func NewExportTenantRequests(requests RequestRepository, flags SecurityFlagStore, audit *AuditRecorder) *ExportTenantRequests {
	return &ExportTenantRequests{requests: requests, flags: flags, audit: audit, active: map[string]bool{}}
}

// Run sends chunks through send until done, a send error or ctx cancellation. One export runs per tenant at a time.
func (uc *ExportTenantRequests) Run(ctx context.Context, projectID string, send func(line string, index int64) error) (count int64, err error) {
	if err := requireTenantAdmin(ctx); err != nil {
		return 0, err
	}
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return 0, domain.ErrRequestTenantRequired()
	}
	if !uc.acquire(tenantID) {
		return 0, apperrors.New(apperrors.KindResourceExhausted, "REQUEST_EXPORT_IN_PROGRESS", "another tenant export is already running", nil)
	}
	defer uc.release(tenantID)

	var chunks int64
	// A cancelled client still leaves a trace of what was handed out.
	defer func() {
		_ = uc.audit.RecordDurable(context.WithoutCancel(ctx), AuditEvent{
			Action: domain.AuditRequestExport, TargetType: "tenant", TargetID: tenantID, Outcome: "allowed",
			Metadata: map[string]any{"requests": count, "chunks": chunks, "complete": err == nil},
		})
	}()

	token := ""
	for {
		if err := ctx.Err(); err != nil {
			return count, err
		}
		res, err := uc.requests.List(ctx, ListFilter{ProjectID: projectID, PageSize: exportPageSize, PageToken: token})
		if err != nil {
			return count, err
		}
		for _, r := range res.Requests {
			lines, err := uc.linesFor(ctx, r)
			if err != nil {
				return count, err
			}
			for _, l := range lines {
				if err := ctx.Err(); err != nil {
					return count, err
				}
				if err := send(l, chunks); err != nil {
					return count, err
				}
				chunks++
			}
			count++
		}
		if res.NextPageToken == "" {
			return count, nil
		}
		token = res.NextPageToken
	}
}

func (uc *ExportTenantRequests) acquire(tenantID string) bool {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if uc.active[tenantID] {
		return false
	}
	uc.active[tenantID] = true
	return true
}

func (uc *ExportTenantRequests) release(tenantID string) {
	uc.mu.Lock()
	delete(uc.active, tenantID)
	uc.mu.Unlock()
}

// linesFor renders a Request as one line, or as several when the body would not fit a 64 KB chunk.
func (uc *ExportTenantRequests) linesFor(ctx context.Context, r domain.Request) ([]string, error) {
	flags, err := uc.flags.Get(ctx, r.ID)
	if err != nil {
		return nil, err
	}
	doc := requestSkeleton(r)
	if flags.ErasedAt != nil {
		doc["erased"] = true
		return encodeLine(doc)
	}
	doc["title"] = r.Title
	// JSON escaping can inflate a part, so halve the part size until every line fits one chunk.
	for part := exportBodyPart; ; part /= 2 {
		lines, err := bodyLines(r, doc, part)
		if err != nil {
			return nil, err
		}
		if part <= 1024 || fitsChunk(lines) {
			return lines, nil
		}
	}
}

func fitsChunk(lines []string) bool {
	for _, l := range lines {
		if len(l) > ExportChunkBytes {
			return false
		}
	}
	return true
}

func bodyLines(r domain.Request, base map[string]any, partBytes int) ([]string, error) {
	doc := make(map[string]any, len(base)+3)
	for k, v := range base {
		doc[k] = v
	}
	parts := splitRunes(r.Body, partBytes)
	doc["body"] = ""
	if len(parts) > 0 {
		doc["body"] = parts[0]
	}
	if len(parts) > 1 {
		doc["part"], doc["parts"] = 0, len(parts)
	}
	lines, err := encodeLine(doc)
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(parts); i++ {
		more, err := encodeLine(map[string]any{"id": r.ID, "part": i, "parts": len(parts), "body_part": parts[i]})
		if err != nil {
			return nil, err
		}
		lines = append(lines, more...)
	}
	return lines, nil
}

func encodeLine(doc map[string]any) ([]string, error) {
	out, err := redactedJSON(doc)
	if err != nil {
		return nil, err
	}
	return []string{string(out) + "\n"}, nil
}

// splitRunes cuts s into pieces of at most maxBytes without splitting a UTF-8 sequence.
func splitRunes(s string, maxBytes int) []string {
	if s == "" {
		return nil
	}
	var out []string
	for len(s) > maxBytes {
		cut := maxBytes
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		out = append(out, s[:cut])
		s = s[cut:]
	}
	return append(out, s)
}

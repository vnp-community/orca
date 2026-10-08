// Package audit writes Request decisions to auth-service's audit log through common/auditclient.
package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/stablyai/orca-go/common/auditclient"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// recordTimeout bounds the synchronous append so a slow auth-service cannot stall the caller.
const recordTimeout = 2 * time.Second

// allowedMetadataKeys is the only metadata that may leave this service; anything else (title, body, prompt)
// is dropped here because the filter is the last place where a mistake upstream can still be stopped.
var allowedMetadataKeys = map[string]bool{
	"source_provider": true, "client_name": true, "type": true, "type_source": true, "from": true, "to": true,
	"returned_from_stage": true, "option": true, "subject_type": true, "stage": true, "enabled": true,
}

// Appender is the part of auditclient.Client this recorder needs.
type Appender interface {
	AppendDetailed(ctx context.Context, e auditclient.Entry)
}

// AuthAuditRecorder is best effort: AppendDetailed swallows RPC errors by design.
type AuthAuditRecorder struct {
	client Appender
}

func NewAuthAuditRecorder(c Appender) *AuthAuditRecorder { return &AuthAuditRecorder{client: c} }

var _ usecase.RPCAuditRecorder = (*AuthAuditRecorder)(nil)

func (r *AuthAuditRecorder) Record(ctx context.Context, e usecase.RPCAuditEvent) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()
	r.client.AppendDetailed(ctx, auditclient.Entry{
		TenantID:     e.TenantID,
		ActorID:      e.ActorID,
		ActorType:    string(e.ActorKind),
		Action:       e.Action,
		Target:       e.Target(),
		TargetType:   e.TargetType,
		TargetID:     e.TargetID,
		Outcome:      e.Outcome,
		MetadataJSON: filteredMetadata(e.Metadata),
	})
}

func filteredMetadata(in map[string]string) string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		if allowedMetadataKeys[k] && v != "" {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return ""
	}
	b, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(b)
}

package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// PublishApprovalNotifications enriches approval events just before the outbox relay publishes them:
// recipients, title, body and deep link are added so notification-service needs no directory knowledge.
// The event id is untouched, so notification-service dedupes redeliveries.
type PublishApprovalNotifications struct {
	Expander *ExpandApprovalRecipients
}

// ErrNotEnrichable marks a payload that can never be enriched (malformed JSON); the relay publishes it as is
// instead of blocking the stream behind it.
var ErrNotEnrichable = fmt.Errorf("approval event payload not enrichable")

func (uc *PublishApprovalNotifications) Handles(subject string) bool {
	return subject == domain.SubjectApprovalRequested || subject == domain.SubjectApprovalDecided
}

func str(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return v
}

// Enrich must run under the event's tenant (ctx). Only directory failures return an error (relay retries).
func (uc *PublishApprovalNotifications) Enrich(ctx context.Context, subject string, payload []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil || m == nil {
		return nil, ErrNotEnrichable
	}
	approvalID, requestID := str(m, "approval_id"), str(m, "request_id")
	label := domain.ApprovalSubjectLabel(domain.SubjectType(str(m, "subject_type")))
	ref := ""
	if n, ok := m["request_number"].(float64); ok && n > 0 {
		ref = fmt.Sprintf(" for REQ-%d", int64(n))
	}
	reporterID, requestedBy := str(m, "reporter_id"), str(m, "requested_by")

	switch subject {
	case domain.SubjectApprovalRequested:
		selfAllowed, _ := m["self_approval_allowed"].(bool)
		users, err := uc.Expander.Execute(ctx, approvalID, selfAllowed, reporterID, requestedBy)
		if err != nil {
			return nil, err
		}
		m["title"], m["body"] = "Approval needed", label+" approval required"+ref
		if str(m, "reason") == "reminder" {
			m["title"], m["body"] = "Approval reminder", label+" approval still pending"+ref
		}
		if len(users) > 0 {
			m["user_ids"] = users
		}
	case domain.SubjectApprovalDecided:
		decision, decidedBy := str(m, "decision"), str(m, "decided_by")
		users := make([]string, 0, 2)
		for _, u := range []string{reporterID, requestedBy} {
			if u != "" && u != systemActor && u != decidedBy && !contains(users, u) {
				users = append(users, u)
			}
		}
		m["title"], m["body"] = label+" "+decision, "Decision: "+decision
		if len(users) > 0 {
			m["user_ids"] = users
		}
	default:
		return payload, nil
	}
	m["deep_link"] = "/?section=requests&request=" + url.QueryEscape(requestID) + "&approval=" + url.QueryEscape(approvalID)
	delete(m, "comment") // defense in depth: a stored notification must never carry decision text
	return json.Marshal(m)
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

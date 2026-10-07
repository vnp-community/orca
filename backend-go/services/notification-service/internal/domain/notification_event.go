package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// DeliveryChannel is a way a NotificationEvent can reach a user — WS
// fan-out (through api-gateway) or mobile push (APNs/FCM), the two
// distinct delivery mechanisms notification-service.md keeps separate
// throughout (§1).
type DeliveryChannel string

const (
	ChannelDeliveryWS   DeliveryChannel = "ws"
	ChannelDeliveryPush DeliveryChannel = "push"
)

// Severity classifies how urgently a NotificationEvent should be surfaced.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// ErrNoRecipients is returned by TranslateEvent when the source event's
// payload names no user to notify — nothing to broadcast, per §2 (no
// offline WS replay queue): a notification nobody can receive is a no-op,
// not a delivery worth retrying.
var ErrNoRecipients = errors.New("domain: event payload names no recipient user")

// ErrInvalidCursor is returned by NotificationRepository.ListByRecipient
// when the caller-supplied cursor isn't the "<rfc3339nano>|<id>" shape this
// repository encodes — a malformed/tampered cursor is a client input error
// (usecase/ maps it to apperrors.KindInvalidArgument), not a panic.
var ErrInvalidCursor = errors.New("domain: invalid pagination cursor")

// EventPayload is the generic shape TranslateEvent decodes a consumed bus
// event's JSON payload into. Per §3, the subject list is illustrative, not
// exhaustive — a new publisher's payload only needs to carry these
// well-known fields to be translatable without a schema change here.
type EventPayload struct {
	UserID   string   `json:"user_id,omitempty"`
	UserIDs  []string `json:"user_ids,omitempty"`
	Title    string   `json:"title,omitempty"`
	Body     string   `json:"body,omitempty"`
	DeepLink string   `json:"deep_link,omitempty"`
	// ClientName is the MCP client that opened a terminal; only read by rules
	// that build their own text (see subjectRule.Locked).
	ClientName string `json:"client_name,omitempty"`
	// Reason and Origin are read only from infra-fleet's terminal.closed event.
	Reason string              `json:"reason,omitempty"`
	Origin *TerminalOriginInfo `json:"origin,omitempty"`
}

// TerminalOriginInfo is the origin block of orca.infrafleet.terminal.closed.
type TerminalOriginInfo struct {
	Type       string `json:"type"`
	ClientName string `json:"client_name"`
}

// SubjectInfraTerminalClosed is infra-fleet's lossless (outbox) close event;
// only idle closes of MCP-created terminals become a notification.
const SubjectInfraTerminalClosed = "orca.infrafleet.terminal.closed"

// ErrNotNotifiable means a well-formed event that intentionally produces no
// notification (e.g. a user-initiated terminal close): acked, never retried.
var ErrNotNotifiable = errors.New("domain: event does not produce a notification")

// DecodePayload unmarshals a bus event's raw JSON payload into an
// EventPayload. Kept in domain/ (encoding/json is stdlib, so this stays
// within the zero-framework-imports rule) so malformed-payload handling is
// unit-testable alongside TranslateEvent, and usecase/ doesn't need its own
// decode step.
func DecodePayload(raw []byte) (EventPayload, error) {
	if len(raw) == 0 {
		return EventPayload{}, nil
	}
	var p EventPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return EventPayload{}, err
	}
	return p, nil
}

// NotificationEvent is the internal representation after translating a
// domain event (task/workflow/automation/credential/orchestration) into
// something user-facing — see notification-service.md §4. TranslateEvent
// produces this; DeliverWS/DeliverPush (broadcaster/push adapters) each
// consume it independently.
type NotificationEvent struct {
	ID               string
	TenantID         string
	RecipientUserIDs []string
	SourceEventID    string
	SourceSubject    string
	Type             string
	Title            string
	Body             string
	DeepLink         string
	Severity         Severity
	Channels         []DeliveryChannel
	CreatedAt        time.Time
	// IsRead/ReadAt are read-model fields — TranslateEvent never sets them
	// (zero value: false/nil), so every newly translated notification is
	// unread by construction. Populated by NotificationRepository when
	// reading persisted rows back, and set to true (with ReadAt) by
	// MarkAsRead/MarkAllAsRead's read-receipt broadcast. See
	// specs/backend-go/crs/v4/notification/solutions/BE-NOTIF-SOL-001's
	// §1 for why this lives directly on NotificationEvent instead of a
	// separate wrapper struct.
	IsRead bool
	ReadAt *time.Time
}

// subjectRule is one row of §3's subject table: how a subject maps to a
// notification's type/title/body/severity/channels absent payload
// overrides.
type subjectRule struct {
	Type     string
	Title    string
	Body     string
	Severity Severity
	Channels []DeliveryChannel
	// DeepLink is the same-origin path used when the payload carries none.
	DeepLink string
	// Locked ignores payload title/body/deep_link: the text is built here from
	// whitelisted fields only, since notifications are stored and may be pushed.
	Locked bool
}

// subjectRules is illustrative, not exhaustive (§3) — a subject missing
// from this table still translates, via defaultRule, so a new publisher
// doesn't require a code change here to produce a (generic) notification.
var subjectRules = map[string]subjectRule{
	"orca.task.task.completed": {
		Type: "task_completed", Title: "Task completed", Body: "Your task has finished.",
		Severity: SeverityInfo, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
	},
	// Added SOL-PW-04 (TASK-PW-04-08). Fires on EVERY status transition
	// (open->in_progress, etc.), not just completion — an "open ->
	// in_progress" toast on every single task dispatch would be noise the
	// .completed subject above doesn't have, so this is deliberately
	// WS-only, low-severity: available to any future in-app UI without
	// becoming a push notification.
	"orca.task.task.statuschanged": {
		Type: "task_status_changed", Title: "Task updated", Body: "",
		Severity: SeverityInfo, Channels: []DeliveryChannel{ChannelDeliveryWS},
	},
	"orca.workflow.execution.completed": {
		Type: "workflow_completed", Title: "Workflow finished", Body: "Your workflow execution has completed.",
		Severity: SeverityInfo, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
	},
	"orca.workflow.execution.failed": {
		Type: "workflow_failed", Title: "Workflow failed", Body: "Your workflow execution has failed.",
		Severity: SeverityWarning, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
	},
	"orca.automation.run.completed": {
		Type: "automation_run_completed", Title: "Automation run finished", Body: "Your automation run has completed.",
		Severity: SeverityInfo, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
	},
	// BE-MCP-SOL-013: an AI agent waits for the user's approval. Type follows
	// CONTRACT section 4 ("mcp.approval"); the producer's body carries only
	// client/tool/risk, never arguments, since notifications are stored.
	"orca.mcp.approval.requested": {
		Type: "mcp.approval", Title: "Approval needed", Body: "An AI agent is waiting for your approval.",
		Severity: SeverityWarning, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
	},
	// BE-REQ-SOL-010: request-service approvals
	"orca.request.approval.requested": {
		Type: "request.approval_requested", Title: "Approval needed", Body: "Approval required.",
		Severity: SeverityWarning, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
		DeepLink: "/?section=requests",
	},
	"orca.request.approval.decided": {
		Type: "request.approval_decided", Title: "Approval decided", Body: "Approval decided.",
		Severity: SeverityInfo, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
	},
	// A terminal an AI client opened was stopped for being idle. Locked: the
	// body names the client at most, never command text or output.
	"orca.mcp.terminal.idlestopped": {
		Type: MCPTerminalIdleStoppedType, Title: "Terminal stopped",
		Body:     "A terminal opened by an AI agent was stopped after being idle.",
		Severity: SeverityInfo, Channels: []DeliveryChannel{ChannelDeliveryWS},
		DeepLink: "/?section=mcp&tab=connect", Locked: true,
	},
	"orca.credential.credential.rotated": {
		// "Always delivered regardless of preferences" per §2 — this
		// scaffold has no preference filter at all yet, so "always" is
		// trivially true today, not an enforced override.
		Type: "credential_rotated", Title: "Security alert: credential rotated", Body: "One of your credentials was rotated.",
		Severity: SeverityCritical, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
	},
	"orca.orchestration.decision_gate.opened": {
		Type: "decision_gate_opened", Title: "Needs your decision", Body: "A workflow is waiting on your decision.",
		Severity: SeverityWarning, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
	},
	"orca.project.devserver.changed": {
		Type: "project_devserver_changed", Title: "Dev server changed",
		Body:     "This project's dev server binding was changed.",
		Severity: SeverityWarning, Channels: []DeliveryChannel{ChannelDeliveryWS},
	},
	// BL-MB-02 (SOL-MB-02): infra-fleet-service's PTY output-quiescence
	// tracking and ai-provider-service's connection-test relay parsing.
	"orca.infra.terminal_session.agent_completed": {
		Type: "agent_completed", Title: "✅ Agent xong", Body: "{agent} đã hoàn thành task.",
		Severity: SeverityInfo, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
	},
	"orca.infra.terminal_session.agent_error": {
		Type: "agent_error", Title: "❌ Agent lỗi",
		Severity: SeverityWarning, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
	},
	"orca.infra.terminal_session.agent_waiting": {
		Type: "agent_waiting", Title: "⏸ Agent chờ input",
		Severity: SeverityInfo, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
	},
	"orca.aiprovider.account.rate_limited": {
		Type: "rate_limited", Title: "⚠️ Rate limit",
		Severity: SeverityWarning, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
	},
	// starNag.subscribe's cross-replica visibility push (TASK-014, SOL-005)
	// — WS-only (no push notification for a UI-prompt-visibility toggle),
	// empty Title/Body: payload.Body always carries the real
	// {event,mode,surface} JSON triple (tenant-service's
	// starNagVisibilityBody), which the payload.Body != "" override below
	// passes through unchanged — no schema change needed here, per
	// notification-service.md §3's "a new subject can be added without a
	// schema change" design.
	"orca.tenant.star_nag.visibility_changed": {
		Type: "star_nag_visibility", Title: "", Body: "",
		Severity: SeverityInfo, Channels: []DeliveryChannel{ChannelDeliveryWS},
	},
}

// MCPTerminalIdleStoppedType is the NotificationEvent.Type of an idle-stop notice.
const MCPTerminalIdleStoppedType = "mcp.terminal.idle_stopped"

// idleStoppedBodyWithClient builds the body from a sanitized client name.
func idleStoppedBodyWithClient(name string) string {
	var b []rune
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			continue
		}
		if b = append(b, r); len(b) == 64 {
			break
		}
	}
	if n := strings.TrimSpace(string(b)); n != "" {
		return "A terminal opened by " + n + " was stopped after being idle."
	}
	return ""
}

// defaultRule is used for any subject not in subjectRules — WS-only,
// informational, so an unrecognized publisher's event degrades safely
// instead of being silently dropped.
var defaultRule = subjectRule{
	Type: "generic", Title: "Notification", Body: "",
	Severity: SeverityInfo, Channels: []DeliveryChannel{ChannelDeliveryWS},
}

// TranslateEvent maps one consumed bus event into a NotificationEvent —
// pure and unit-testable without touching NATS/Postgres, per
// architecture/03's domain-layer-has-zero-framework-imports rule. id is the
// NotificationEvent's own identity (generated by the caller, e.g. via
// uuid.NewString() in usecase/); sourceEventID/subject/tenantID/occurredAt
// come from the consumed bus envelope (common/eventbus.Event).
func TranslateEvent(id, sourceEventID, subject, tenantID string, payload EventPayload, occurredAt time.Time) (NotificationEvent, error) {
	ruleSubject := subject
	if subject == SubjectInfraTerminalClosed {
		if payload.Reason != "idle" || payload.Origin == nil || payload.Origin.Type != "mcp" {
			return NotificationEvent{}, ErrNotNotifiable
		}
		ruleSubject = "orca.mcp.terminal.idlestopped" // reuse the locked, sanitised idle-stop rule
		if payload.ClientName == "" {
			payload.ClientName = payload.Origin.ClientName
		}
	}

	recipients := recipientsOf(payload)
	if len(recipients) == 0 {
		return NotificationEvent{}, ErrNoRecipients
	}

	rule, ok := subjectRules[ruleSubject]
	if !ok {
		rule = defaultRule
	}

	title := rule.Title
	if payload.Title != "" {
		title = payload.Title
	}
	body := rule.Body
	if payload.Body != "" {
		body = payload.Body
	}

	deepLink := payload.DeepLink
	if rule.Locked {
		title, body, deepLink = rule.Title, rule.Body, rule.DeepLink
		if b := idleStoppedBodyWithClient(payload.ClientName); b != "" {
			body = b
		}
	} else if deepLink == "" {
		deepLink = rule.DeepLink
	}

	return NotificationEvent{
		ID:               id,
		TenantID:         tenantID,
		RecipientUserIDs: recipients,
		SourceEventID:    sourceEventID,
		SourceSubject:    subject,
		Type:             rule.Type,
		Title:            title,
		Body:             body,
		DeepLink:         deepLink,
		Severity:         rule.Severity,
		Channels:         rule.Channels,
		CreatedAt:        occurredAt,
	}, nil
}

// BufferedNotification is one pending buffered push notification (BR-MB-07)
// — decoded back into a NotificationEvent so callers (StreamNotifications'
// reconnect drain) can reuse the same wire-framing a live event uses.
type BufferedNotification struct {
	ID    string
	Event NotificationEvent
}

func recipientsOf(p EventPayload) []string {
	if len(p.UserIDs) > 0 {
		return p.UserIDs
	}
	if p.UserID != "" {
		return []string{p.UserID}
	}
	return nil
}

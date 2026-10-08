package domain

import "strings"

// Rules for request-service subjects (orca.request.*) live apart from the core
// table so each request feature (approval, clarification, ...) adds its rows
// here without touching notification_event.go.
//
// Payload contract (producer: request-service outbox, see
// specs/backend-go/crs/v6/approval/IMPLEMENTATION-NOTES.md): non-empty
// user_ids, plus title, body (short, never the comment/question/answer text,
// since notifications are stored) and an in-app deep_link. Rules are not
// Locked so the producer's concrete title wins; the strings below are fallbacks.
const (
	TypeRequestApprovalRequested      = "request.approval_requested"
	TypeRequestApprovalDecided        = "request.approval_decided"
	TypeRequestClarificationRequested = "request.clarification_requested"
	TypeRequestClarificationExpired   = "request.clarification_expired"

	requestDeepLink = "/?section=requests"
)

var bothChannels = []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush}

var requestSubjectRules = map[string]subjectRule{
	// BE-REQ-SOL-010 (CR-REQ-010).
	"orca.request.approval.requested": {
		Type: TypeRequestApprovalRequested, Title: "Approval needed", Body: "Approval required.",
		Severity: SeverityWarning, Channels: bothChannels, DeepLink: requestDeepLink, SameOriginDeepLink: true,
	},
	"orca.request.approval.decided": {
		Type: TypeRequestApprovalDecided, Title: "Approval decided", Body: "Approval decided.",
		Severity: SeverityInfo, Channels: bothChannels, DeepLink: requestDeepLink, SameOriginDeepLink: true,
	},
	// BE-REQ-SOL-028 (CR-REQ-028): also carries reminders (payload reason="reminder").
	"orca.request.clarification.requested": {
		Type: TypeRequestClarificationRequested, Title: "Request cần bổ sung thông tin", Body: "Có yêu cầu bổ sung thông tin.",
		Severity: SeverityWarning, Channels: bothChannels, DeepLink: requestDeepLink, SameOriginDeepLink: true,
	},
	"orca.request.clarification.expired": {
		Type: TypeRequestClarificationExpired, Title: "Yêu cầu bổ sung thông tin đã hết hạn", Body: "Yêu cầu bổ sung thông tin đã hết hạn.",
		Severity: SeverityWarning, Channels: []DeliveryChannel{ChannelDeliveryWS}, DeepLink: requestDeepLink, SameOriginDeepLink: true,
	},
}

func init() {
	for subject, rule := range requestSubjectRules {
		subjectRules[subject] = rule
	}
}

// isInAppPath accepts "/x" but not "//host" or "/\host", which browsers treat as external.
func isInAppPath(link string) bool {
	return strings.HasPrefix(link, "/") && !strings.HasPrefix(link, "//") && !strings.HasPrefix(link, `/\`)
}

package domain

import "strings"

// MCPApprovalType is the NotificationEvent.Type of an MCP approval request
// (BE-MCP-SOL-013).
const MCPApprovalType = "mcp.approval"

// mcpApprovalPushBody is the only body an mcp.approval push may carry.
const mcpApprovalPushBody = "An AI agent is waiting for your approval."

// PushMessage is the Web Push payload the frontend service worker reads
// (frontend/src/renderer/public/service-worker.js): exactly these four keys.
type PushMessage struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	DeepLink string `json:"deepLink"`
	Tag      string `json:"tag"`
}

// ToPushMessage builds the browser-visible payload for e. Why: push bodies
// transit third-party push services, so an mcp.approval body is pinned to a
// fixed string — tool arguments must never leave the platform this way.
func ToPushMessage(e NotificationEvent) PushMessage {
	body := e.Body
	if e.Type == MCPApprovalType {
		body = mcpApprovalPushBody
	}
	return PushMessage{Title: e.Title, Body: body, DeepLink: sameOriginPath(e.DeepLink), Tag: e.Type + ":" + e.ID}
}

// sameOriginPath keeps only app-relative paths; the service worker applies
// the same rule, this just avoids shipping a foreign URL at all.
func sameOriginPath(raw string) string {
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "/\\") {
		return ""
	}
	return raw
}

// PushDelivery is the RFC 8030 delivery hint for an event: TTL in seconds
// and Urgency (very-low|low|normal|high).
type PushDelivery struct {
	TTLSeconds int
	Urgency    string
}

// PushDeliveryFor picks TTL/urgency. Why: an approval request is useless once
// the approval times out, so it must not be queued for hours; warnings and
// critical alerts wake a sleeping device, informational ones don't.
func PushDeliveryFor(e NotificationEvent) PushDelivery {
	if e.Type == MCPApprovalType {
		return PushDelivery{TTLSeconds: 600, Urgency: "high"}
	}
	switch e.Severity {
	case SeverityCritical, SeverityWarning:
		return PushDelivery{TTLSeconds: 86400, Urgency: "high"}
	default:
		return PushDelivery{TTLSeconds: 86400, Urgency: "normal"}
	}
}

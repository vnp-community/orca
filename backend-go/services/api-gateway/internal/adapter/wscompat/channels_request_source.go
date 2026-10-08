package wscompat

import (
	"context"
	"errors"
	"strings"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// RequestSourceInput is the client-declared source of a new Request.
type RequestSourceInput struct {
	Provider string `json:"provider"`
	Ref      string `json:"ref"`
	URL      string `json:"url"`
	Site     string `json:"site"`
}

// unknownMCPClient keeps source_site non-empty for display; it is not part of any idempotency key.
const unknownMCPClient = "unknown-mcp-client"

var errRequestSourceForbidden = errors.New("REQUEST_SOURCE_FORBIDDEN: source provider not allowed from this client")

// ResolveRequestSource applies CONTRACT section 6.1. A non-nil origin means the
// call came through MCP, set by the executor from the verified session, so the
// provider is mcp whatever the input says. Clients may only claim an external
// tracker; mcp, webhook and manual are assigned by the gateway or the service.
func ResolveRequestSource(origin *ToolOrigin, in RequestSourceInput) (*requestv1.RequestSource, error) {
	if origin != nil {
		site := strings.TrimSpace(origin.ClientName)
		if site == "" {
			site = unknownMCPClient
		}
		return &requestv1.RequestSource{Provider: "mcp", Site: site}, nil
	}
	switch strings.ToLower(strings.TrimSpace(in.Provider)) {
	case "":
		return &requestv1.RequestSource{Provider: "manual"}, nil
	case "jira", "github", "gitlab", "linear":
		return &requestv1.RequestSource{Provider: strings.ToLower(strings.TrimSpace(in.Provider)), Ref: in.Ref, Url: in.URL, Site: in.Site}, nil
	default:
		return nil, errRequestSourceForbidden
	}
}

func resolveRequestSource(ctx context.Context, in RequestSourceInput) (*requestv1.RequestSource, error) {
	if o, ok := toolOriginFromContext(ctx); ok {
		return ResolveRequestSource(&o, in)
	}
	return ResolveRequestSource(nil, in)
}

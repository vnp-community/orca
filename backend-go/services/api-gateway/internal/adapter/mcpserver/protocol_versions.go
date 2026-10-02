package mcpserver

// SupportedProtocolVersions is the ONLY place that lists MCP spec revisions
// this server speaks (newest last). Bumping the spec means changing this and
// re-running the conformance suite; the SDK is narrowed to exactly this list
// so it can never negotiate a revision we have not validated.
var SupportedProtocolVersions = []string{"2025-06-18"}

// LatestProtocolVersion returns the newest supported revision.
func LatestProtocolVersion() string {
	return SupportedProtocolVersions[len(SupportedProtocolVersions)-1]
}

// Negotiate returns the client's requested revision if supported, otherwise
// the newest one we support (MCP lifecycle: server answers with its own).
func Negotiate(requested string) string {
	for _, v := range SupportedProtocolVersions {
		if v == requested {
			return v
		}
	}
	return LatestProtocolVersion()
}

// sdkProtocolVersions is the SDK's expected order (newest first).
func sdkProtocolVersions() []string {
	out := make([]string, len(SupportedProtocolVersions))
	for i, v := range SupportedProtocolVersions {
		out[len(out)-1-i] = v
	}
	return out
}

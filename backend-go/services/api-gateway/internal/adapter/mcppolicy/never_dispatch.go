package mcppolicy

import "strings"

// NeverDispatchPrefixes are channel namespaces the tool executor must refuse
// to dispatch even if a gate wrongly allowed them (last-resort fuse). They
// MUST equal data.orca.authz.mcp.hard_deny_prefixes; a test enforces it.
var NeverDispatchPrefixes = []string{"credentials.", "auth.", "mcp."}

// NeverDispatch reports whether channel is in a never-dispatch namespace.
func NeverDispatch(channel string) bool {
	for _, p := range NeverDispatchPrefixes {
		if strings.HasPrefix(channel, p) {
			return true
		}
	}
	return false
}

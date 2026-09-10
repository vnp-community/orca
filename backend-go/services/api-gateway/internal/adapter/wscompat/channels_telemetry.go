// ── telemetry.* ──────────────────────────────────────────────────────────
//
// The old TS backend's telemetry.track (backend/src/main/runtime/rpc/methods/
// telemetry.ts) forwarded consent-gated product-analytics events to PostHog
// (backend/src/main/telemetry/client.ts) — a real external analytics
// integration, not local state. Porting that (consent resolution, cohort
// enrichment, PostHog client, burst-rate cap) is a distinct analytics-
// integration task, out of scope here.
//
// frontend/src/renderer/src/lib/telemetry.ts's track() is genuinely
// fire-and-forget — every call site already swallows a failed/missing
// telemetry.track into a console.warn and never blocks on it (see that
// file's track() doc comment). Accepting the call and no-op'ing is the
// honest interim answer: it silences that harmless-but-noisy warning
// without pretending events reach any analytics backend.
package wscompat

import (
	"context"
	"encoding/json"
)

func registerTelemetryChannels(r *Registry) {
	r.Register("telemetry.track", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		// Args ({name, props}) are intentionally not decoded — there is
		// nowhere for this event to go yet, so there is nothing to validate
		// against. See this file's package doc comment.
		return nil, nil
	})
}

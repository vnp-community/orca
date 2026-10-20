package schemas

import "embed"

// OutcomeReview holds the contract of the context run of an Outcome Review (CR-OR-002). It is kept apart from V1: those are the
// documents a Request stores, this one is only what an agent is asked to return.
//
//go:embed outcome-review/*.schema.json
var OutcomeReview embed.FS

// Why: shared by the buffered (agent.execPrompt) and streaming (agent.execPromptStream)
// handlers so both keep identical timeout clamping.
export const DEFAULT_TIMEOUT_MS = 5 * 60_000
// Raised from 15 minutes: authoring the OpenSpec change of a size L request took 13 to 25 minutes in the pilot, and a run cut at the ceiling is lost whole.
export const MAX_TIMEOUT_MS = 30 * 60_000
export const MIN_TIMEOUT_MS = 1_000

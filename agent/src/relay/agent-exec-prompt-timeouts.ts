// Why: shared by the buffered (agent.execPrompt) and streaming (agent.execPromptStream)
// handlers so both keep identical timeout clamping.
export const DEFAULT_TIMEOUT_MS = 5 * 60_000
export const MAX_TIMEOUT_MS = 15 * 60_000
export const MIN_TIMEOUT_MS = 1_000

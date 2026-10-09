// Structured failures of ai.complete. Kept apart from the handler so the API providers and the CLI-session
// provider can both throw them without importing each other.

export type AICompleteErrorReason =
  | 'PROVIDER_HTTP_ERROR'
  | 'PROVIDER_TIMEOUT'
  | 'PROVIDER_NETWORK'
  | 'NO_API_KEY'
  | 'UNKNOWN_MODEL_PROVIDER'
  // Claude Code CLI session source (the dev server's own login):
  | 'CLI_UNAVAILABLE'
  | 'CLI_NOT_LOGGED_IN'
  | 'CLI_SESSION_EXPIRED'
  | 'CLI_QUOTA_EXHAUSTED'
  | 'CLI_BUSY'
  | 'CLI_FAILED'

export type AICompleteErrorData = {
  provider: string
  httpStatus: number | null
  retryable: boolean
  reason: AICompleteErrorReason
}

/** Structured error thrown by provider call functions; caught by dispatchAiRpc and sent as JSON-RPC error.data. */
export class AICompleteProviderError extends Error {
  constructor(
    message: string,
    public readonly errorData: AICompleteErrorData
  ) {
    super(message)
    this.name = 'AICompleteProviderError'
  }
}

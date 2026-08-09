/**
 * AI Provider Types — v5.0 (TDD-16)
 *
 * Shared types for AI provider account management.
 * Credentials are NEVER stored on Orca Server — only on Dev Server via relay.
 *
 * @module shared/ai-provider-types
 */

/** Supported AI provider backends */
export type AIProviderType =
  | 'anthropic'
  | 'openai'
  | 'gemini'
  | 'azure'
  | 'bedrock'
  | 'ollama'
  | 'vllm'

/**
 * Scope at which the provider account applies:
 * - server: all users on that dev server
 * - project: users of a specific project
 * - user: a single user's personal account
 */
export type AIProviderScope = 'server' | 'project' | 'user'

/** Health / quota status of a provider account */
export type AIProviderStatus =
  | 'pending'        // newly registered, not yet tested
  | 'active'         // health check passed
  | 'invalid'        // credentials rejected
  | 'quota_exceeded' // daily quota hit
  | 'unreachable'    // network / relay error

/** A registered AI provider account (no credential stored here) */
export type AIProviderAccount = {
  id: string
  /** Which dev server holds the encrypted credential */
  devServerId: string
  provider: AIProviderType
  scope: AIProviderScope
  /** projectId (scope='project') or userId (scope='user'); null for scope='server' */
  scopeRefId?: string
  /** Human-readable label, e.g. "Team Claude 3.5" */
  label: string
  /** Optional default model override */
  model?: string
  /** Optional base URL override (Ollama, vLLM, Azure) */
  baseUrl?: string
  status: AIProviderStatus
  lastHealthCheck?: Date
  /** Daily token quota (0 = unlimited) */
  quotaLimitDay: number
  /** Tokens used today — populated on demand from orca_provider_usage */
  quotaUsedToday?: number
  createdBy: string
  createdAt: Date
  updatedAt: Date
}

/** Payload sent to relay when writing an encrypted credential */
export type CredentialWriteRequest = {
  accountId: string
  /** AES-256-GCM encrypted API key blob (base64) */
  encryptedBlob: string
  /** AES-256-GCM IV (base64) */
  iv: string
}

/** Today's aggregated usage for a provider account */
export type ProviderUsageToday = {
  tokens: number
  requests: number
  costUsd: number
}

/**
 * Env var names that contain credentials for each provider.
 * Used by relay handler to locate the key from environment.
 */
export const PROVIDER_ENV_KEYS: Readonly<Record<AIProviderType, readonly string[]>> = {
  anthropic: ['ANTHROPIC_API_KEY'],
  openai: ['OPENAI_API_KEY'],
  gemini: ['GEMINI_API_KEY', 'GOOGLE_API_KEY'],
  azure: ['AZURE_OPENAI_API_KEY', 'AZURE_OPENAI_ENDPOINT'],
  bedrock: ['AWS_ACCESS_KEY_ID', 'AWS_SECRET_ACCESS_KEY', 'AWS_DEFAULT_REGION'],
  ollama: ['OLLAMA_BASE_URL'],
  vllm: ['VLLM_BASE_URL', 'VLLM_API_KEY'],
} as const

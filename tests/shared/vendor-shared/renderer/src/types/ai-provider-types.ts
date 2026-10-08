// ai-provider-types.ts — Shared types for AI Provider (TDD-FE-13)

export type AIProviderType =
  | 'anthropic'
  | 'openai'
  | 'gemini'
  | 'azure'
  | 'bedrock'
  | 'ollama'
  | 'vllm'

export type AIProviderScope = 'server' | 'project' | 'user'

export type AIProviderStatus =
  | 'active'
  | 'pending'
  | 'invalid'
  | 'quota_exceeded'
  | 'unreachable'

export type AIProviderAccount = {
  id:            string
  provider:      AIProviderType
  label:         string
  model:         string
  baseUrl?:      string         // Ollama / vLLM
  scope:         AIProviderScope
  scopeRefId:    string
  devServerId:   string
  status:        AIProviderStatus
  quotaLimitDay: number         // 0 = unlimited
  createdAt:     number
}

export type AIProviderUsage = {
  accountId: string
  tokens:    number
  requests:  number
  date:      string             // YYYY-MM-DD
}

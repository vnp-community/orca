/**
 * ai-provider-types-shared.ts — Re-export từ src/shared/ (Conflict C6 resolution)
 *
 * Cho components MỚI import từ đây để lấy types chính xác từ shared source-of-truth.
 * Components CŨ vẫn dùng ai-provider-types.ts (không đụng file đó).
 *
 * Pattern:
 *   // Component cũ (không đổi):
 *   import { AIProviderType } from '../types/ai-provider-types'
 *
 *   // Component mới (dùng file này):
 *   import { AIProviderType } from '../types/ai-provider-types-shared'
 *
 * @module renderer/types/ai-provider-types-shared
 */

// Re-export tất cả từ shared source of truth:
export type {
  AIProviderType,
  AIProviderAccount,
  AIProviderStatus,
  AIProviderScope,
} from '../../../../shared/ai-provider-types'

// Re-export shared credential contract:
export type {
  CredentialReadResult,
  HealthCheckResult,
  CredentialWriteParams,
  HealthCheckParams,
} from '../../../../shared/ai-credential-contract'

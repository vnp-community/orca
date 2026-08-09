/**
 * ai-credential-contract.ts — Shared interface contract for credential operations
 *
 * Đồng bộ API shape giữa:
 *   - src/relay/agent-credential-store.ts  (Agent tier — Dev Server)
 *   - src/relay/ai-provider-handler.ts     (Backend tier — Orca Server)
 *
 * Hai files trên implement RIÊNG (2-tier intentional) nhưng cần
 * cùng return shape để Frontend không bị type mismatch.
 *
 * @module shared/ai-credential-contract
 */

/**
 * Kết quả đọc credential từ bất kỳ tier nào.
 */
export type CredentialReadResult = {
  encryptedBlob: string
  iv:            string
  updatedAt:     string
}

/**
 * Kết quả kiểm tra sức khỏe AI provider connection.
 * Cả agent-credential-store.ts và ai-provider-handler.ts phải trả về shape này.
 */
export type HealthCheckResult = {
  ok:        boolean
  latencyMs: number
  error?:    string
}

/**
 * Params để write/update credential.
 */
export type CredentialWriteParams = {
  accountId:     string
  encryptedBlob: string
  iv:            string
}

/**
 * Params để thực hiện health check.
 */
export type HealthCheckParams = {
  accountId: string
  /** Timeout tính bằng ms. Default: 5000 */
  timeoutMs?: number
}

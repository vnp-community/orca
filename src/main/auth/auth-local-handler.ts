/**
 * AuthLocalHandler — Local email+password login handler
 *
 * Validates credentials, creates session on success.
 * Performs input validation BEFORE touching the database.
 *
 * @module main/auth/auth-local-handler
 */

import type { AuthUserStore } from './auth-user-store'
import type { AuthSessionStore } from './auth-session-store'
import type { LocalLoginInput, LocalLoginResult } from './auth-types'

const EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export class AuthLocalHandler {
  constructor(
    private readonly userStore:    AuthUserStore,
    private readonly sessionStore: AuthSessionStore
  ) {}

  /**
   * Attempt local login.
   * 1. Validate input format (no DB query if invalid)
   * 2. Verify credentials with bcrypt
   * 3. Create session on success
   */
  async login(
    input:     LocalLoginInput,
    ipAddress: string,
    userAgent: string
  ): Promise<LocalLoginResult> {
    // Step 1: Validate input format before touching DB
    if (!input.email || !input.password) {
      return { success: false, error: 'validation_error', detail: 'email and password are required' }
    }
    if (!EMAIL_REGEX.test(input.email)) {
      return { success: false, error: 'validation_error', detail: 'invalid email format' }
    }

    // Step 2: Verify credentials
    const user = await this.userStore.verifyPassword(input.email, input.password)
    if (!user) {
      return { success: false, error: 'invalid_credentials' }
    }

    // Step 3: Create session
    const session = await this.sessionStore.createSession({
      userId:    user.id,
      userEmail: user.email,
      role:      user.role,
      ipAddress,
      userAgent
    })

    return { success: true, sessionId: session.sessionId, user }
  }
}

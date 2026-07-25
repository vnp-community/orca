/**
 * Admin User Handlers — CRUD operations for user management
 *
 * Handles: list users, create user, deactivate user (+ revoke sessions).
 * All routes require requireAdmin middleware (applied via admin-router).
 *
 * @module main/admin/admin-user-handlers
 */

import type { Request, Response } from 'express'
import type { AuthUserStore } from '../auth/auth-user-store'
import type { AuthSessionStore } from '../auth/auth-session-store'
import type { AuditLogger } from './audit-logger'
import { AUDIT_ACTIONS } from './admin-types'

const VALID_ROLES = ['developer', 'lead', 'admin'] as const

export class AdminUserHandlers {
  constructor(private readonly deps: {
    userStore:    AuthUserStore
    sessionStore: AuthSessionStore
    auditLogger:  AuditLogger
  }) {}

  listUsers = async (_req: Request, res: Response): Promise<void> => {
    const users = this.deps.userStore.listUsers()
    res.json({ users, total: users.length })
  }

  createUser = async (req: Request, res: Response): Promise<void> => {
    const { email, name, password, role } = (req.body ?? {}) as Record<string, string>

    if (!email || !name || !password || !role) {
      res.status(400).json({ error: 'missing_fields', required: ['email', 'name', 'password', 'role'] })
      return
    }
    if (!(VALID_ROLES as readonly string[]).includes(role)) {
      res.status(400).json({ error: 'invalid_role', allowed: VALID_ROLES })
      return
    }

    try {
      const user = await this.deps.userStore.createLocalUser({
        email,
        name,
        password,
        role: role as typeof VALID_ROLES[number]
      })

      this.deps.auditLogger.log({
        userId:    req.orcaSession!.userId,
        userEmail: req.orcaSession!.userEmail,
        action:    AUDIT_ACTIONS.USER_CREATE,
        ipAddress: req.ip,
        detail:    { targetEmail: email, role }
      })

      res.status(201).json(user)
    } catch (err: unknown) {
      // Duplicate email or other DB error
      const message = err instanceof Error ? err.message : 'Unknown error'
      if (message.includes('UNIQUE') || message.toLowerCase().includes('duplicate')) {
        res.status(409).json({ error: 'email_taken', message: 'Email is already in use' })
      } else {
        res.status(500).json({ error: 'internal_error', message })
      }
    }
  }

  deactivateUser = async (req: Request, res: Response): Promise<void> => {
    const { id } = req.params

    // Prevent self-deactivation
    if (id === req.orcaSession!.userId) {
      res.status(400).json({ error: 'cannot_deactivate_self' })
      return
    }

    this.deps.userStore.deactivateUser(id!)
    const revokedCount = this.deps.sessionStore.revokeAllUserSessions(id!)

    this.deps.auditLogger.log({
      userId:    req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action:    AUDIT_ACTIONS.USER_DEACTIVATE,
      ipAddress: req.ip,
      detail:    { targetUserId: id, revokedSessions: revokedCount }
    })

    res.json({ ok: true, revokedSessions: revokedCount })
  }
}

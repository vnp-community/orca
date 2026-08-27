/**
 * Auth Subsystem — Core Types
 *
 * Types cho authentication, session management, và user operations trong server mode.
 * Dùng `OrcaUser` từ shared/rbac-types để không duplicate type definitions.
 *
 * @module main/auth/auth-types
 */

import type { OrcaUser } from '../../shared/rbac-types'

/** Subset of OrcaUser fields returned after authentication (no sensitive data) */
export type OrcaSessionUser = Pick<
  OrcaUser,
  'id' | 'email' | 'name' | 'role' | 'provider' | 'departmentId'
>

/** An authenticated HTTP session stored in orca_sessions table */
export type OrcaSession = {
  /** 64-hex string (32 random bytes) — stored as HttpOnly cookie */
  sessionId:   string
  userId:      string
  userEmail:   string
  role:        OrcaUser['role']
  createdAt:   number   // Unix ms
  expiresAt:   number   // createdAt + SESSION_TTL_MS
  lastSeenAt:  number | null
  ipAddress:   string | null
  userAgent:   string | null
}

/** Input for creating a new session */
export type CreateSessionInput = {
  userId:    string
  userEmail: string
  role:      OrcaUser['role']
  ipAddress: string
  userAgent: string
}

/** Input for creating a local (email + password) user */
export type LocalUserInput = {
  email:    string
  name:     string
  password: string
  role:     OrcaUser['role']
}

/** Input for upserting a user via SSO / OAuth2 */
export type SsoUserInput = {
  email:          string
  name:           string
  provider:       'github' | 'google' | 'keycloak'
  providerUserId: string
  avatarUrl?:     string
}

/** Local login credentials */
export type LocalLoginInput = {
  email:    string
  password: string
}

/** Result of a local login attempt */
export type LocalLoginResult =
  | { success: true;  sessionId: string; user: OrcaSessionUser }
  | { success: false; error: 'invalid_credentials' | 'account_disabled' | 'validation_error'; detail?: string }

/** Session TTL — 8 hours in milliseconds */
export const SESSION_TTL_MS = 8 * 60 * 60 * 1000

/**
 * First-Run Setup — Seeds initial admin user on first server boot
 *
 * Called during server-bootstrap initialization.
 * Idempotent: does nothing if an active admin already exists.
 *
 * @module main/admin/first-run-setup
 */

import { randomBytes } from 'node:crypto'
import type { IDatabase } from '../db/types'
import type { AuthUserStore } from '../auth/auth-user-store'

/**
 * Ensure at least one admin user exists.
 * Only creates the admin user if NO active admin users are present (first run).
 * Credentials sourced from env vars or auto-generated.
 *
 * Env vars:
 *   ORCA_ADMIN_EMAIL    (default: admin@localhost)
 *   ORCA_ADMIN_PASSWORD (default: random 16-char hex — printed to stdout)
 */
export async function ensureFirstAdminUser(
  _db: IDatabase,
  userStore: AuthUserStore
): Promise<void> {
  const adminCount = await userStore.countAdmins()  // async — must await
  if (adminCount > 0) return  // Admin already exists — skip


  const adminEmail    = process.env['ORCA_ADMIN_EMAIL']    ?? 'admin@localhost'
  const adminPassword = process.env['ORCA_ADMIN_PASSWORD'] ?? randomBytes(8).toString('hex')
  const isRandom      = !process.env['ORCA_ADMIN_PASSWORD']

  await userStore.createLocalUser({
    email:    adminEmail,
    name:     'Administrator',
    password: adminPassword,
    role:     'admin'
  })

  console.log('')
  console.log('══════════════════════════════════════════════════════')
  console.log('  ⚠️   FIRST RUN: Admin account created')
  console.log(`       Email:    ${adminEmail}`)
  console.log(`       Password: ${adminPassword}${isRandom ? ' (auto-generated)' : ''}`)
  console.log('')
  if (isRandom) {
    console.log('  ▶  Change the password immediately after first login!')
    console.log('  ▶  Or set ORCA_ADMIN_EMAIL / ORCA_ADMIN_PASSWORD env vars')
    console.log('     before starting the server to use custom credentials.')
  }
  console.log('══════════════════════════════════════════════════════')
  console.log('')
}

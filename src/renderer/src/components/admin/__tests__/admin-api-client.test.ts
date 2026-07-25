// @vitest-environment happy-dom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  fetchAdminStats,
  fetchAdminUsers,
  createAdminUser,
  killAdminSession,
  fetchAdminAudit
} from '../admin-api-client'

describe('admin-api-client', () => {
  beforeEach(() => { global.fetch = vi.fn() })
  afterEach(() => { vi.restoreAllMocks() })

  // ── fetchAdminStats ────────────────────────────────────────────────────────

  it('fetchAdminStats returns stats on 200', async () => {
    const mockStats = { totalUsers: 5, activeSessions: 2, sshConnections: 1, pairedDevices: 3 }
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(mockStats), {
        status: 200,
        headers: { 'Content-Type': 'application/json' }
      })
    )
    const stats = await fetchAdminStats()
    expect(stats).toEqual(mockStats)
    expect(fetch).toHaveBeenCalledWith(
      '/admin/api/stats',
      expect.objectContaining({ credentials: 'include' })
    )
  })

  it('fetchAdminStats throws on 403', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response('Forbidden', { status: 403 }))
    await expect(fetchAdminStats()).rejects.toThrow('Forbidden')
  })

  // ── fetchAdminUsers ────────────────────────────────────────────────────────

  it('fetchAdminUsers calls /admin/api/users with credentials', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify([]), {
        status: 200,
        headers: { 'Content-Type': 'application/json' }
      })
    )
    await fetchAdminUsers()
    expect(fetch).toHaveBeenCalledWith(
      '/admin/api/users',
      expect.objectContaining({ credentials: 'include' })
    )
  })

  // ── createAdminUser ────────────────────────────────────────────────────────

  it('createAdminUser calls POST with serialized body', async () => {
    const newUser = { email: 'bob@co.com', name: 'Bob', role: 'developer' as const, provider: 'none' as const, isActive: true }
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ id: 'u2', ...newUser, lastLoginAt: null }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' }
      })
    )
    await createAdminUser(newUser)
    expect(fetch).toHaveBeenCalledWith(
      '/admin/api/users',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify(newUser)
      })
    )
  })

  // ── killAdminSession ───────────────────────────────────────────────────────

  it('killAdminSession calls DELETE /admin/api/sessions/:id', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(null, { status: 204 }))
    await killAdminSession('sess-abc')
    expect(fetch).toHaveBeenCalledWith(
      '/admin/api/sessions/sess-abc',
      expect.objectContaining({ method: 'DELETE', credentials: 'include' })
    )
  })

  // ── fetchAdminAudit ────────────────────────────────────────────────────────

  it('fetchAdminAudit passes filter params in query string', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify([]), {
        status: 200,
        headers: { 'Content-Type': 'application/json' }
      })
    )
    await fetchAdminAudit({ action: 'login.success', from: 1000, to: 2000 })
    const url = vi.mocked(fetch).mock.calls[0][0] as string
    expect(url).toContain('action=login.success')
    expect(url).toContain('from=1000')
    expect(url).toContain('to=2000')
  })

  // ── 401 handling ──────────────────────────────────────────────────────────

  it('throws Unauthorized on 401', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response('', { status: 401 }))
    await expect(fetchAdminStats()).rejects.toThrow('Unauthorized')
  })
})

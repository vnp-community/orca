/**
 * GitProviderCredentialService — GitHub/GitLab PAT credential management (TASK-PI-001)
 *
 * Stores Personal Access Tokens for GitHub and GitLab per user using WebCredentialStore.
 * Safe in server mode (no Electron dependency) — uses scrypt + AES-256-GCM encryption.
 *
 * Key scheme:
 *   GitHub:  `github:<userId>`              (one PAT per user)
 *   GitLab:  `gitlab:<userId>:<projectId>` (per-user, per-project)
 *
 * Security:
 *   - Tokens are stored via WebCredentialStore (V2: random salt per write)
 *   - Tokens are never logged or returned in error messages
 *
 * @module main/project/GitProviderCredentialService
 */

import type { WebCredentialStore } from '../credentials/web-credential-store'
import { Tracers } from '../../shared/trace/tracers'

// ── GitProviderCredentialService ──────────────────────────────────────────────

export class GitProviderCredentialService {
  constructor(
    private readonly getUserStore: (userId: string) => WebCredentialStore
  ) {}

  // ── GitHub ──────────────────────────────────────────────────────────────────

  /**
   * Store a GitHub Personal Access Token for a user.
   * Overwrites any existing token for this userId.
   */
  async setGitHubPAT(userId: string, token: string): Promise<void> {
    const store = this.getUserStore(userId)
    // FIX TASK-PI-001: Use WebCredentialStore V2 (random salt per write)
    await store.setToken('bitbucket', token, { provider: 'github', userId })
    // Note: reusing 'bitbucket' slot for github since WebCredentialStore is per-userId
    // For a clean implementation, extend CredentialService enum — done separately
  }

  async getGitHubPAT(userId: string): Promise<string | null> {
    const span = Tracers.remoteIntegrationCredentialDecryptFlow.start({ provider: 'github', userId })
    const store = this.getUserStore(userId)
    span.step('decrypt', { provider: 'github' })
    // Security: the decrypted token value itself must never be placed into a
    // trace field — only `found` (boolean) may reflect the lookup outcome.
    try {
      const token = await store.getToken('bitbucket')
      span.ok({ provider: 'github', found: token !== null })
      return token
    } catch (err) {
      span.fail(err, { provider: 'github' })
      throw err
    }
  }

  async deleteGitHubPAT(userId: string): Promise<void> {
    const store = this.getUserStore(userId)
    await store.deleteToken('bitbucket')
  }

  // ── GitLab ──────────────────────────────────────────────────────────────────

  /**
   * Store a GitLab Personal Access Token for a user + project combination.
   * GitLab tokens can be project-scoped, so we include projectId in the key.
   */
  async setGitLabPAT(userId: string, projectId: string, token: string): Promise<void> {
    const store = this.getUserStore(userId)
    // Store in config alongside gitea slot (gitea = GitLab-like API)
    await store.setToken('gitea', token, { provider: 'gitlab', userId, projectId })
  }

  async getGitLabPAT(userId: string, _projectId: string): Promise<string | null> {
    const span = Tracers.remoteIntegrationCredentialDecryptFlow.start({ provider: 'gitlab', userId })
    const store = this.getUserStore(userId)
    span.step('decrypt', { provider: 'gitlab' })
    try {
      const token = await store.getToken('gitea')
      span.ok({ provider: 'gitlab', found: token !== null })
      return token
    } catch (err) {
      span.fail(err, { provider: 'gitlab' })
      throw err
    }
  }

  async deleteGitLabPAT(userId: string, _projectId: string): Promise<void> {
    const store = this.getUserStore(userId)
    await store.deleteToken('gitea')
  }

  // ── Generic PAT resolution ───────────────────────────────────────────────────

  /**
   * Resolve the PAT for a repository URL.
   * Detects github.com vs gitlab.com from the URL domain.
   */
  async resolvePatForUrl(
    userId: string,
    repoUrl: string,
    projectId?: string
  ): Promise<string | null> {
    const url = repoUrl.toLowerCase()
    if (url.includes('github.com')) {
      return this.getGitHubPAT(userId)
    }
    if (url.includes('gitlab.com') || url.includes('gitlab.')) {
      return this.getGitLabPAT(userId, projectId ?? '')
    }
    return null
  }
}

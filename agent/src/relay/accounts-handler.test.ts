// agent/src/relay/accounts-handler.test.ts
// TASK-023: accounts.selectClaude/selectCodex/removeClaude/removeCodex.
import { describe, expect, it, beforeEach, afterEach } from 'vitest'
import { mkdtemp, mkdir, writeFile, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import {
  HOST_ACCOUNT_ID,
  handleAccountsRemoveClaude,
  handleAccountsRemoveCodex,
  handleAccountsSelectClaude,
  handleAccountsSelectCodex,
  listClaudeAccounts,
  listCodexAccounts,
  removeClaudeAccount,
  removeCodexAccount,
  selectClaudeAccount,
  selectCodexAccount,
  type AccountsHandlerPaths,
} from './accounts-handler'

let home: string
let paths: AccountsHandlerPaths

function b64url(json: unknown): string {
  return Buffer.from(JSON.stringify(json))
    .toString('base64')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '')
}

function fakeJwt(payload: Record<string, unknown>): string {
  return `${b64url({ alg: 'none' })}.${b64url(payload)}.sig`
}

beforeEach(async () => {
  home = await mkdtemp(join(tmpdir(), 'orca-accounts-handler-'))
  paths = {
    claudeDir: join(home, '.claude'),
    claudeConfigFile: join(home, '.claude.json'),
    codexDir: join(home, '.codex'),
  }
})

afterEach(async () => {
  await rm(home, { recursive: true, force: true })
})

describe('Claude — empty state', () => {
  it('reports no accounts when ~/.claude/.credentials.json does not exist', async () => {
    const snapshot = await listClaudeAccounts(paths)
    expect(snapshot).toEqual({
      accounts: [],
      activeAccountId: null,
      activeAccountIdsByRuntime: { host: null, wsl: {} },
    })
  })

  it('selectClaudeAccount(null) is a no-op success against an empty state', async () => {
    const snapshot = await selectClaudeAccount(null, paths)
    expect(snapshot.activeAccountId).toBeNull()
  })

  it('selectClaudeAccount(id) throws when no account exists', async () => {
    await expect(selectClaudeAccount(HOST_ACCOUNT_ID, paths)).rejects.toThrow(
      'That Claude account no longer exists.'
    )
  })

  it('removeClaudeAccount throws for a nonexistent account instead of silently no-op-ing', async () => {
    await expect(removeClaudeAccount(HOST_ACCOUNT_ID, paths)).rejects.toThrow(
      'That Claude account no longer exists.'
    )
  })
})

describe('Claude — one host account present', () => {
  beforeEach(async () => {
    await mkdir(paths.claudeDir, { recursive: true })
    await writeFile(join(paths.claudeDir, '.credentials.json'), JSON.stringify({ claudeAiOauth: {} }))
    await writeFile(
      paths.claudeConfigFile,
      JSON.stringify({ oauthAccount: { emailAddress: 'dev@example.com' } })
    )
  })

  it('lists the single host account with the resolved email', async () => {
    const snapshot = await listClaudeAccounts(paths)
    expect(snapshot.accounts).toHaveLength(1)
    expect(snapshot.accounts[0]).toMatchObject({
      id: HOST_ACCOUNT_ID,
      email: 'dev@example.com',
      authMethod: 'subscription-oauth',
    })
    expect(snapshot.activeAccountId).toBe(HOST_ACCOUNT_ID)
  })

  it('falls back to a labeled placeholder email when ~/.claude.json has none', async () => {
    await rm(paths.claudeConfigFile, { force: true })
    const snapshot = await listClaudeAccounts(paths)
    expect(snapshot.accounts[0]?.email).toBe('Claude account (email unavailable)')
  })

  it('selectClaudeAccount(hostId) confirms the account is active', async () => {
    const snapshot = await selectClaudeAccount(HOST_ACCOUNT_ID, paths)
    expect(snapshot.activeAccountId).toBe(HOST_ACCOUNT_ID)
  })

  it('selectClaudeAccount(null) deselects without deleting the credentials file', async () => {
    const snapshot = await selectClaudeAccount(null, paths)
    expect(snapshot.activeAccountId).toBeNull()
    // The underlying account still exists — only the active pointer cleared.
    expect((await listClaudeAccounts(paths)).accounts).toHaveLength(1)
  })

  it('selectClaudeAccount(unknownId) throws', async () => {
    await expect(selectClaudeAccount('not-a-real-id', paths)).rejects.toThrow(
      'That Claude account no longer exists.'
    )
  })

  it('removeClaudeAccount deletes the credentials file and returns an empty snapshot', async () => {
    const snapshot = await removeClaudeAccount(HOST_ACCOUNT_ID, paths)
    expect(snapshot).toEqual({
      accounts: [],
      activeAccountId: null,
      activeAccountIdsByRuntime: { host: null, wsl: {} },
    })
    expect((await listClaudeAccounts(paths)).accounts).toHaveLength(0)
  })
})

describe('Codex — empty state', () => {
  it('reports no accounts when ~/.codex/auth.json does not exist', async () => {
    const snapshot = await listCodexAccounts(paths)
    expect(snapshot.accounts).toHaveLength(0)
    expect(snapshot.activeAccountId).toBeNull()
  })

  it('removeCodexAccount throws for a nonexistent account', async () => {
    await expect(removeCodexAccount(HOST_ACCOUNT_ID, paths)).rejects.toThrow(
      'That Codex rate limit account no longer exists.'
    )
  })
})

describe('Codex — one host account present', () => {
  beforeEach(async () => {
    await mkdir(paths.codexDir, { recursive: true })
    const idToken = fakeJwt({
      email: 'codex-user@example.com',
      'https://api.openai.com/auth': {
        chatgpt_account_id: 'acct-123',
        workspace_name: 'Acme Workspace',
        workspace_account_id: 'ws-acct-123',
      },
    })
    await writeFile(
      join(paths.codexDir, 'auth.json'),
      JSON.stringify({ tokens: { id_token: idToken, account_id: 'acct-123' } })
    )
  })

  it('lists the single host account with identity derived from the id_token', async () => {
    const snapshot = await listCodexAccounts(paths)
    expect(snapshot.accounts).toHaveLength(1)
    expect(snapshot.accounts[0]).toMatchObject({
      id: HOST_ACCOUNT_ID,
      email: 'codex-user@example.com',
      providerAccountId: 'acct-123',
      workspaceLabel: 'Acme Workspace',
      workspaceAccountId: 'ws-acct-123',
    })
    expect(snapshot.activeAccountId).toBe(HOST_ACCOUNT_ID)
  })

  it('treats an API-key auth.json as having no derivable identity, but still lists a placeholder account', async () => {
    await writeFile(join(paths.codexDir, 'auth.json'), JSON.stringify({ OPENAI_API_KEY: 'sk-test' }))
    const snapshot = await listCodexAccounts(paths)
    expect(snapshot.accounts[0]?.email).toBe('Codex account (email unavailable)')
    expect(snapshot.accounts[0]?.providerAccountId).toBeNull()
  })

  it('selectCodexAccount(hostId) confirms the account is active', async () => {
    const snapshot = await selectCodexAccount(HOST_ACCOUNT_ID, paths)
    expect(snapshot.activeAccountId).toBe(HOST_ACCOUNT_ID)
  })

  it('selectCodexAccount(null) deselects without deleting auth.json', async () => {
    const snapshot = await selectCodexAccount(null, paths)
    expect(snapshot.activeAccountId).toBeNull()
    expect((await listCodexAccounts(paths)).accounts).toHaveLength(1)
  })

  it('removeCodexAccount deletes auth.json and returns an empty snapshot', async () => {
    const snapshot = await removeCodexAccount(HOST_ACCOUNT_ID, paths)
    expect(snapshot.accounts).toHaveLength(0)
    expect((await listCodexAccounts(paths)).accounts).toHaveLength(0)
  })
})

describe('JSON-RPC dispatch handlers', () => {
  it('handleAccountsSelectClaude rejects a missing accountId field with InvalidParams', async () => {
    const resp = (await handleAccountsSelectClaude(1, {})) as { error?: { code: number } }
    expect(resp.error?.code).toBe(-32602)
  })

  it('handleAccountsRemoveClaude rejects a non-string accountId with InvalidParams', async () => {
    const resp = (await handleAccountsRemoveClaude(1, { accountId: 42 })) as {
      error?: { code: number }
    }
    expect(resp.error?.code).toBe(-32602)
  })

  it('handleAccountsSelectCodex accepts accountId: null and resolves ok', async () => {
    const resp = (await handleAccountsSelectCodex(1, { accountId: null })) as {
      result?: { activeAccountId: string | null }
    }
    expect(resp.result?.activeAccountId).toBeNull()
  })

  it('handleAccountsRemoveCodex surfaces the not-found error as a JSON-RPC error, not a throw', async () => {
    const resp = (await handleAccountsRemoveCodex(1, { accountId: 'ghost' })) as {
      error?: { message: string }
    }
    expect(resp.error?.message).toBe('That Codex rate limit account no longer exists.')
  })
})

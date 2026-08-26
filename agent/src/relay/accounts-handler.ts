// agent/src/relay/accounts-handler.ts
// TASK-023 (specs/backend-go/bugs/missing-v1/tasks/TASK-023-document-accounts-agent-gap.md):
// implements accounts.selectClaude/selectCodex/removeClaude/removeCodex as
// real JSON-RPC methods on the Dev Server Agent's dispatcher
// (agent-rpc-dispatch.ts) — the surface backend-go's infra-fleet-service
// `Relay` RPC and api-gateway's wscompat channels_accounts.go actually reach.
//
// Why this is NEW capability, not a port: the desktop app's
// ClaudeAccountService/CodexAccountService (backend/src/main/claude-accounts,
// codex-accounts) manage MULTIPLE named accounts in an Electron-userData
// "managed accounts" store, each with its own captured OAuth credentials, and
// support adding a new account via an interactive `claude auth login`/
// `codex login` browser flow. None of that infrastructure exists on a bare
// remote Dev Server host, and SOL-004 scopes this work to exactly 4
// non-interactive methods (select/remove — no add/reauthenticate, which need
// a desktop browser per rpc/methods/accounts.ts's own comment). A remote
// host realistically has at most ONE already-authenticated CLI identity per
// provider: whatever `claude`/`codex` the host owner already logged into
// directly. This module models that identity as a single pseudo-account with
// the fixed id 'host' (mirroring the existing host/wsl selection-bucket
// vocabulary in ClaudeManagedAccountRuntimeSelection/
// CodexManagedAccountRuntimeSelection — a remote agent host has no WSL
// concept, so 'host' is the only bucket that ever applies), derived by
// reading the CLI's own well-known config files — no CLI subprocess spawn,
// no keychain access (Linux/remote hosts have no Electron Keychain).
import { promises as fs } from 'node:fs'
import { homedir } from 'node:os'
import { join } from 'node:path'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

// ─── Types (mirrors frontend/src/shared/types.ts's shapes verbatim — these
// responses are forwarded to the frontend unmodified through infra-fleet-
// service's Relay and wscompat's channels_accounts.go) ──────────────────────

type ClaudeManagedAccountSummary = {
  id: string
  email: string
  managedAuthRuntime?: 'host' | 'wsl'
  wslDistro?: string | null
  authMethod: 'subscription-oauth' | 'unknown'
  organizationUuid?: string | null
  organizationName?: string | null
  createdAt: number
  updatedAt: number
  lastAuthenticatedAt: number
}

type ClaudeRateLimitAccountsState = {
  accounts: ClaudeManagedAccountSummary[]
  activeAccountId: string | null
  activeAccountIdsByRuntime?: { host: string | null; wsl: Record<string, string | null> }
}

type CodexManagedAccountSummary = {
  id: string
  email: string
  managedHomeRuntime?: 'host' | 'wsl'
  wslDistro?: string | null
  providerAccountId?: string | null
  workspaceLabel?: string | null
  workspaceAccountId?: string | null
  createdAt: number
  updatedAt: number
  lastAuthenticatedAt: number
}

type CodexRateLimitAccountsState = {
  accounts: CodexManagedAccountSummary[]
  activeAccountId: string | null
  activeAccountIdsByRuntime?: { host: string | null; wsl: Record<string, string | null> }
}

export type AccountsHandlerPaths = {
  /** ~/.claude/.credentials.json's directory */
  claudeDir: string
  /** ~/.claude.json — top-level Claude CLI config, holds oauthAccount.emailAddress after login */
  claudeConfigFile: string
  /** ~/.codex/auth.json's directory */
  codexDir: string
}

// Why: a remote host has exactly one CLI-authenticated identity per
// provider today (see module doc comment) — a fixed id avoids inventing a
// per-login UUID scheme with no second account to ever disambiguate against.
export const HOST_ACCOUNT_ID = 'host'

export function defaultAccountsHandlerPaths(home: string = homedir()): AccountsHandlerPaths {
  return {
    claudeDir: join(home, '.claude'),
    claudeConfigFile: join(home, '.claude.json'),
    codexDir: join(home, '.codex'),
  }
}

function emptyClaudeState(): ClaudeRateLimitAccountsState {
  return { accounts: [], activeAccountId: null, activeAccountIdsByRuntime: { host: null, wsl: {} } }
}

function emptyCodexState(): CodexRateLimitAccountsState {
  return { accounts: [], activeAccountId: null, activeAccountIdsByRuntime: { host: null, wsl: {} } }
}

async function statOrNull(path: string): Promise<{ mtimeMs: number } | null> {
  try {
    return await fs.stat(path)
  } catch {
    return null
  }
}

async function readJsonOrNull(path: string): Promise<Record<string, unknown> | null> {
  try {
    const raw = await fs.readFile(path, 'utf-8')
    const parsed = JSON.parse(raw) as unknown
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : null
  } catch {
    return null
  }
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null
}

function readString(value: Record<string, unknown> | null, key: string): string | null {
  const field = value?.[key]
  return typeof field === 'string' && field.trim() ? field.trim() : null
}

// ─── Claude ─────────────────────────────────────────────────────────────────

async function readClaudeEmail(paths: AccountsHandlerPaths): Promise<string | null> {
  const config = await readJsonOrNull(paths.claudeConfigFile)
  const oauthAccount = asRecord(config?.['oauthAccount'])
  return readString(oauthAccount, 'emailAddress') ?? readString(oauthAccount, 'email')
}

export async function listClaudeAccounts(
  paths: AccountsHandlerPaths = defaultAccountsHandlerPaths()
): Promise<ClaudeRateLimitAccountsState> {
  const credentialsPath = join(paths.claudeDir, '.credentials.json')
  const stat = await statOrNull(credentialsPath)
  if (!stat) {
    return emptyClaudeState()
  }
  // Why: the credentials file rarely carries the account's email itself —
  // it lives in ~/.claude.json's oauthAccount, written once by `claude
  // auth login`. Fall back to a clearly-labeled placeholder (never null —
  // ClaudeManagedAccountSummary.email is required) rather than failing the
  // whole snapshot just because identity resolution came up empty.
  const email = (await readClaudeEmail(paths)) ?? 'Claude account (email unavailable)'
  const timestamp = Math.round(stat.mtimeMs) || Date.now()
  const account: ClaudeManagedAccountSummary = {
    id: HOST_ACCOUNT_ID,
    email,
    managedAuthRuntime: 'host',
    wslDistro: null,
    authMethod: 'subscription-oauth',
    organizationUuid: null,
    organizationName: null,
    createdAt: timestamp,
    updatedAt: timestamp,
    lastAuthenticatedAt: timestamp,
  }
  return {
    accounts: [account],
    activeAccountId: HOST_ACCOUNT_ID,
    activeAccountIdsByRuntime: { host: HOST_ACCOUNT_ID, wsl: {} },
  }
}

export async function selectClaudeAccount(
  accountId: string | null,
  paths: AccountsHandlerPaths = defaultAccountsHandlerPaths()
): Promise<ClaudeRateLimitAccountsState> {
  const snapshot = await listClaudeAccounts(paths)
  if (accountId === null) {
    // Why: mirrors ClaudeAccountService.doSelectAccount(null) — deselecting is
    // always valid, even with zero or one known accounts.
    return { ...snapshot, activeAccountId: null, activeAccountIdsByRuntime: { host: null, wsl: {} } }
  }
  if (!snapshot.accounts.some((account) => account.id === accountId)) {
    throw new Error('That Claude account no longer exists.')
  }
  return snapshot
}

export async function removeClaudeAccount(
  accountId: string,
  paths: AccountsHandlerPaths = defaultAccountsHandlerPaths()
): Promise<ClaudeRateLimitAccountsState> {
  const snapshot = await listClaudeAccounts(paths)
  if (!snapshot.accounts.some((account) => account.id === accountId)) {
    // Why: mirrors ClaudeAccountService.requireAccount's real behavior —
    // removing an unknown id throws, it does not silently no-op.
    throw new Error('That Claude account no longer exists.')
  }
  await fs.rm(join(paths.claudeDir, '.credentials.json'), { force: true })
  return emptyClaudeState()
}

// ─── Codex ──────────────────────────────────────────────────────────────────

function parseJwtPayload(token: string): Record<string, unknown> | null {
  const parts = token.split('.')
  if (parts.length < 2) {
    return null
  }
  let payload = parts[1].replace(/-/g, '+').replace(/_/g, '/')
  while (payload.length % 4 !== 0) {
    payload += '='
  }
  try {
    return JSON.parse(Buffer.from(payload, 'base64').toString('utf-8')) as Record<string, unknown>
  } catch {
    return null
  }
}

type CodexIdentity = {
  email: string | null
  providerAccountId: string | null
  workspaceLabel: string | null
  workspaceAccountId: string | null
}

async function readCodexIdentity(paths: AccountsHandlerPaths): Promise<CodexIdentity> {
  const auth = await readJsonOrNull(join(paths.codexDir, 'auth.json'))
  const empty: CodexIdentity = {
    email: null,
    providerAccountId: null,
    workspaceLabel: null,
    workspaceAccountId: null,
  }
  if (!auth) {
    return empty
  }
  // Why: API-key-based auth.json has no OAuth/JWT identity to read — same
  // early-return CodexAccountService.loadOAuthCredentials uses.
  const apiKey = auth['OPENAI_API_KEY']
  if (typeof apiKey === 'string' && apiKey.trim()) {
    return empty
  }
  const tokens = asRecord(auth['tokens'])
  const idToken = readString(tokens, 'id_token') ?? readString(tokens, 'idToken')
  const accountIdFromTokens = readString(tokens, 'account_id') ?? readString(tokens, 'accountId')
  const payload = idToken ? parseJwtPayload(idToken) : null
  const authClaims = asRecord(payload?.['https://api.openai.com/auth'])
  const profileClaims = asRecord(payload?.['https://api.openai.com/profile'])
  return {
    email: readString(payload, 'email') ?? readString(profileClaims, 'email'),
    providerAccountId:
      accountIdFromTokens ??
      readString(authClaims, 'chatgpt_account_id') ??
      readString(payload, 'chatgpt_account_id'),
    workspaceLabel:
      readString(authClaims, 'workspace_name') ?? readString(profileClaims, 'workspace_name'),
    workspaceAccountId: readString(authClaims, 'workspace_account_id') ?? accountIdFromTokens,
  }
}

export async function listCodexAccounts(
  paths: AccountsHandlerPaths = defaultAccountsHandlerPaths()
): Promise<CodexRateLimitAccountsState> {
  const authPath = join(paths.codexDir, 'auth.json')
  const stat = await statOrNull(authPath)
  if (!stat) {
    return emptyCodexState()
  }
  const identity = await readCodexIdentity(paths)
  const timestamp = Math.round(stat.mtimeMs) || Date.now()
  const account: CodexManagedAccountSummary = {
    id: HOST_ACCOUNT_ID,
    email: identity.email ?? 'Codex account (email unavailable)',
    managedHomeRuntime: 'host',
    wslDistro: null,
    providerAccountId: identity.providerAccountId,
    workspaceLabel: identity.workspaceLabel,
    workspaceAccountId: identity.workspaceAccountId,
    createdAt: timestamp,
    updatedAt: timestamp,
    lastAuthenticatedAt: timestamp,
  }
  return {
    accounts: [account],
    activeAccountId: HOST_ACCOUNT_ID,
    activeAccountIdsByRuntime: { host: HOST_ACCOUNT_ID, wsl: {} },
  }
}

export async function selectCodexAccount(
  accountId: string | null,
  paths: AccountsHandlerPaths = defaultAccountsHandlerPaths()
): Promise<CodexRateLimitAccountsState> {
  const snapshot = await listCodexAccounts(paths)
  if (accountId === null) {
    return { ...snapshot, activeAccountId: null, activeAccountIdsByRuntime: { host: null, wsl: {} } }
  }
  if (!snapshot.accounts.some((account) => account.id === accountId)) {
    throw new Error('That Codex rate limit account no longer exists.')
  }
  return snapshot
}

export async function removeCodexAccount(
  accountId: string,
  paths: AccountsHandlerPaths = defaultAccountsHandlerPaths()
): Promise<CodexRateLimitAccountsState> {
  const snapshot = await listCodexAccounts(paths)
  if (!snapshot.accounts.some((account) => account.id === accountId)) {
    throw new Error('That Codex rate limit account no longer exists.')
  }
  await fs.rm(join(paths.codexDir, 'auth.json'), { force: true })
  return emptyCodexState()
}

// ─── JSON-RPC handlers (agent-rpc-dispatch.ts registration surface) ─────────

const INVALID = Symbol('invalid-params')

function parseSelectAccountId(params: Record<string, unknown>): string | null | typeof INVALID {
  if (!('accountId' in params)) {
    return INVALID
  }
  const { accountId } = params
  if (accountId === null) {
    return null
  }
  return typeof accountId === 'string' && accountId.trim() ? accountId : INVALID
}

function parseRemoveAccountId(params: Record<string, unknown>): string | typeof INVALID {
  const { accountId } = params
  return typeof accountId === 'string' && accountId.trim() ? accountId : INVALID
}

export async function handleAccountsSelectClaude(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const accountId = parseSelectAccountId(params)
  if (accountId === INVALID) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'accounts.selectClaude: accountId is required' } }
  }
  try {
    const result = await selectClaudeAccount(accountId)
    return { jsonrpc: '2.0', id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

export async function handleAccountsSelectCodex(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const accountId = parseSelectAccountId(params)
  if (accountId === INVALID) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'accounts.selectCodex: accountId is required' } }
  }
  try {
    const result = await selectCodexAccount(accountId)
    return { jsonrpc: '2.0', id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

export async function handleAccountsRemoveClaude(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const accountId = parseRemoveAccountId(params)
  if (accountId === INVALID) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'accounts.removeClaude: accountId is required' } }
  }
  try {
    const result = await removeClaudeAccount(accountId)
    return { jsonrpc: '2.0', id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

export async function handleAccountsRemoveCodex(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const accountId = parseRemoveAccountId(params)
  if (accountId === INVALID) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'accounts.removeCodex: accountId is required' } }
  }
  try {
    const result = await removeCodexAccount(accountId)
    return { jsonrpc: '2.0', id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

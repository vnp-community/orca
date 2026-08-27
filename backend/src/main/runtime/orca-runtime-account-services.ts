// frontend/src/main/runtime/orca-runtime-account-services.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-056): account/rate-limit RPC bridge
// commands extracted from OrcaRuntimeService via the composition pattern.
// Field-span analysis (TASK-BIGFILE-054) confirmed `accountServices` and
// these 9 methods are fully self-contained.
import type { ClaudeRateLimitAccountsState, CodexRateLimitAccountsState } from '../../shared/types'
import type { AccountsSnapshot } from './orca-runtime-types'
import type { RuntimeAccountServices } from './orca-runtime'
import type { RateLimitService } from '../rate-limits/service'

// Why: every other extracted domain in this file takes a host object for its
// cross-domain dependencies, but this one has none (accountServices is
// self-contained) — no host param needed.
export class RuntimeAccountServicesCommands {
  private accountServices: RuntimeAccountServices | null = null

  setAccountServices(services: RuntimeAccountServices): void {
    this.accountServices = services
  }

  private requireAccountServices(): RuntimeAccountServices {
    if (!this.accountServices) {
      throw new Error('Account services are not configured on this runtime')
    }
    return this.accountServices
  }

  // Why: server mode (backend/) never calls setAccountServices() today — the
  // underlying ClaudeAccountService/CodexAccountService detect CLI accounts
  // on the SAME machine Orca runs on, a desktop-only concept that doesn't
  // map to a headless multi-user server (real accounts live on whichever
  // remote Dev Server the user connects to, already covered by
  // preflight.detectAgents/AIProviderService). rate-limits.ts's RPC methods
  // use this to degrade to a safe empty state instead of throwing an
  // uncaught rejection on every page load — see that file's callers.
  hasAccountServices(): boolean {
    return this.accountServices !== null
  }

  getAccountsSnapshot(): AccountsSnapshot {
    const { claudeAccounts, codexAccounts, rateLimits } = this.requireAccountServices()
    return {
      claude: claudeAccounts.listAccounts(),
      codex: codexAccounts.listAccounts(),
      rateLimits: rateLimits.getState()
    }
  }

  // Why: RATE_LIMIT_METHODS (rpc/methods/rate-limits.ts) needs direct access to
  // the shared RateLimitService instance -- same service getAccountsSnapshot()
  // reads from, exposed for the desktop-parity rateLimits.* RPC namespace.
  getRateLimitService(): RateLimitService {
    return this.requireAccountServices().rateLimits
  }

  // Why: RateLimitService polls only when the Electron window is visible AND
  // focused, and the inactive-account caches fill lazily when the user opens
  // the desktop AccountsPane. Mobile has neither trigger, so without this the
  // phone shows 0% / "—" against a backgrounded desktop. Errors swallowed
  // because partial usage is still useful for the rest of the snapshot.
  async refreshAccountsForMobile(): Promise<void> {
    const { rateLimits } = this.requireAccountServices()
    await Promise.allSettled([
      rateLimits.refresh(),
      rateLimits.fetchInactiveClaudeAccountsOnOpen(),
      rateLimits.fetchInactiveCodexAccountsOnOpen()
    ])
  }

  selectClaudeAccount(accountId: string | null): Promise<ClaudeRateLimitAccountsState> {
    return this.requireAccountServices().claudeAccounts.selectAccount(accountId)
  }

  selectCodexAccount(accountId: string | null): Promise<CodexRateLimitAccountsState> {
    return this.requireAccountServices().codexAccounts.selectAccount(accountId)
  }

  removeClaudeAccount(accountId: string): Promise<ClaudeRateLimitAccountsState> {
    return this.requireAccountServices().claudeAccounts.removeAccount(accountId)
  }

  removeCodexAccount(accountId: string): Promise<CodexRateLimitAccountsState> {
    return this.requireAccountServices().codexAccounts.removeAccount(accountId)
  }

  // Why: rate-limit polling fires every 5 minutes and on account switch.
  // Mobile clients subscribe to receive a fresh AccountsSnapshot whenever
  // RateLimitService pushes new usage data, mirroring the existing
  // `rateLimits:update` IPC channel desktop already uses.
  onAccountsChanged(listener: (snapshot: AccountsSnapshot) => void): () => void {
    const services = this.requireAccountServices()
    return services.rateLimits.onStateChange((rateLimits) => {
      listener({
        claude: services.claudeAccounts.listAccounts(),
        codex: services.codexAccounts.listAccounts(),
        rateLimits
      })
    })
  }
}

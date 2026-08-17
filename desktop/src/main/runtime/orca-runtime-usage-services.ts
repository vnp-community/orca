// desktop/src/main/runtime/orca-runtime-usage-services.ts
// Why: the desktop-local RPC dispatcher runs in the same main process as the
// real ipcMain handlers, so claudeUsage.*/codexUsage.*/openCodeUsage.* method
// files need the same ClaudeUsageStore/CodexUsageStore/OpenCodeUsageStore
// instances `registerCoreHandlers` wires into ipcMain -- mirrors the
// composition pattern in orca-runtime-account-services.ts (setAccountServices).
import type { ClaudeUsageStore } from '../claude-usage/store'
import type { CodexUsageStore } from '../codex-usage/store'
import type { OpenCodeUsageStore } from '../opencode-usage/store'

export type RuntimeUsageServices = {
  claudeUsage: ClaudeUsageStore
  codexUsage: CodexUsageStore
  openCodeUsage: OpenCodeUsageStore
}

export class RuntimeUsageServicesCommands {
  private usageServices: RuntimeUsageServices | null = null

  setUsageServices(services: RuntimeUsageServices): void {
    this.usageServices = services
  }

  private requireUsageServices(): RuntimeUsageServices {
    if (!this.usageServices) {
      throw new Error('Usage services are not configured on this runtime')
    }
    return this.usageServices
  }

  getClaudeUsageStore(): ClaudeUsageStore {
    return this.requireUsageServices().claudeUsage
  }

  getCodexUsageStore(): CodexUsageStore {
    return this.requireUsageServices().codexUsage
  }

  getOpenCodeUsageStore(): OpenCodeUsageStore {
    return this.requireUsageServices().openCodeUsage
  }
}

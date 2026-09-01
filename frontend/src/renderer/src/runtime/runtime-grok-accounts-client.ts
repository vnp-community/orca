// Why: Grok status is derived from the CLI's own config on disk (no PTY
// login) -- local-only, so it goes straight through window.api.grokAccounts
// (real IPC on desktop, an honest "unsupported in browser" stub on web) --
// NOT callRuntimeRpc, which is a network call to backend-go and has no
// grokAccounts.* channels (same bug class as runtime-cli-client.ts's
// pre-fix cli.* calls — see backend-go solutions/README.md, "Sixteenth").
import type { GrokAccountStatus } from '../../../shared/rate-limit-types'

export async function getGrokAccountStatus(): Promise<GrokAccountStatus> {
  return window.api.grokAccounts.getStatus()
}

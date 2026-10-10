/**
 * Gateway-side cross-checks for the live UI flow: what the service says about a Request, to compare with the screen.
 * There are no fallbacks here on purpose: every product step must be possible through the UI.
 */
import type { GatewayRpcClient, RpcRow } from './gateway-rpc-client'

export async function findRequestByTitle(
  rpc: GatewayRpcClient,
  projectId: string,
  title: string
): Promise<RpcRow | undefined> {
  const listed = await rpc.call<{ requests?: RpcRow[] }>('request.list', { projectId })
  return (listed.requests ?? []).find((r) => r.title === title)
}

export async function requestState(rpc: GatewayRpcClient, id: string): Promise<RpcRow> {
  return (await rpc.call<{ request: RpcRow }>('request.get', { id })).request
}

export async function waitForApiStatus(
  rpc: GatewayRpcClient,
  id: string,
  wanted: string[],
  timeoutMs: number,
  intervalMs = 1_500
): Promise<RpcRow> {
  const deadline = Date.now() + timeoutMs
  let last = ''
  while (Date.now() < deadline) {
    const req = await requestState(rpc, id)
    last = req.status
    if (wanted.includes(req.status)) {
      return req
    }
    await new Promise((r) => setTimeout(r, intervalMs))
  }
  throw new Error(
    `request ${id} did not reach ${wanted.join('|')} within ${timeoutMs}ms (last: ${last})`
  )
}

export async function pendingApprovals(
  rpc: GatewayRpcClient,
  requestId: string
): Promise<RpcRow[]> {
  const listed = await rpc.call<{ approvals?: RpcRow[] }>('approval.list', {
    requestId,
    status: 'pending'
  })
  return listed.approvals ?? []
}

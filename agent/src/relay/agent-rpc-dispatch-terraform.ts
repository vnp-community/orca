// src/relay/agent-rpc-dispatch-terraform.ts
// terraform.apply — split into its own domain dispatcher file mirroring
// agent-rpc-dispatch-vm.ts/-git.ts's convention (one file per RPC-method
// domain, see agent-rpc-dispatch.ts's header comment). CR-FLEET-002,
// TASK-BE-FLEET-007.
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import type { JsonRpcRequest, JsonRpcResponse } from './agent-rpc-dispatch'
import { makeError } from './agent-rpc-dispatch'

export async function dispatchTerraformRpc(rpc: JsonRpcRequest): Promise<JsonRpcResponse | null> {
  switch (rpc.method) {
    // ── terraform.apply (ordinary request/response, mirrors vm.exec) ────────
    // Not a streaming method: apply/output both run to completion before
    // this handler resolves, unlike vm.provision's fire-and-forget +
    // stream.chunk/stream.end convention.
    case 'terraform.apply': {
      try {
        const { handleTerraformApply, validateTerraformApplyParams } =
          await import('./agent-terraform-handler')
        const params = validateTerraformApplyParams(rpc.params)
        const result = await handleTerraformApply(params)
        return { jsonrpc: '2.0', id: rpc.id, result }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `terraform.apply failed: ${msg}`)
      }
    }

    default:
      return null
  }
}

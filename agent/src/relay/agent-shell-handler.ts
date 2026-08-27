// src/relay/agent-shell-handler.ts
// Part A (direct-websocket/relay-websocket) implementation of fs.copyFile —
// the one genuinely new filesystem capability the web/server-mode
// DevServerFilePickerDialog flow needs beyond what fs-agent-extensions.ts
// already exposes (fs.stat/fs.readFile/fs.writeFile/fs.mkdir/fs.rmdir).
//
// Backs backend/src/main/runtime/rpc/methods/dev-server.ts's devServer.copyFile,
// itself the server/web-mode counterpart of desktop's shell.copyFile
// (copyPickedFile in desktop/src/main/ipc/shell.ts) — e.g. copying a picked
// image next to the markdown file that references it.

import { copyFile, mkdir } from 'node:fs/promises'
import { constants } from 'node:fs'
import { dirname, isAbsolute, resolve as resolvePath } from 'node:path'
import type { AgentConfig } from './agent-config'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { createTracer } from '../shared/trace'

const shellTracer = createTracer('agent:shell')

export async function handleFsCopyFile(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const rawSrc = typeof params.srcPath === 'string' ? params.srcPath : ''
  const rawDest = typeof params.destPath === 'string' ? params.destPath : ''
  const span = shellTracer.start({
    method: 'fs.copyFile',
    srcPath: rawSrc || '(empty)',
    destPath: rawDest || '(empty)'
  })

  if (!rawSrc || !rawDest) {
    span.fail('missing param: srcPath/destPath')
    return {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.InvalidParams,
        message: 'Missing required params: srcPath, destPath'
      }
    }
  }
  if (!isAbsolute(rawSrc) || !isAbsolute(rawDest)) {
    span.fail('non-absolute path', { srcPath: rawSrc, destPath: rawDest })
    return {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.InvalidParams,
        message: 'Both srcPath and destPath must be absolute paths'
      }
    }
  }

  const absDest = resolvePath(rawDest)
  const resolvedWork = resolvePath(config.workDir)
  // Why: mirrors fs.writeFile's containment check — the destination is always
  // inside a worktree (e.g. next to a markdown file). The source can be any
  // path the agent process can read, mirroring fs.readFile's unrestricted read.
  if (absDest !== resolvedWork && !absDest.startsWith(`${resolvedWork}/`)) {
    span.fail('dest outside project root', { destPath: rawDest })
    return {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.InvalidParams,
        message: `Destination outside project root: ${rawDest}`
      }
    }
  }

  try {
    await mkdir(dirname(absDest), { recursive: true })
    // Why COPYFILE_EXCL: mirrors desktop's copyPickedFile — the renderer-side
    // deconfliction loop already picks a unique destination name, so a
    // pre-existing dest means something is wrong; fail loudly instead of
    // silently clobbering data.
    await copyFile(rawSrc, absDest, constants.COPYFILE_EXCL)
    span.ok({ destPath: absDest })
    return { jsonrpc: '2.0', id, result: { ok: true, path: absDest } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { srcPath: rawSrc, destPath: absDest })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// src/relay/agent-exec-prompt-output-capture.ts
// Collects one agent.execPrompt run's stdout/stderr: bounded buffers for the result returned
// to the caller, and redacted live output for `agent.execPrompt.output` notifications.

import { realpathSync } from 'node:fs'
import { homedir, tmpdir } from 'node:os'
import { BoundedOutputBuffer } from './agent-bounded-output-buffer'
import { ClaudeStreamActivityParser } from './agent-claude-stream-activity'
import { resolveFinalStdout } from './agent-claude-final-stdout'
import type { StreamUsage } from './agent-claude-stream-usage'
import { createRedactor, secretEnvValuesFrom } from './quality-output-redaction'

export type ExecPromptOutputCaptureOptions = {
  /** claude runs with `--output-format stream-json`: live output is activity lines, stdout is the `result` text. */
  streamJson: boolean
  stdoutBytes: number
  stderrBytes: number
  spawnCwd: string
  env: Record<string, string>
  stepId: string | undefined
  notify?: (method: string, params: Record<string, unknown>) => void
}

export type ExecPromptOutputCapture = {
  onStdout: (chunk: Buffer) => void
  onStderr: (chunk: Buffer) => void
  /** Final answer text; call once the run ended (also settles any half-read stream line). */
  finalStdout: () => string
  stderrText: () => string
  /** Reason to add to stderr when claude ended without any answer text. */
  errorNote: () => string | null
  stdoutTruncated: () => boolean
  stderrTruncated: () => boolean
  /** Tool call counts and usage of a stream-json run; null for a plain-text run. */
  streamStats: () => { toolCalls: Record<string, number>; usage: StreamUsage | null } | null
}

// Real path too: claude reports the realpath'd cwd (e.g. /private/var on macOS) while the caller may pass the alias.
function cwdPrefixesFor(cwd: string): string[] {
  const prefixes = [cwd]
  try {
    prefixes.push(realpathSync(cwd))
  } catch {
    /* cwd was validated before spawn; a failed realpath only means no second alias */
  }
  return prefixes
}

export function createExecPromptOutputCapture(
  opts: ExecPromptOutputCaptureOptions
): ExecPromptOutputCapture {
  const stdoutBuf = new BoundedOutputBuffer(opts.stdoutBytes)
  const stderrBuf = new BoundedOutputBuffer(opts.stderrBytes)
  // Live output is redacted: every viewer of the task sees it, unlike the result returned to the caller.
  const redact = createRedactor({
    repoRoot: opts.spawnCwd,
    home: homedir(),
    tmpRoot: tmpdir(),
    secretEnvValues: secretEnvValuesFrom({ ...process.env, ...opts.env })
  })
  const emit = (stream: 'stdout' | 'stderr', data: string): void => {
    opts.notify?.('agent.execPrompt.output', { stepId: opts.stepId, stream, data })
  }
  const activity = opts.streamJson
    ? new ClaudeStreamActivityParser({
        emit: (text) => emit('stdout', text),
        redact,
        cwdPrefixes: cwdPrefixesFor(opts.spawnCwd)
      })
    : undefined
  let stdoutTruncated = false

  return {
    onStdout: (chunk) => {
      stdoutBuf.append(chunk)
      if (activity) {
        activity.push(chunk)
      } else {
        emit('stdout', redact(chunk.toString('utf8')))
      }
    },
    onStderr: (chunk) => {
      stderrBuf.append(chunk)
      emit('stderr', redact(chunk.toString('utf8')))
    },
    // stdout stays "the final answer": the `result` event in stream-json mode, the raw bytes otherwise.
    finalStdout: () => {
      if (!activity) {
        stdoutTruncated = stdoutBuf.truncated
        return stdoutBuf.toString()
      }
      const resolved = resolveFinalStdout(activity.finish(), stdoutBuf, opts.stdoutBytes)
      stdoutTruncated = resolved.truncated
      return resolved.stdout
    },
    stderrText: () => stderrBuf.toString(),
    errorNote: () => activity?.finish().errorNote ?? null,
    stdoutTruncated: () => stdoutTruncated,
    stderrTruncated: () => stderrBuf.truncated,
    streamStats: () => {
      const outcome = activity?.finish()
      return outcome?.structured ? { toolCalls: outcome.toolCalls, usage: outcome.usage } : null
    }
  }
}

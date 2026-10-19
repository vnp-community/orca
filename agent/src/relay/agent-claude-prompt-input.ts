// src/relay/agent-claude-prompt-input.ts
// How agent.execPrompt hands the prompt to `claude --print`.
//
// Default is stdin: the OS argv ceiling (about 128 KB per argument on Linux, 32 KB for a whole command line
// on Windows) cannot truncate or reject a large prompt, the prompt does not show up in the process list,
// and a prompt that starts with '-' cannot be parsed as a flag. ORCA_AGENT_PROMPT_VIA_ARGV=1 restores the
// previous argv behaviour as a rollback switch.

import type { Writable } from 'node:stream'
import type { AgentLogger } from './agent-logger'

/** Bytes per stdin write; small enough that a 500 KB prompt is paced by the child's reads instead of buffered at once. */
const STDIN_CHUNK_BYTES = 64 * 1024

/**
 * Read from the agent's own environment only, never from request-supplied `env`: a caller of the RPC must not
 * be able to flip how the prompt is delivered.
 */
export function promptViaArgvEnabled(source: NodeJS.ProcessEnv = process.env): boolean {
  const v = (source['ORCA_AGENT_PROMPT_VIA_ARGV'] ?? '').trim().toLowerCase()
  return v === '1' || v === 'true'
}

/** Project-context preamble (initFile) goes first, separated by a single newline, exactly as before. */
export function composePromptInput(initFile: string, prompt: string): string {
  return initFile ? `${initFile}\n${prompt}` : prompt
}

/** Composes the prompt input and picks the delivery path; logs once when the rollback switch is on (never the prompt). */
export function resolvePromptDelivery(opts: {
  initFile: string
  prompt: string
  stepId: string | undefined
  log: AgentLogger
}): { promptInput: string; viaArgv: boolean } {
  const viaArgv = promptViaArgvEnabled()
  if (viaArgv) {
    opts.log.info(
      `agent.execPrompt: ORCA_AGENT_PROMPT_VIA_ARGV is set, prompt passed as argv (stepId=${opts.stepId ?? '(none)'})`
    )
  }
  return {
    promptInput: composePromptInput(opts.initFile, opts.prompt),
    viaArgv
  }
}

/** The first argv elements for the one-shot run: the prompt is an argument only in the rollback mode. */
export function printModeLeadArgs(promptInput: string, viaArgv: boolean): string[] {
  return viaArgv ? ['--print', promptInput] : ['--print']
}

/**
 * Writes the prompt to the child's stdin in chunks and closes it. Honours backpressure (waits for 'drain')
 * and swallows stream errors: when the child exits before reading everything (EPIPE) its exit code and
 * stderr are the real failure signal, and an unhandled 'error' event would crash the agent.
 */
export function writePromptToStdin(stdin: Writable, promptInput: string): void {
  stdin.on('error', () => undefined)
  const data = Buffer.from(promptInput, 'utf8')
  let offset = 0
  const pump = (): void => {
    while (offset < data.length) {
      if (stdin.destroyed || !stdin.writable) {
        return
      }
      const end = Math.min(offset + STDIN_CHUNK_BYTES, data.length)
      const flushed = stdin.write(data.subarray(offset, end))
      offset = end
      if (!flushed) {
        stdin.once('drain', pump)
        return
      }
    }
    stdin.end()
  }
  pump()
}

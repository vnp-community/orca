// src/relay/agent-print-mode-args.ts
// The claude argument list of one agent.execPrompt run, minus the knowledge-hub flags added later.

import { YOLO_TUI_AGENT_ARGS } from '../shared/tui-agent-permissions'
import type { AgentLogger } from './agent-logger'
import type { ExecPromptOptions } from './agent-exec-prompt-options'
import { profileArgs } from './agent-permission-profiles'
import { printModeLeadArgs } from './agent-claude-prompt-input'

type PrintModeArgsInput = {
  promptInput: string
  /** Rollback mode (ORCA_AGENT_PROMPT_VIA_ARGV): the prompt rides in argv instead of stdin. */
  viaArgv: boolean
  streamJson: boolean
  options: Pick<ExecPromptOptions, 'accessMode' | 'permissionProfile'>
  trustPresetFull: boolean
  readonlyArgs: readonly string[]
  log: AgentLogger
}

export function buildPrintModeArgs(input: PrintModeArgsInput): {
  args: string[]
  warnings: string[]
} {
  const { options, trustPresetFull } = input
  const warnings: string[] = []
  const args = printModeLeadArgs(input.promptInput, input.viaArgv)
  // Why: stream-json is what exposes tool calls to the web UI; an older claude, or a failed
  // probe, keeps the previous plain-text invocation so execution never breaks.
  if (input.streamJson) {
    args.push('--output-format', 'stream-json', '--verbose')
  }
  if (options.permissionProfile) {
    args.push(...profileArgs(options.permissionProfile))
    // A named profile is the whole permission story of the run; full trust would void it.
    if (trustPresetFull) {
      warnings.push('TRUST_PRESET_IGNORED_PROFILE')
    }
  } else if (options.accessMode === 'write' && trustPresetFull && YOLO_TUI_AGENT_ARGS.claude) {
    args.push(YOLO_TUI_AGENT_ARGS.claude)
  } else if (options.accessMode === 'readonly') {
    args.push(...input.readonlyArgs)
    // YOLO flags override tool restrictions — discard them to keep readonly enforced
    if (trustPresetFull) {
      input.log.warn(
        'agent.execPrompt: trustPreset=full ignored in readonly mode (TRUST_PRESET_IGNORED_READONLY)'
      )
      warnings.push('TRUST_PRESET_IGNORED_READONLY')
    }
  }
  return { args, warnings }
}

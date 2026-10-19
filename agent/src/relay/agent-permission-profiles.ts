// src/relay/agent-permission-profiles.ts
// Named permission profiles for write-mode agent.execPrompt runs that must not get full trust. The profile is chosen by
// name; every flag below is fixed here so no request-controlled string becomes a tool rule.

export type PermissionProfileName = 'openspec-author'

type PermissionProfile = {
  /** Built-in tools that exist at all in the run. */
  tools: readonly string[]
  permissionMode: 'acceptEdits'
  /** Tool rules that run without a prompt (a headless run cannot answer one, so everything else is denied). */
  allowedTools: readonly string[]
}

// Why: authoring an OpenSpec change needs file edits inside the worktree and the openspec / tdd-agent status commands, and
// nothing else. acceptEdits keeps edits inside the working directory; any other Bash command is denied, not asked.
const PROFILES: Record<PermissionProfileName, PermissionProfile> = {
  'openspec-author': {
    tools: ['Read', 'Glob', 'Grep', 'Write', 'Edit', 'Bash'],
    permissionMode: 'acceptEdits',
    allowedTools: [
      'Read',
      'Glob',
      'Grep',
      'Write',
      'Edit',
      'Bash(openspec:*)',
      'Bash(tdd-agent -C * status:*)'
    ]
  }
}

export function isPermissionProfileName(v: unknown): v is PermissionProfileName {
  return typeof v === 'string' && Object.hasOwn(PROFILES, v)
}

export function profileAllowedTools(name: PermissionProfileName): readonly string[] {
  return PROFILES[name].allowedTools
}

/** Flags for the profile; allowedTools are added by the caller because the knowledge hub merges its own into one flag. */
export function profileArgs(name: PermissionProfileName): string[] {
  const p = PROFILES[name]
  return ['--permission-mode', p.permissionMode, '--tools', p.tools.join(',')]
}

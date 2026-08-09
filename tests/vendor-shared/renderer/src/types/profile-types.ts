// profile-types.ts — Shared types for Profile Hierarchy (TDD-FE-11)

export type ProfileSource = 'company' | 'dept' | 'user' | 'concat'

export type McpServerConfig = {
  name:    string
  command: string
  args?:   string[]
  env?:    Record<string, string>
}

export type OrcaProfile = {
  agent?: {
    preferredModel?:     string
    trustPreset?:        'strict' | 'standard' | 'relaxed' | 'custom'
    customInstructions?: string
    approvedModels?:     string[]  // user-level override (subset of company approved)
  }
  editor?: {
    theme?:     string
    fontSize?:  number
    fontFamily?: string
    keybindings?: 'vscode' | 'vim' | 'emacs'
  }
  mcp?: {
    servers?: McpServerConfig[]
  }
  shell?: {
    defaultShell?:    string
    pathAdditions?:   string[]
    envVars?:         Record<string, string>
    startupCommands?: string[]
  }
  integrations?: {
    githubOrg?:       string
    linearWorkspace?: string
    prTemplate?:      string
  }
  fleet?: {
    allowedServerTags?:      string[]
    defaultConnectionType?:  'ssh' | 'local'
  }
  security?: {
    approvedModels?:  string[]   // glob patterns, e.g. 'claude-*'
    disallowedCmds?:  string[]
    require2FA?:      boolean
    sessionTimeoutHours?: number
  }
}

export type ResolvedProfile = OrcaProfile & {
  _sources: Record<string, ProfileSource>  // e.g. 'agent.preferredModel' → 'dept'
}

export type Department = {
  id:           string
  name:         string
  parentId:     string | null
  leadId?:      string
  memberCount?: number
}

// Shared types cho Profile Hierarchy (TDD-FE-11)

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
  }
  mcp?: {
    servers?: McpServerConfig[]
  }
  shell?: {
    defaultShell?:  string
    pathAdditions?: string[]
    envVars?:       Record<string, string>
  }
  security?: {
    approvedModels?: string[]   // glob patterns, e.g. 'claude-*'
    disallowedCmds?: string[]
  }
}

export type ResolvedProfile = OrcaProfile & {
  _sources: Record<string, ProfileSource>  // e.g. 'agent.preferredModel' → 'dept'
}

export type Department = {
  id:       string
  name:     string
  parentId: string | null
}

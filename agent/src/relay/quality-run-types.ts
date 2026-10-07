export interface PlannedStep {
  id: string
  toolPath: string
  args: string[]
  cwd: string
  env: NodeJS.ProcessEnv
  timeoutMs: number
  maxOutputBytes: number
  heavy: boolean
  parser: string
}

export interface StepExecResult {
  kind: 'success' | 'failed_exit' | 'spawn_error' | 'timeout' | 'output_too_large' | 'cancelled' | 'gate_timeout'
  exitCode: number | null
  stdoutTail: string
  stderrTail: string
  durationMs: number
}

export interface StepParser {
  name: string
  parse(stdoutPath: string, stderrPath: string): Promise<any>
}

export interface QualityEnvMissing {
  reason: string
  hint?: string
  required?: string
  built?: string
}

export interface QualityFinding {
  ruleId: string;
  level: string;
  message: string;
  file?: string;
  line?: number;
  col?: number;
  outsideScope?: boolean;
}

export interface QualityStepResult {
  id: string;
  tool: string;
  state: string;
  durationMs: number;
  ruleResults?: {
    ruleId: string;
    status: 'ran' | 'skipped_scope' | 'script_not_found' | 'env_not_ready' | 'disabled';
    durationMs?: number;
  }[];
}

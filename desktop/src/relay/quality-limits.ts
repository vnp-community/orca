export const ORCA_HEAVY_QUEUE_WAIT_MS = parseInt(process.env.ORCA_HEAVY_QUEUE_WAIT_MS || '600000', 10) || 600000

export const ORCA_QUALITY_QUEUE_MAX = parseInt(process.env.ORCA_QUALITY_QUEUE_MAX || '4', 10) || 4;
export const ORCA_QUALITY_RUN_TIMEOUT_MS = parseInt(process.env.ORCA_QUALITY_RUN_TIMEOUT_MS || '2700000', 10) || 2700000;
export const ORCA_QUALITY_RESULT_TTL_MS = parseInt(process.env.ORCA_QUALITY_RESULT_TTL_MS || '3600000', 10) || 3600000;
export const ORCA_QUALITY_CONCURRENT_RUNS = parseInt(process.env.ORCA_QUALITY_CONCURRENT_RUNS || '1', 10) || 1;
export const ORCA_QUALITY_RUNS_PER_WORKTREE = parseInt(process.env.ORCA_QUALITY_RUNS_PER_WORKTREE || '20', 10) || 20;

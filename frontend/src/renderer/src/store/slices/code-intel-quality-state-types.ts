/**
 * code-intel-quality-state-types.ts — FE-CV-TASK-087-02
 *
 * Shape of the in-memory quality state kept per worktree (never persisted: it holds findings
 * and run output). Actions live in code-intel-quality-state.ts.
 *
 * @module store/slices/code-intel-quality-state-types
 */

import type { CodeIntelRpcError } from '../../runtime/code-intel-client'
import type {
  QualityCategory,
  QualityCoverageResponse,
  QualityFinding,
  QualityGateResponse,
  QualityProfileResponse,
  QualityRun,
  QualityRunScope,
  QualityRunStatus,
  QualitySeverity,
  QualityTrendResponse
} from '../../../../shared/code-intel-quality-types'
import type { MissingCheck } from '../../../../shared/code-intel-quality-types'

export type CachedQuality<T> = { data: T; fetchedAt: number; stale: boolean }

export type QualityRunPhase = 'starting' | 'queued' | 'running' | 'cancelling' | 'finished'

export type ActiveQualityRun = {
  /** null only while `phase === 'starting'` (the backend has not answered yet). */
  runId: string | null
  profile: string
  scope: QualityRunScope
  phase: QualityRunPhase
  /** Terminal status once `phase === 'finished'`; 'interrupted' is not a gate result. */
  status?: QualityRunStatus
  stage: string
  /** null = unknown (render a static spinner, never a fake percentage). */
  percent: number | null
  stepIndex?: number
  stepCount?: number
  message: string
  startedAt: number
}

/** Why a start request failed; rendered inline and persistent, never as a toast. */
export type QualityRunError = {
  kind: CodeIntelRpcError['kind']
  message: string
  missing?: MissingCheck[]
  available?: string[]
  retryAfterSeconds?: number
  runId?: string
}

export type QualityResource = 'gate' | 'runs' | 'profiles' | 'trend' | 'coverage'

export type QualityUiState = {
  profile: string | null
  runScope: 'worktree' | 'changed' | 'commitRange'
  source: 'structure' | 'quality'
  annotationsOn: boolean
  severity: QualitySeverity[]
  category: QualityCategory[]
  onlyInScope: boolean
  showWaived: boolean
  selectedFingerprint: string | null
  openBlocks: string[]
}

export type QualityTrendGroup = 'commit' | 'turn'

export type QualityWorktreeState = {
  run: ActiveQualityRun | null
  runError: QualityRunError | null
  gate: CachedQuality<QualityGateResponse> | null
  runs: CachedQuality<QualityRun[]> | null
  profiles: CachedQuality<QualityProfileResponse> | null
  trend: Partial<Record<QualityTrendGroup, CachedQuality<QualityTrendResponse>>>
  coverage: CachedQuality<QualityCoverageResponse> | null
  /** At most 16 files; used by diff annotations when the loaded list is not complete. */
  findingsByFile: Record<string, CachedQuality<QualityFinding[]>>
  loading: Partial<Record<QualityResource, boolean>>
  errors: Partial<Record<QualityResource, CodeIntelRpcError>>
  /** Bumped when a run finishes or the gate changes so finding lists refetch. */
  epoch: number
  ui: QualityUiState
}

export const DEFAULT_QUALITY_UI: QualityUiState = {
  profile: null,
  runScope: 'changed',
  source: 'structure',
  annotationsOn: true,
  severity: [],
  category: [],
  onlyInScope: true,
  showWaived: false,
  selectedFingerprint: null,
  openBlocks: []
}

export const EMPTY_QUALITY_WORKTREE_STATE: QualityWorktreeState = {
  run: null,
  runError: null,
  gate: null,
  runs: null,
  profiles: null,
  trend: {},
  coverage: null,
  findingsByFile: {},
  loading: {},
  errors: {},
  epoch: 0,
  ui: DEFAULT_QUALITY_UI
}

export const MAX_FINDINGS_FILES_PER_WORKTREE = 16

// frontend/src/main/runtime/orca-runtime-graph-store.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-041): pure state container for the
// "live graph" (leaves, tabs, PTY handles, waiters) that OrcaRuntimeService's
// own header comment calls out as the reason it can't yet drop below the
// oxlint max-lines budget. Field-access analysis (TASK-BIGFILE-035) found
// these fields referenced across 15,000-21,000+ lines of the class body —
// too pervasive to move alongside any single domain, so they get their own
// dedicated, logic-free holder instead. No behavior change: every field
// keeps its exact original type/initializer, just addressed via
// `this.graph.<field>` instead of `this.<field>` from OrcaRuntimeService.
import type {
  RuntimeLeafRecord,
  RuntimePtyWorktreeRecord,
  TerminalHandleRecord,
  TerminalWaiter
} from './orca-runtime'
import type { RuntimeGraphStatus, RuntimeSyncedTab } from '../../shared/runtime-types'

export class RuntimeGraphStore {
  rendererGraphEpoch = 0
  graphStatus: RuntimeGraphStatus = 'unavailable'
  authoritativeWindowId: number | null = null
  tabs = new Map<string, RuntimeSyncedTab>()
  leaves = new Map<string, RuntimeLeafRecord>()
  // Why: PTY output is a per-keystroke hot path. Looking up affected leaves by
  // ptyId keeps active TUI redraws independent of the total open terminal count.
  leavesByPtyId = new Map<string, RuntimeLeafRecord[]>()
  handles = new Map<string, TerminalHandleRecord>()
  handleByLeafKey = new Map<string, string>()
  handleByPtyId = new Map<string, string>()
  detachedPreAllocatedLeaves = new Map<string, RuntimeLeafRecord>()
  graphSyncCallbacks: (() => void)[] = []
  waitersByHandle = new Map<string, Set<TerminalWaiter>>()
  ptysById = new Map<string, RuntimePtyWorktreeRecord>()
}

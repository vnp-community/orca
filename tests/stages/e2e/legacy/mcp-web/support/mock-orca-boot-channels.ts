// Minimal answers for the non-MCP channels the web SPA calls while booting. Without them the
// session/graph hydration fails and a cold-start deep link is lost; MCP channels stay on the fake.
import {
  MIN_COMPATIBLE_RUNTIME_CLIENT_VERSION,
  RUNTIME_PROTOCOL_VERSION
} from '../../../../frontend/src/shared/protocol-version'

export const BOOT_CHANNEL_STUBS: Record<string, unknown> = {
  'status.get': {
    runtimeId: 'backend-go',
    rendererGraphEpoch: 1,
    graphStatus: 'ready',
    authoritativeWindowId: null,
    liveTabCount: 0,
    liveLeafCount: 0,
    runtimeProtocolVersion: RUNTIME_PROTOCOL_VERSION,
    minCompatibleRuntimeClientVersion: MIN_COMPATIBLE_RUNTIME_CLIENT_VERSION
  },
  'clientState.get': { found: false },
  'worktree.lineageList': { lineage: {} },
  // Closed flow so the first-run wizard never covers the screen under test.
  'onboarding.get': {
    flowVersion: 1,
    closedAt: 1,
    outcome: 'dismissed',
    lastCompletedStep: 5,
    checklist: {}
  },
  'telemetry.track': null,
  'project.list': [],
  'devServer.list': [],
  'rateLimits.get': {
    claude: null,
    codex: null,
    gemini: null,
    opencodeGo: null,
    kimi: null,
    antigravity: null,
    minimax: null,
    grok: null
  },
  'crashReports.getLatestPending': null,
  'clientState.set': {},
  'session.tabs.listAll': { snapshots: [] },
  'profile.getUserProfile': {
    userId: 'u1',
    companyId: 'c1',
    departmentId: 'd1',
    settingsJson: '{}'
  },
  'runtime.clientEvents.subscribe': null,
  'session.tabs.subscribeAll': null,
  'connectivity.getSummary': { connections: [] }
}

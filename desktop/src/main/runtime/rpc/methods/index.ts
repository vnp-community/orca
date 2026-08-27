import type { RpcAnyMethod } from '../core'
import { STATUS_METHODS } from './status'
import { AI_VAULT_METHODS } from './ai-vault'
import { AUTOMATION_METHODS } from './automations'
import { REPO_METHODS } from './repo'
import { WORKTREE_METHODS } from './worktree'
import { TERMINAL_METHODS } from './terminal'
import { BROWSER_CORE_METHODS } from './browser-core'
import { BROWSER_EXTRA_METHODS } from './browser-extras'
import { BROWSER_SCREENCAST_METHODS } from './browser-screencast'
import { ORCHESTRATION_METHODS } from './orchestration'
import { NOTIFICATION_METHODS } from './notifications'
import { STATS_METHODS } from './stats'
import { DIAGNOSTICS_METHODS } from './diagnostics'
import { ACCOUNT_METHODS } from './accounts'
import { PREFLIGHT_METHODS } from './preflight'
import { COMPUTER_METHODS } from './computer'
import { SESSION_TAB_METHODS } from './session-tabs'
import { NATIVE_CHAT_METHODS } from './native-chat'
import { FILE_METHODS } from './files'
import { GIT_METHODS } from './git'
import { GITHUB_METHODS } from './github'
import { GITLAB_METHODS } from './gitlab'
import { HOSTED_REVIEW_METHODS } from './hosted-review'
import { LINEAR_METHODS } from './linear'
import { LINEAR_AGENT_ACCESS_METHODS } from './linear-agent-access'
import { JIRA_METHODS } from './jira'
import { SSH_METHODS } from './ssh'
import { SPEECH_METHODS } from './speech'
import { CLIENT_UI_METHODS } from './client-ui'
import { CLIENT_EVENT_METHODS } from './client-events'
import { WORKSPACE_PORT_METHODS } from './workspace-ports'
import { SKILL_METHODS } from './skills'
import { CLIPBOARD_METHODS } from './clipboard'
import { HOST_CAPABILITY_METHODS } from './host-capabilities'
import { EMULATOR_METHODS } from './emulator'
import { GITHUB_AUTH_METHODS } from './github-auth'
import { GITLAB_AUTH_METHODS } from './gitlab-auth'
import { CREDENTIAL_METHODS } from './credentials'
import { DEV_SERVER_METHODS } from './dev-server'
// Below: added while migrating window.api.* (Electron IPC) call sites in
// frontend/ to callRuntimeRpc() — each wraps the exact function its ipcMain
// handler already calls, so desktop's local RPC dispatcher can reach the
// same native-only capabilities without duplicating logic.
import { SHELL_METHODS } from './shell'
import { APP_METHODS } from './app'
import { UPDATER_METHODS } from './updater'
import { PLATFORM_METHODS } from './platform'
import { AGENT_TRUST_METHODS } from './agent-trust'
import { COMPUTER_USE_PERMISSIONS_METHODS } from './computer-use-permissions'
import { DEVELOPER_PERMISSIONS_METHODS } from './developer-permissions'
import { CRASH_REPORTS_METHODS } from './crash-reports'
import { DIAGNOSTICS_CRASH_BUNDLE_METHODS } from './diagnostics-crash-bundle'
import { TELEMETRY_METHODS } from './telemetry'
import { MEMORY_METHODS } from './memory'
import { E2E_METHODS } from './e2e'
import { CACHE_METHODS } from './cache'
import { MINIMAX_CREDENTIALS_METHODS } from './minimax-credentials'
import { GROK_ACCOUNTS_METHODS } from './grok-accounts'
import { CLAUDE_USAGE_METHODS } from './claude-usage'
import { CODEX_USAGE_METHODS } from './codex-usage'
import { OPEN_CODE_USAGE_METHODS } from './opencode-usage'
import { RATE_LIMIT_METHODS } from './rate-limits'
import { CLAUDE_ACCOUNTS_METHODS } from './claude-accounts'
import { CODEX_ACCOUNTS_METHODS } from './codex-accounts'
import { AGENT_STATUS_METHODS } from './agent-status'
import { MOBILE_METHODS } from './mobile'
import { STAR_NAG_METHODS } from './star-nag'
import { ORCA_PROFILES_METHODS } from './orca-profiles'
import { ONBOARDING_METHODS } from './onboarding'
import { WORKSPACE_SPACE_METHODS } from './workspace-space'
import { WORKSPACE_CLEANUP_METHODS } from './workspace-cleanup'
import { REMOTE_WORKSPACE_METHODS } from './remote-workspace'
import { SPARSE_PRESET_METHODS } from './sparse-presets'
import { LOCALHOST_WORKTREE_LABEL_METHODS } from './localhost-worktree-labels'
import { CLI_METHODS } from './cli'
import { EPHEMERAL_VM_METHODS } from './ephemeral-vm'
import { PET_METHODS } from './pet'
import { FEEDBACK_METHODS } from './feedback'
import { EXPORT_METHODS } from './export'

// Why: a flat manifest keeps registration order explicit and provides one
// grep-point for "what methods does the RPC server expose?" — useful when
// auditing the security boundary or wiring new CLI commands.
export const ALL_RPC_METHODS: readonly RpcAnyMethod[] = [
  ...STATUS_METHODS,
  ...AI_VAULT_METHODS,
  ...AUTOMATION_METHODS,
  ...REPO_METHODS,
  ...WORKTREE_METHODS,
  ...TERMINAL_METHODS,
  ...BROWSER_CORE_METHODS,
  ...BROWSER_SCREENCAST_METHODS,
  ...BROWSER_EXTRA_METHODS,
  ...ORCHESTRATION_METHODS,
  ...NOTIFICATION_METHODS,
  ...STATS_METHODS,
  ...DIAGNOSTICS_METHODS,
  ...ACCOUNT_METHODS,
  ...PREFLIGHT_METHODS,
  ...COMPUTER_METHODS,
  ...SESSION_TAB_METHODS,
  ...NATIVE_CHAT_METHODS,
  ...FILE_METHODS,
  ...GIT_METHODS,
  ...GITHUB_METHODS,
  ...GITLAB_METHODS,
  ...HOSTED_REVIEW_METHODS,
  ...LINEAR_METHODS,
  ...LINEAR_AGENT_ACCESS_METHODS,
  ...JIRA_METHODS,
  ...SSH_METHODS,
  ...SPEECH_METHODS,
  ...WORKSPACE_PORT_METHODS,
  ...SKILL_METHODS,
  ...CLIPBOARD_METHODS,
  ...HOST_CAPABILITY_METHODS,
  ...CLIENT_EVENT_METHODS,
  ...CLIENT_UI_METHODS,
  ...EMULATOR_METHODS,
  ...GITHUB_AUTH_METHODS,
  ...GITLAB_AUTH_METHODS,
  ...CREDENTIAL_METHODS,
  ...DEV_SERVER_METHODS,
  ...SHELL_METHODS,
  ...APP_METHODS,
  ...UPDATER_METHODS,
  ...PLATFORM_METHODS,
  ...AGENT_TRUST_METHODS,
  ...COMPUTER_USE_PERMISSIONS_METHODS,
  ...DEVELOPER_PERMISSIONS_METHODS,
  ...CRASH_REPORTS_METHODS,
  ...DIAGNOSTICS_CRASH_BUNDLE_METHODS,
  ...TELEMETRY_METHODS,
  ...MEMORY_METHODS,
  ...E2E_METHODS,
  ...CACHE_METHODS,
  ...MINIMAX_CREDENTIALS_METHODS,
  ...GROK_ACCOUNTS_METHODS,
  ...CLAUDE_USAGE_METHODS,
  ...CODEX_USAGE_METHODS,
  ...OPEN_CODE_USAGE_METHODS,
  ...RATE_LIMIT_METHODS,
  ...CLAUDE_ACCOUNTS_METHODS,
  ...CODEX_ACCOUNTS_METHODS,
  ...AGENT_STATUS_METHODS,
  ...MOBILE_METHODS,
  ...STAR_NAG_METHODS,
  ...ORCA_PROFILES_METHODS,
  ...ONBOARDING_METHODS,
  ...WORKSPACE_SPACE_METHODS,
  ...WORKSPACE_CLEANUP_METHODS,
  ...REMOTE_WORKSPACE_METHODS,
  ...SPARSE_PRESET_METHODS,
  ...LOCALHOST_WORKTREE_LABEL_METHODS,
  ...CLI_METHODS,
  ...EPHEMERAL_VM_METHODS,
  ...PET_METHODS,
  ...FEEDBACK_METHODS,
  ...EXPORT_METHODS
]

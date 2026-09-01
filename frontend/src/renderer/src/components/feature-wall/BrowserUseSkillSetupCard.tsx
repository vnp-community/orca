import type { JSX } from 'react'
import {
  ORCA_CLI_SKILL_INSTALL_COMMAND,
  ORCA_CLI_SKILL_UPDATE_COMMAND
} from '@/lib/agent-feature-install-commands'
import {
  AGENT_SKILL_CLI_PREREQUISITE_NOTICE,
  ensureOrcaCliAvailableForAgentSkillTerminal
} from '@/lib/agent-skill-cli-prerequisite'
import { BROWSER_USE_ENABLED_STORAGE_KEY } from '@/lib/browser-use-setup-state'
import type { InstalledAgentSkillState } from '@/hooks/useInstalledAgentSkills'
import { useActiveProjectSkillRuntime } from '@/hooks/useActiveProjectSkillRuntime'
import { useActiveDevServer, useConnectedDevServers } from '@/store/slices/dev-servers-selectors'
import { AgentSkillSetupPanel } from '@/components/settings/AgentSkillSetupPanel'
import {
  buildSkillCommandForRuntime,
  ensureWslCliAvailableForAgentSkillTerminal,
  getWslCliDistroRequest
} from '@/components/settings/CliSkillRuntimeSetup'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import {
  getRuntimeCliInstallStatus,
  getRuntimeWslCliInstallStatus
} from '@/runtime/runtime-cli-client'

export function BrowserUseSkillSetupCard(props: {
  compact?: boolean
  terminalHeightPx?: number
  skill: InstalledAgentSkillState
}): JSX.Element {
  const { compact, terminalHeightPx, skill } = props
  const activeSkillRuntime = useActiveProjectSkillRuntime()
  // Why: this panel's setup terminal has no project/repo behind it, so it
  // has no natural dev-server binding to inherit — see
  // OnboardingInlineCommandTerminal's devServerId doc comment.
  const activeDevServer = useActiveDevServer()
  const connectedDevServers = useConnectedDevServers()
  const devServerId =
    activeDevServer?.status === 'connected'
      ? activeDevServer.id
      : (connectedDevServers[0]?.id ?? null)
  const installCommand = !activeSkillRuntime.installDisabledReason
    ? buildSkillCommandForRuntime(ORCA_CLI_SKILL_INSTALL_COMMAND, activeSkillRuntime.agentRuntime)
    : ORCA_CLI_SKILL_INSTALL_COMMAND
  const updateCommand = !activeSkillRuntime.installDisabledReason
    ? buildSkillCommandForRuntime(ORCA_CLI_SKILL_UPDATE_COMMAND, activeSkillRuntime.agentRuntime)
    : ORCA_CLI_SKILL_UPDATE_COMMAND

  const handleBeforeOpenTerminal = async (): Promise<void> => {
    useAppStore.getState().recordFeatureInteraction('agent-browser-setup')
    await (activeSkillRuntime.agentRuntime?.runtime === 'wsl'
      ? ensureWslCliAvailableForAgentSkillTerminal(activeSkillRuntime.agentRuntime)
      : ensureOrcaCliAvailableForAgentSkillTerminal())
    localStorage.setItem(BROWSER_USE_ENABLED_STORAGE_KEY, '1')
  }

  const setupPanel = (
    <AgentSkillSetupPanel
      className={compact ? 'w-full max-w-[520px]' : undefined}
      title={translate(
        'auto.components.feature.wall.BrowserUseSkillSetupCard.d5bb1cd4ba',
        'Browser Use skill'
      )}
      description={translate(
        'auto.components.feature.wall.BrowserUseSkillSetupCard.cbc45022d4',
        "Enables agents to navigate and verify pages in Orca's browser."
      )}
      command={installCommand}
      installedCommand={updateCommand}
      terminalTitle="Browser Use setup"
      terminalAriaLabel="Browser Use skill install terminal"
      terminalWorktreeId="feature-wall-browser-use-skill-terminal"
      devServerId={devServerId}
      terminalShellOverride={activeSkillRuntime.terminalShellOverride}
      installed={skill.installed}
      loading={skill.loading}
      error={activeSkillRuntime.installDisabledReason ?? skill.error}
      installDisabled={Boolean(activeSkillRuntime.installDisabledReason)}
      terminalHeightPx={terminalHeightPx}
      preInstallNotice={AGENT_SKILL_CLI_PREREQUISITE_NOTICE}
      getPrerequisiteStatus={() =>
        activeSkillRuntime.agentRuntime?.runtime === 'wsl'
          ? getRuntimeWslCliInstallStatus(getWslCliDistroRequest(activeSkillRuntime.agentRuntime))
          : getRuntimeCliInstallStatus()
      }
      onBeforeOpenTerminal={handleBeforeOpenTerminal}
      showRecheckWhenInstalled={false}
      onRecheck={skill.refresh}
    />
  )

  if (compact) {
    return <div className="flex min-h-24 flex-1 items-center justify-center pt-3">{setupPanel}</div>
  }
  return <div className="flex">{setupPanel}</div>
}

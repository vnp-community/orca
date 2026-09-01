import { MonitorCog } from 'lucide-react'
import {
  COMPUTER_USE_SKILL_INSTALL_COMMAND,
  COMPUTER_USE_SKILL_NAME,
  COMPUTER_USE_SKILL_UPDATE_COMMAND
} from '@/lib/agent-feature-install-commands'
import {
  AGENT_SKILL_CLI_PREREQUISITE_NOTICE,
  ensureOrcaCliAvailableForAgentSkillTerminal
} from '@/lib/agent-skill-cli-prerequisite'
import {
  GLOBAL_AGENT_SKILL_SOURCE_KINDS,
  useInstalledAgentSkill
} from '@/hooks/useInstalledAgentSkills'
import { useActiveProjectSkillRuntime } from '@/hooks/useActiveProjectSkillRuntime'
import { useActiveDevServer, useConnectedDevServers } from '@/store/slices/dev-servers-selectors'
import { useAppStore } from '@/store'
import { AgentSkillSetupPanel } from './AgentSkillSetupPanel'
import {
  buildSkillCommandForRuntime,
  ensureWslCliAvailableForAgentSkillTerminal,
  getWslCliDistroRequest
} from './CliSkillRuntimeSetup'
import { translate } from '@/i18n/i18n'
import {
  getRuntimeCliInstallStatus,
  getRuntimeWslCliInstallStatus
} from '@/runtime/runtime-cli-client'

export function ComputerUseSkillSetupPanel(): React.JSX.Element {
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
    ? buildSkillCommandForRuntime(
        COMPUTER_USE_SKILL_INSTALL_COMMAND,
        activeSkillRuntime.agentRuntime
      )
    : COMPUTER_USE_SKILL_INSTALL_COMMAND
  const updateCommand = !activeSkillRuntime.installDisabledReason
    ? buildSkillCommandForRuntime(
        COMPUTER_USE_SKILL_UPDATE_COMMAND,
        activeSkillRuntime.agentRuntime
      )
    : COMPUTER_USE_SKILL_UPDATE_COMMAND
  const {
    installed: computerUseSkillDetected,
    loading: computerUseSkillLoading,
    error: computerUseSkillError,
    refresh: refreshComputerUseSkill
  } = useInstalledAgentSkill(COMPUTER_USE_SKILL_NAME, {
    discoveryTarget: activeSkillRuntime.discoveryTarget,
    sourceKinds: GLOBAL_AGENT_SKILL_SOURCE_KINDS
  })

  return (
    <AgentSkillSetupPanel
      title={translate('auto.components.settings.ComputerUsePane.93255aaf18', 'Computer Use skill')}
      description={translate(
        'auto.components.settings.ComputerUsePane.1735461723',
        'Enables agents to inspect and operate local desktop apps.'
      )}
      command={installCommand}
      installedCommand={updateCommand}
      terminalTitle="Computer Use setup"
      terminalAriaLabel="Computer Use skill install terminal"
      terminalWorktreeId="settings-computer-use-skill-terminal"
      devServerId={devServerId}
      terminalShellOverride={activeSkillRuntime.terminalShellOverride}
      installed={computerUseSkillDetected}
      loading={computerUseSkillLoading}
      error={activeSkillRuntime.installDisabledReason ?? computerUseSkillError}
      installDisabled={Boolean(activeSkillRuntime.installDisabledReason)}
      icon={<MonitorCog className="size-5" />}
      preInstallNotice={AGENT_SKILL_CLI_PREREQUISITE_NOTICE}
      getPrerequisiteStatus={() =>
        activeSkillRuntime.agentRuntime?.runtime === 'wsl'
          ? getRuntimeWslCliInstallStatus(getWslCliDistroRequest(activeSkillRuntime.agentRuntime))
          : getRuntimeCliInstallStatus()
      }
      onBeforeOpenTerminal={async () => {
        useAppStore.getState().recordFeatureInteraction('computer-use-setup')
        await (activeSkillRuntime.agentRuntime?.runtime === 'wsl'
          ? ensureWslCliAvailableForAgentSkillTerminal(activeSkillRuntime.agentRuntime)
          : ensureOrcaCliAvailableForAgentSkillTerminal())
      }}
      onRecheck={refreshComputerUseSkill}
    />
  )
}

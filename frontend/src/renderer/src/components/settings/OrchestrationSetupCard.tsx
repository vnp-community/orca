import type { JSX } from 'react'
import {
  AGENT_SKILL_CLI_PREREQUISITE_NOTICE,
  ensureOrcaCliAvailableForAgentSkillTerminal
} from '@/lib/agent-skill-cli-prerequisite'
import {
  ORCHESTRATION_SKILL_INSTALL_COMMAND,
  ORCHESTRATION_SKILL_UPDATE_COMMAND
} from '@/lib/orchestration-install-command'
import type { InstalledAgentSkillState } from '@/hooks/useInstalledAgentSkills'
import { useActiveProjectSkillRuntime } from '@/hooks/useActiveProjectSkillRuntime'
import { useActiveDevServer, useConnectedDevServers } from '@/store/slices/dev-servers-selectors'
import { AgentSkillSetupPanel } from './AgentSkillSetupPanel'
import {
  buildSkillCommandForRuntime,
  ensureWslCliAvailableForAgentSkillTerminal,
  getWslCliDistroRequest
} from './CliSkillRuntimeSetup'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import {
  getRuntimeCliInstallStatus,
  getRuntimeWslCliInstallStatus
} from '@/runtime/runtime-cli-client'

export function OrchestrationSetupCard(props: {
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
    ? buildSkillCommandForRuntime(
        ORCHESTRATION_SKILL_INSTALL_COMMAND,
        activeSkillRuntime.agentRuntime
      )
    : ORCHESTRATION_SKILL_INSTALL_COMMAND
  const updateCommand = !activeSkillRuntime.installDisabledReason
    ? buildSkillCommandForRuntime(
        ORCHESTRATION_SKILL_UPDATE_COMMAND,
        activeSkillRuntime.agentRuntime
      )
    : ORCHESTRATION_SKILL_UPDATE_COMMAND

  const setupPanel = (
    <AgentSkillSetupPanel
      className={compact ? 'w-full max-w-[520px]' : undefined}
      title={translate(
        'auto.components.settings.OrchestrationSetupCard.2777ff0fdc',
        'Orchestration skill'
      )}
      description={translate(
        'auto.components.settings.OrchestrationSetupCard.e7d2a5146c',
        'Enables agents to hand off context and coordinate work through Orca.'
      )}
      command={installCommand}
      installedCommand={updateCommand}
      terminalTitle="Orchestration setup"
      terminalAriaLabel="Orchestration skill install terminal"
      terminalWorktreeId="feature-wall-orchestration-skill-terminal"
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
      onBeforeOpenTerminal={async () => {
        useAppStore.getState().recordFeatureInteraction('agent-orchestration-setup')
        await (activeSkillRuntime.agentRuntime?.runtime === 'wsl'
          ? ensureWslCliAvailableForAgentSkillTerminal(activeSkillRuntime.agentRuntime)
          : ensureOrcaCliAvailableForAgentSkillTerminal())
      }}
      onRecheck={skill.refresh}
    />
  )

  if (compact) {
    return <div className="flex min-h-24 flex-1 items-center justify-center">{setupPanel}</div>
  }
  return <div className="flex">{setupPanel}</div>
}

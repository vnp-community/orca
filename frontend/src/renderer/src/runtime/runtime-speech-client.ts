// Routes speech model management (list/download/delete) through the hybrid
// local/remote-environment RPC pattern, mirroring runtime-linear-client.ts and
// runtime-hooks-client.ts. The live dictation session (start/feedAudio/stop +
// onPartialTranscript/onFinalTranscript/onStopped/onError) is deliberately NOT
// wrapped here — see the note above DictationController's speech.dictation.*
// calls for why.
import type { GlobalSettings } from '../../../shared/types'
import type { SpeechModelState } from '../../../shared/speech-types'
import type { RuntimeSpeechSetupState } from '../../../shared/runtime-types'
import { callRuntimeRpc, getActiveRuntimeTarget } from './runtime-rpc-client'

export type RuntimeSpeechSettings = Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined

// Why: the mobile-dictation RPC surface returns setup state (models joined
// with catalog metadata) rather than the desktop's bare model-state array;
// normalize so callers get the same SpeechModelState[] shape either way.
function normalizeRuntimeSpeechModelStates(state: RuntimeSpeechSetupState): SpeechModelState[] {
  return state.models.map((model) => ({
    id: model.id,
    status: model.status,
    ...(model.progress != null ? { progress: model.progress } : {})
  }))
}

export async function speechGetModelStates(
  settings: RuntimeSpeechSettings
): Promise<SpeechModelState[]> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.speech.getModelStates()
  }
  const state = await callRuntimeRpc<RuntimeSpeechSetupState>(target, 'speech.models.list', undefined, {
    timeoutMs: 15_000
  })
  return normalizeRuntimeSpeechModelStates(state)
}

export async function speechDownloadModel(
  settings: RuntimeSpeechSettings,
  modelId: string
): Promise<void> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.speech.downloadModel(modelId)
  }
  // Why: the remote RPC resolves once the download has started (mobile
  // clients poll speech.models.list for progress), unlike the desktop IPC
  // call which resolves on completion. Callers that need live progress on a
  // remote target must poll speechGetModelStates themselves.
  await callRuntimeRpc(target, 'speech.models.download', { modelId }, { timeoutMs: 30_000 })
}

export async function speechDeleteModel(
  settings: RuntimeSpeechSettings,
  modelId: string
): Promise<void> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.speech.deleteModel(modelId)
  }
  await callRuntimeRpc(target, 'speech.models.delete', { modelId }, { timeoutMs: 30_000 })
}

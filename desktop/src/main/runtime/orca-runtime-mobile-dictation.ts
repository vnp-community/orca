/* eslint-disable max-lines -- Why: straight extraction of orca-runtime.ts's
mobile speech/dictation command block (11 methods), already covered by
orca-runtime.ts's own grandfathered max-lines disable before this move.
Registered in config/max-lines-baseline.txt per AGENTS.md — NEEDS PR
REVIEW. Only marginally over budget (304 vs 300 lines). */
// frontend/src/main/runtime/orca-runtime-mobile-dictation.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-055): mobile speech/dictation
// commands extracted from OrcaRuntimeService via the composition pattern.
// Field-span analysis (TASK-BIGFILE-054) confirmed `mobileDictation` and
// these 11 methods are fully self-contained — only `this.store` is a real
// cross-domain dependency.
import { getDefaultVoiceSettings } from '../../shared/constants'
import type { VoiceSettings } from '../../shared/speech-types'
import type { RuntimeSpeechModelSummary, RuntimeSpeechSetupState } from '../../shared/runtime-types'
import { getSpeechModelManager, getSpeechSttService } from '../speech/speech-runtime-service'
import { getCatalogModel, isLocalSpeechModel, SPEECH_MODEL_CATALOG } from '../speech/model-catalog'
import {
  deleteLocalSpeechModel,
  getSpeechModelDeletionErrorCode
} from '../speech/speech-model-deletion'
import type { RuntimeStore } from './orca-runtime'

type MobileDictationSession = {
  id: string
  owner: string
  clientId?: string
  connectionId?: string
  state: 'starting' | 'active' | 'closing'
  partialText: string
  finalTexts: string[]
  errors: string[]
}

export type RuntimeMobileDictationCommandHost = {
  getStore(): RuntimeStore | null
}

export class RuntimeMobileDictationCommands {
  private mobileDictation: MobileDictationSession | null = null

  constructor(private readonly host: RuntimeMobileDictationCommandHost) {}

  // Lists the speech-model catalog joined with live download/ready state, plus
  // the current enabled flag + selected model, so mobile can present a dictation
  // setup sheet and drive remote enable/download. Always targets this (paired)
  // desktop — speech never routes to a worktree's SSH host.
  async listMobileSpeechModels(): Promise<RuntimeSpeechSetupState> {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('voice_dictation_unavailable')
    }
    const voice = store.getSettings().voice ?? getDefaultVoiceSettings()
    const states = await getSpeechModelManager(store).getModelStates()
    const stateById = new Map(states.map((state) => [state.id, state]))
    const models: RuntimeSpeechModelSummary[] = SPEECH_MODEL_CATALOG.map((manifest) => {
      const state = stateById.get(manifest.id)
      return {
        id: manifest.id,
        label: manifest.label,
        provider: manifest.provider === 'openai' ? 'openai' : 'local',
        sizeBytes: manifest.sizeBytes ?? null,
        recommended: manifest.recommended === true,
        status: state?.status ?? 'not-downloaded',
        progress: state?.progress ?? null
      }
    })
    return {
      enabled: voice.enabled === true,
      selectedModelId: voice.sttModel ?? '',
      dictationMode: voice.dictationMode === 'hold' ? 'hold' : 'toggle',
      models
    }
  }

  // Fire-and-forget model download; the ModelManager writes progress into its
  // per-model state, which mobile reads back via listMobileSpeechModels polling.
  async downloadMobileSpeechModel(modelId: string): Promise<{ started: true }> {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('voice_dictation_unavailable')
    }
    const manifest = getCatalogModel(modelId)
    if (!manifest || !isLocalSpeechModel(manifest)) {
      throw new Error('voice_model_not_downloadable')
    }
    // Why: do not await — downloads run for tens of seconds; the call returns
    // immediately and mobile polls for progress/ready.
    void getSpeechModelManager(store)
      .downloadModel(modelId)
      .catch((err) => {
        console.error('[runtime] mobile speech model download failed', { modelId, err })
      })
    return { started: true }
  }

  async deleteMobileSpeechModel(modelId: string): Promise<RuntimeSpeechSetupState> {
    const store = this.host.getStore()
    if (!store?.getSettings || !store.updateSettings) {
      throw new Error('voice_dictation_unavailable')
    }
    try {
      // The runtime store is adapted to the minimal speech settings contract used by deletion.
      await deleteLocalSpeechModel({
        store: {
          getSettings: () => store.getSettings(),
          updateSettings: (updates, options) => store.updateSettings?.(updates, options)
        },
        modelManager: getSpeechModelManager(store),
        sttService: getSpeechSttService(store),
        modelId
      })
    } catch (error) {
      throw new Error(getSpeechModelDeletionErrorCode(error) ?? 'voice_model_delete_failed')
    }
    return this.listMobileSpeechModels()
  }

  // Enables/disables dictation and/or selects the model, merging into the
  // existing voice settings so other voice fields are preserved.
  async configureMobileDictation(params: {
    enabled?: boolean
    modelId?: string
    dictationMode?: 'toggle' | 'hold'
  }): Promise<RuntimeSpeechSetupState> {
    const store = this.host.getStore()
    if (!store?.getSettings || !store.updateSettings) {
      throw new Error('voice_dictation_unavailable')
    }
    const current = store.getSettings().voice ?? getDefaultVoiceSettings()
    // An explicit '' clears the selected model (the OptionalString RPC schema
    // maps '' → undefined, so this only matters for direct callers); any other
    // non-empty modelId must be a known catalog entry.
    if (params.modelId !== undefined && params.modelId !== '' && !getCatalogModel(params.modelId)) {
      throw new Error('voice_model_unknown')
    }
    const nextVoice: VoiceSettings = {
      ...current,
      ...(params.enabled !== undefined ? { enabled: params.enabled } : {}),
      ...(params.modelId !== undefined ? { sttModel: params.modelId } : {}),
      ...(params.dictationMode !== undefined ? { dictationMode: params.dictationMode } : {})
    }
    store.updateSettings({ voice: nextVoice }, { notifyListeners: true })
    return this.listMobileSpeechModels()
  }

  async startMobileDictation(params: {
    dictationId: string
    modelId?: string
    clientId?: string
    connectionId?: string
  }): Promise<{
    dictationId: string
    modelId: string
  }> {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('voice_dictation_unavailable')
    }

    const voice = store.getSettings().voice ?? getDefaultVoiceSettings()
    if (!voice.enabled) {
      throw new Error('voice_dictation_disabled')
    }

    const modelId = params.modelId || voice.sttModel
    if (!modelId) {
      throw new Error('voice_model_not_selected')
    }

    const modelState = await getSpeechModelManager(store).getModelState(modelId)
    if (modelState.status !== 'ready') {
      throw new Error(`voice_model_not_ready:${modelState.status}`)
    }

    if (!params.clientId) {
      throw new Error('dictation_requires_mobile_client')
    }

    if (this.mobileDictation) {
      throw new Error('dictation_already_active')
    }

    const owner = `mobile:${params.dictationId}`
    this.mobileDictation = {
      id: params.dictationId,
      owner,
      clientId: params.clientId,
      connectionId: params.connectionId,
      state: 'starting',
      partialText: '',
      finalTexts: [],
      errors: []
    }

    try {
      await getSpeechSttService(store).startDictation(
        modelId,
        (event) => {
          const session = this.mobileDictation
          if (!session || session.id !== params.dictationId) {
            return
          }
          if (event.type === 'partial') {
            session.partialText = event.text ?? ''
          } else if (event.type === 'final') {
            const text = event.text?.trim()
            if (text) {
              session.finalTexts.push(text)
              session.partialText = ''
            }
          } else if (event.type === 'error') {
            session.errors.push(event.error ?? 'Speech worker error')
          }
        },
        undefined,
        owner
      )
      if (this.mobileDictation?.id !== params.dictationId) {
        throw new Error('dictation_canceled')
      }
      this.mobileDictation.state = 'active'
    } catch (error) {
      if (this.mobileDictation?.id === params.dictationId) {
        this.mobileDictation = null
      }
      throw error
    }

    return { dictationId: params.dictationId, modelId }
  }

  feedMobileDictation(params: {
    dictationId: string
    audioBase64: string
    sampleRate: number
    clientId?: string
    connectionId?: string
  }): {
    dictationId: string
  } {
    const session = this.mobileDictation
    if (!session || session.id !== params.dictationId) {
      throw new Error('dictation_stream_not_started')
    }
    if (!params.clientId || session.clientId !== params.clientId) {
      throw new Error('dictation_owner_mismatch')
    }
    if (session.connectionId && session.connectionId !== params.connectionId) {
      throw new Error('dictation_owner_mismatch')
    }
    if (session.state !== 'active') {
      throw new Error('dictation_stream_closing')
    }
    if (session.errors.length > 0) {
      throw new Error(session.errors[0])
    }

    const pcm = Buffer.from(params.audioBase64, 'base64')
    const samples = new Float32Array(Math.floor(pcm.length / 2))
    for (let i = 0; i < samples.length; i += 1) {
      samples[i] = pcm.readInt16LE(i * 2) / 32768
    }
    getSpeechSttService(this.host.getStore()!).feedAudio(samples, params.sampleRate, session.owner)
    return { dictationId: params.dictationId }
  }

  async finishMobileDictation(params: {
    dictationId: string
    clientId?: string
    connectionId?: string
  }): Promise<{
    dictationId: string
    text: string
  }> {
    const session = this.mobileDictation
    if (!session || session.id !== params.dictationId) {
      throw new Error('dictation_stream_not_started')
    }
    if (!params.clientId || session.clientId !== params.clientId) {
      throw new Error('dictation_owner_mismatch')
    }
    if (session.connectionId && session.connectionId !== params.connectionId) {
      throw new Error('dictation_owner_mismatch')
    }
    session.state = 'closing'
    try {
      await getSpeechSttService(this.host.getStore()!).stopDictation(session.owner)
      if (session.errors.length > 0) {
        throw new Error(session.errors[0])
      }
      const text = [...session.finalTexts, session.partialText].join(' ').trim()
      return { dictationId: params.dictationId, text }
    } finally {
      if (this.mobileDictation?.id === session.id) {
        this.mobileDictation = null
      }
    }
  }

  async cancelMobileDictation(params: {
    dictationId: string
    clientId?: string
    connectionId?: string
  }): Promise<{ dictationId: string }> {
    const session = this.mobileDictation
    if (
      session?.id === params.dictationId &&
      params.clientId &&
      session.clientId === params.clientId &&
      (!session.connectionId || session.connectionId === params.connectionId)
    ) {
      session.state = 'closing'
      try {
        await getSpeechSttService(this.host.getStore()!).stopDictation(session.owner)
      } finally {
        if (this.mobileDictation?.id === session.id) {
          this.mobileDictation = null
        }
      }
    }
    return { dictationId: params.dictationId }
  }

  private cancelMobileDictationSession(session: MobileDictationSession): void {
    if (session.state === 'closing') {
      return
    }
    session.state = 'closing'
    void getSpeechSttService(this.host.getStore()!)
      .stopDictation(session.owner)
      .finally(() => {
        if (this.mobileDictation?.id === session.id) {
          this.mobileDictation = null
        }
      })
  }

  cancelMobileDictationForConnection(connectionId: string): void {
    const session = this.mobileDictation
    if (!session || session.connectionId !== connectionId) {
      return
    }
    this.cancelMobileDictationSession(session)
  }

  // Why: also called from OrcaRuntimeService outside this domain (mobile-floor host wiring, client-disconnect cleanup) — public, not private.
  cancelMobileDictationForClient(clientId: string): void {
    const session = this.mobileDictation
    if (!session || session.clientId !== clientId) {
      return
    }
    this.cancelMobileDictationSession(session)
  }
}

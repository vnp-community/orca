/**
 * useProjectEngineSettings — FE-REQ-TASK-043-05
 *
 * Reads a project's engine settings and saves a new execution mode. The backend takes the whole set on every write (an empty
 * engine becomes `native`, `solutionAgent` is a bare boolean), so a save sends back exactly what the last read returned.
 *
 * @module hooks/useProjectEngineSettings
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import {
  parseEngineSettings,
  type ExecutionModeValue,
  type ProjectEngineSettings
} from '../../../shared/request-run-all'
import { backendErrorCode } from '../components/request/solution/solution-error-classification'
import type { RequestRpcError } from '../../../shared/request-errors'

export type EngineSettingsStatus = 'loading' | 'ready' | 'forbidden' | 'unsupported' | 'error'

/** `conflict`: someone else saved first; the latest values were loaded again. */
export type EngineSettingsSaveOutcome = 'saved' | 'conflict' | 'forbidden' | 'error'

const VERSION_CONFLICT_CODE = 'REQUEST_ENGINE_SETTINGS_VERSION_CONFLICT'

function readFailureStatus(error: RequestRpcError): EngineSettingsStatus {
  // Why: a gateway without the channel, or with request management off, simply has no such setting to show.
  if (error.kind === 'unsupported' || error.kind === 'feature_off') {
    return 'unsupported'
  }
  return error.kind === 'forbidden' ? 'forbidden' : 'error'
}

export function useProjectEngineSettings(projectId: string) {
  const [status, setStatus] = useState<EngineSettingsStatus>('loading')
  const [settings, setSettings] = useState<ProjectEngineSettings | null>(null)
  const [saving, setSaving] = useState(false)
  // Why: a read for a previous project (or an older reload) must never overwrite a newer answer.
  const readToken = useRef(0)
  const savingRef = useRef(false)

  const reload = useCallback(async (): Promise<void> => {
    const token = ++readToken.current
    setStatus('loading')
    const res = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.ENGINE_SETTINGS, { projectId })
    if (token !== readToken.current) {
      return
    }
    if (!res.ok) {
      setSettings(null)
      setStatus(readFailureStatus(res.error))
      return
    }
    const parsed = parseEngineSettings(res.value)
    setSettings(parsed)
    setStatus(parsed ? 'ready' : 'error')
  }, [projectId])

  const dropPendingRead = useCallback(() => {
    readToken.current++
  }, [])

  useEffect(() => {
    void reload()
    return dropPendingRead
  }, [reload, dropPendingRead])

  const saveExecutionMode = useCallback(
    async (executionMode: ExecutionModeValue): Promise<EngineSettingsSaveOutcome> => {
      if (!settings || savingRef.current) {
        return 'error'
      }
      savingRef.current = true
      setSaving(true)
      try {
        const res = await callRequestRpc<unknown>(REQUEST_RPC_METHODS.ENGINE_SETTINGS_SET, {
          projectId,
          engine: settings.engine,
          openspecMinVersion: settings.openspecMinVersion,
          solutionAgent: settings.solutionAgent,
          executionMode,
          expectedVersion: settings.version
        })
        if (!res.ok) {
          if (backendErrorCode(res.error) === VERSION_CONFLICT_CODE) {
            await reload()
            return 'conflict'
          }
          return res.error.kind === 'forbidden' ? 'forbidden' : 'error'
        }
        const saved = parseEngineSettings(res.value)
        if (saved) {
          readToken.current++
          setSettings(saved)
          setStatus('ready')
        } else {
          // The write went through but the answer was not readable; show what is stored instead of guessing.
          await reload()
        }
        return 'saved'
      } finally {
        savingRef.current = false
        setSaving(false)
      }
    },
    [projectId, reload, settings]
  )

  return { status, settings, saving, reload, saveExecutionMode }
}

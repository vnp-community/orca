// TASK-FE-024: useSshUserAccount
import { useEffect, useState } from 'react'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'
import { toLinuxUsername } from '../auth/auth-utils'

type Result = {
  linuxUsername: string | null
  previewUsername: string | null   // computed from email before provisioned
  provisioned: boolean
  isLoading: boolean
  error: string | null
}

export function useSshUserAccount(
  serverId: string,
  options?: { previewFromEmail?: string }
): Result {
  const [linuxUsername, setLinuxUsername] = useState<string | null>(null)
  const [provisioned, setProvisioned] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const previewUsername = options?.previewFromEmail
    ? toLinuxUsername(options.previewFromEmail)
    : null

  useEffect(() => {
    let cancelled = false
    callRuntimeRpc({ kind: 'local' }, 'ssh.getUserAccount', { serverId })
      .then((result: { linuxUsername: string; provisioned: boolean }) => {
        if (!cancelled) {
          setLinuxUsername(result.linuxUsername)
          setProvisioned(result.provisioned)
        }
      })
      .catch((err: Error) => { if (!cancelled) setError(err.message) })
      .finally(() => { if (!cancelled) setIsLoading(false) })
    return () => { cancelled = true }
  }, [serverId])

  return { linuxUsername, previewUsername, provisioned, isLoading, error }
}

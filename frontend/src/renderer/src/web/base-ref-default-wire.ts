import type { BaseRefDefaultResult } from '../../../shared/types'

type BaseRefDefaultWire = { ref?: string; defaultBaseRef?: string | null; remoteCount?: number }

/**
 * The gateway answers `repo.baseRefDefault` with `{ref}` (the git-gateway proto field); the screens read `defaultBaseRef`.
 * Without this mapping the "Start from" button never learned the default branch's name.
 */
export function baseRefDefaultFromWire(
  wire: BaseRefDefaultWire | null | undefined
): BaseRefDefaultResult {
  return {
    defaultBaseRef: wire?.defaultBaseRef ?? (wire?.ref ? wire.ref : null),
    remoteCount: wire?.remoteCount ?? 0
  }
}

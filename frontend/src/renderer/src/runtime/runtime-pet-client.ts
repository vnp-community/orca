// Why: custom pet sprites live on the desktop filesystem (userData/sidekicks)
// — always local, no remote-environment routing.
import type { CustomPet } from '../../../shared/types'
import { callRuntimeRpc } from './runtime-rpc-client'

const LOCAL_TARGET = { kind: 'local' } as const

export function importRuntimePet(): Promise<CustomPet | null> {
  return callRuntimeRpc<CustomPet | null>(LOCAL_TARGET, 'pet.import')
}

export function importRuntimePetBundle(): Promise<CustomPet | null> {
  return callRuntimeRpc<CustomPet | null>(LOCAL_TARGET, 'pet.importPetBundle')
}

export function readRuntimePetFile(
  id: string,
  fileName: string,
  kind?: 'image' | 'bundle'
): Promise<ArrayBuffer | null> {
  return callRuntimeRpc<ArrayBuffer | null>(LOCAL_TARGET, 'pet.read', { id, fileName, kind })
}

export function deleteRuntimePetFile(
  id: string,
  fileName: string,
  kind?: 'image' | 'bundle'
): Promise<void> {
  return callRuntimeRpc<void>(LOCAL_TARGET, 'pet.delete', { id, fileName, kind })
}

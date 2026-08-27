import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import { deletePetFile, importPet, importPetBundle, readPetFile } from '../../../ipc/pet'

const PetFileArgs = z.object({
  id: z.string(),
  fileName: z.string(),
  kind: z.enum(['image', 'bundle']).optional()
})

export const PET_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'pet.import',
    params: null,
    // Why: no webContents/event to derive a parent window from over RPC —
    // importPet() falls back to BrowserWindow.getFocusedWindow(), same as the
    // ipc handler does when event.sender doesn't resolve to a window.
    handler: () => importPet()
  }),
  defineMethod({
    name: 'pet.importPetBundle',
    params: null,
    handler: () => importPetBundle()
  }),
  defineMethod({
    name: 'pet.read',
    params: PetFileArgs,
    handler: (params) => readPetFile(params.id, params.fileName, params.kind)
  }),
  defineMethod({
    name: 'pet.delete',
    params: PetFileArgs,
    handler: (params) => deletePetFile(params.id, params.fileName, params.kind)
  })
]

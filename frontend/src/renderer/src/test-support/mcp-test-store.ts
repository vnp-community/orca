// Tests-only zustand store built from the REAL mcp slices, for `vi.mock('@/store', ...)`.
import { create } from 'zustand'
import { createMcpApprovalSlice } from '../store/slices/mcp-approval-slice'
import { createMcpSlice } from '../store/slices/mcp-slice'

type AnyCreator = (...a: unknown[]) => object

export function createMcpTestStore() {
  return create((...a: unknown[]) => ({
    ...(createMcpSlice as unknown as AnyCreator)(...a),
    ...(createMcpApprovalSlice as unknown as AnyCreator)(...a),
    currentUser: null as { id: string; role: string } | null,
    openSettingsPage: () => {},
    openSettingsTarget: () => {}
  }))
}

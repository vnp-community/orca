/**
 * Minimal zustand store carrying only the code-intel slice, for hook tests that mock '@/store'
 * (importing the real app store would pull the whole renderer into the test).
 */
import { create } from 'zustand'
import { createCodeIntelSlice } from '../store/slices/code-intel'
import type { CodeIntelSlice } from '../store/slices/code-intel'

export function createCodeIntelTestStore() {
  return create<CodeIntelSlice>()((set, get) => createCodeIntelSlice(set as never, get as never))
}

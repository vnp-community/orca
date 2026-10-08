/**
 * useDiffSectionModelDisposal.ts
 *
 * Disposes the detached Monaco models of one combined-diff section when its row unmounts or
 * the section collapses. Extracted from DiffSectionItem unchanged.
 *
 * @module components/editor/useDiffSectionModelDisposal
 */

import { useCallback, useEffect, useRef } from 'react'
import { monaco } from '@/lib/monaco-setup'
import { disposeUnattachedMonacoModelPaths } from './diff-monaco-model-disposal'

export function useDiffSectionModelDisposal(
  modelPathBase: string,
  collapsed: boolean | undefined
): { disposeDiffModels: () => void; setSectionRootNode: (node: HTMLDivElement | null) => void } {
  const disposeDiffModels = useCallback(() => {
    window.setTimeout(() => {
      disposeUnattachedMonacoModelPaths(monaco, [
        `${modelPathBase}:original`,
        `${modelPathBase}:modified`
      ])
    }, 0)
  }, [modelPathBase])
  const disposeDiffModelsRef = useRef(disposeDiffModels)
  disposeDiffModelsRef.current = disposeDiffModels

  const setSectionRootNode = useCallback((node: HTMLDivElement | null): void => {
    if (node) {
      return
    }
    // Why: virtualized diff rows remount as their keyed section/collapse state
    // changes; the row root is the owner of the detached Monaco models.
    disposeDiffModelsRef.current()
  }, [])

  useEffect(() => {
    if (collapsed) {
      disposeDiffModels()
    }
  }, [disposeDiffModels, collapsed])

  return { disposeDiffModels, setSectionRootNode }
}

import { useState } from 'react'
import { toast } from 'sonner'
import { Github, Image, Link2 } from 'lucide-react'
import type { RepoIcon } from '../../../../shared/repo-icon'
import { faviconUrlFromWebsite, MAX_REPO_ICON_UPLOAD_BYTES } from '../../../../shared/repo-icon'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/tabs'
import { Tooltip, TooltipContent, TooltipTrigger } from '../ui/tooltip'
import { getRepoLucideIconOptions } from '../repo/repo-icon'
import { useMountedRef } from '@/hooks/useMountedRef'
import { translate } from '@/i18n/i18n'
import { isWebClientLocation } from '../../lib/web-client-location'
import { useActiveDevServer } from '../../store/slices/dev-servers-selectors'
import { DevServerFilePickerDialog } from '../remote-browser/DevServerFilePickerDialog'
import { devServerReadFile } from '../../runtime/runtime-dev-server-shell-client'

import { shellPickRepoIconImage } from '../../runtime/runtime-shell-client'
const EMOJI_OPTIONS = ['🚀', '✨', '💻', '🧠', '📦', '🔧', '🎨', '🌐', '📊', '🔒', '⚡', '✅']

type RepositoryIconTabsProps = {
  initialTab: 'avatar' | 'icon' | 'emoji'
  selectedLucideName: string | null
  selectedEmoji: string
  loadingGitHub: boolean
  onSetIcon: (repoIcon: RepoIcon | null) => void
  onUseGitHubAvatar: () => void
}

export function RepositoryIconTabs({
  initialTab,
  selectedLucideName,
  selectedEmoji,
  loadingGitHub,
  onSetIcon,
  onUseGitHubAvatar
}: RepositoryIconTabsProps): React.JSX.Element {
  const [website, setWebsite] = useState('')
  const [pickerOpen, setPickerOpen] = useState(false)
  const mountedRef = useMountedRef()
  const activeDevServer = useActiveDevServer()

  const handleUploadImage = async () => {
    // Why: there is no OS-native file dialog in server/web mode — browse the
    // connected Dev Server's filesystem instead (see DevServerFilePickerDialog).
    if (isWebClientLocation()) {
      if (!activeDevServer) {
        toast.error('Connect a Dev Server to browse its files.')
        return
      }
      setPickerOpen(true)
      return
    }
    try {
      const result = await shellPickRepoIconImage()
      if (!result || !mountedRef.current) {
        return
      }
      onSetIcon({
        type: 'image',
        src: result.dataUrl,
        source: 'upload',
        label: result.fileName
      })
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : translate(
              'auto.components.settings.RepositoryIconPicker.868c5c9b56',
              'Failed to import repo icon'
            )
      )
    }
  }

  const handlePickerSelect = async (path: string): Promise<void> => {
    setPickerOpen(false)
    if (!activeDevServer) {
      return
    }
    try {
      if (!path.toLowerCase().endsWith('.png')) {
        throw new Error('Repo icons must be PNG files.')
      }
      const file = await devServerReadFile(activeDevServer.id, path)
      if (!mountedRef.current) {
        return
      }
      const byteLength =
        file.encoding === 'base64' ? atob(file.content).length : file.content.length
      if (byteLength > MAX_REPO_ICON_UPLOAD_BYTES) {
        throw new Error('Repo icon image must be 256KB or smaller.')
      }
      const contentBase64 = file.encoding === 'base64' ? file.content : btoa(file.content)
      const fileName = path.split('/').pop() || 'icon.png'
      onSetIcon({
        type: 'image',
        src: `data:image/png;base64,${contentBase64}`,
        source: 'upload',
        label: fileName
      })
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : translate(
              'auto.components.settings.RepositoryIconPicker.868c5c9b56',
              'Failed to import repo icon'
            )
      )
    }
  }

  const handleUseWebsiteFavicon = () => {
    const src = faviconUrlFromWebsite(website)
    if (!src) {
      toast.error(
        translate(
          'auto.components.settings.RepositoryIconPicker.acf31559a0',
          'Enter a valid website URL.'
        )
      )
      return
    }
    onSetIcon({
      type: 'image',
      src,
      source: 'favicon',
      label: translate(
        'auto.components.settings.RepositoryIconPicker.4d039317f4',
        'Website favicon'
      )
    })
  }

  return (
    <>
      <Tabs defaultValue={initialTab} className="gap-3">
        <TabsList variant="line" className="h-8">
          <TabsTrigger value="avatar" className="h-7 text-xs">
            {translate('auto.components.settings.RepositoryIconPicker.2d8bd302fa', 'Avatar')}
          </TabsTrigger>
          <TabsTrigger value="icon" className="h-7 text-xs">
            {translate('auto.components.settings.RepositoryIconPicker.b2d7fd2116', 'Icon')}
          </TabsTrigger>
          <TabsTrigger value="emoji" className="h-7 text-xs">
            {translate('auto.components.settings.RepositoryIconPicker.c490787d24', 'Emoji')}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="avatar" className="space-y-3">
          <Button
            type="button"
            variant="default"
            className="w-full gap-2"
            disabled={loadingGitHub}
            onClick={() => void onUseGitHubAvatar()}
          >
            <Github className="size-3.5" />
            {translate(
              'auto.components.settings.RepositoryIconPicker.39da8a10bf',
              'Use GitHub Avatar'
            )}
          </Button>
          <p className="text-xs text-muted-foreground">
            {translate(
              'auto.components.settings.RepositoryIconPicker.7da623abcc',
              "Used by default — GitHub always provides one, even when the owner hasn't set a custom image."
            )}
          </p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="gap-2"
            onClick={() => void handleUploadImage()}
          >
            <Image className="size-3.5" />
            {translate('auto.components.settings.RepositoryIconPicker.381b4844fd', 'Upload PNG')}
          </Button>
          <div className="flex gap-2">
            <Input
              value={website}
              onChange={(event) => setWebsite(event.target.value)}
              placeholder={translate(
                'auto.components.settings.RepositoryIconPicker.03ca1a4e9b',
                'example.com'
              )}
              className="h-9 text-sm"
            />
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-9 gap-2"
              onClick={handleUseWebsiteFavicon}
            >
              <Link2 className="size-3.5" />
              {translate('auto.components.settings.RepositoryIconPicker.cc1286e263', 'Favicon')}
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">
            {translate(
              'auto.components.settings.RepositoryIconPicker.fde066a63b',
              'PNG uploads must be 256KB or smaller.'
            )}
          </p>
        </TabsContent>

        <TabsContent value="icon" className="space-y-3">
          <div className="grid grid-cols-10 gap-1.5">
            {getRepoLucideIconOptions().map((option) => (
              <Tooltip key={option.name}>
                <TooltipTrigger asChild>
                  <Button
                    type="button"
                    variant={selectedLucideName === option.name ? 'secondary' : 'ghost'}
                    size="icon-xs"
                    className="size-8"
                    onClick={() => onSetIcon({ type: 'lucide', name: option.name })}
                    aria-label={translate(
                      'auto.components.settings.RepositoryIconPicker.2b7d27b93c',
                      'Use {{value0}} repo icon',
                      { value0: option.label }
                    )}
                  >
                    <option.icon className="size-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="top" sideOffset={4}>
                  {option.label}
                </TooltipContent>
              </Tooltip>
            ))}
          </div>
        </TabsContent>

        <TabsContent value="emoji" className="grid grid-cols-12 gap-1.5">
          {EMOJI_OPTIONS.map((emoji) => (
            <Button
              key={emoji}
              type="button"
              variant={selectedEmoji === emoji ? 'secondary' : 'ghost'}
              size="icon-xs"
              className="size-8 text-base"
              onClick={() => onSetIcon({ type: 'emoji', emoji })}
              aria-label={translate(
                'auto.components.settings.RepositoryIconPicker.2b7d27b93c',
                'Use {{value0}} repo icon',
                { value0: emoji }
              )}
            >
              {emoji}
            </Button>
          ))}
        </TabsContent>
      </Tabs>
      <DevServerFilePickerDialog
        open={pickerOpen}
        mode="file"
        extensions={['png']}
        title="Choose a repo icon"
        description="PNG uploads must be 256KB or smaller."
        onSelect={(path) => void handlePickerSelect(path)}
        onClose={() => setPickerOpen(false)}
      />
    </>
  )
}

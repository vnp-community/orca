// UserProfileBadge — user identity button with dropdown in titlebar (CR-006, TASK-006-B)
// Why: avatar.tsx UI component does not exist in this codebase — using initials div fallback.
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'

function getInitials(name: string): string {
  return name
    .split(' ')
    .map((n) => n[0] ?? '')
    .join('')
    .toUpperCase()
    .slice(0, 2)
}

export function UserProfileBadge(): React.JSX.Element | null {
  const user = useAppStore((s) => s.currentUser)
  if (!user) {return null}

  const initials = getInitials(user.name)

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="sm" className="h-7 gap-1.5 px-2 text-xs">
          {/* Initials avatar — avatar.tsx not available in this codebase */}
          <span className="flex size-5 items-center justify-center rounded-full bg-primary/20 text-[10px] font-semibold text-primary">
            {initials}
          </span>
          <span className="max-w-[80px] truncate">{user.name}</span>
        </Button>
      </DropdownMenuTrigger>

      <DropdownMenuContent align="end" className="w-[200px]">
        <DropdownMenuLabel className="font-normal">
          <div className="flex flex-col space-y-0.5">
            <p className="text-sm font-medium">{user.name}</p>
            <p className="text-xs text-muted-foreground">{user.email}</p>
          </div>
        </DropdownMenuLabel>

        <DropdownMenuSeparator />

        {/* Teams badges */}
        {user.teams.length > 0 && (
          <>
            <DropdownMenuLabel className="text-xs text-muted-foreground">
              {translate('user.teams', 'Teams')}
            </DropdownMenuLabel>
            <div className="flex flex-wrap gap-1 px-2 pb-1">
              {user.teams.map((team) => (
                <Badge key={team} variant="secondary" className="text-xs">
                  {team}
                </Badge>
              ))}
            </div>
            <DropdownMenuSeparator />
          </>
        )}

        <DropdownMenuItem
          className="text-destructive focus:text-destructive"
          onClick={() => void window.api.auth?.signOut?.()}
        >
          {translate('user.signOut', 'Sign out')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

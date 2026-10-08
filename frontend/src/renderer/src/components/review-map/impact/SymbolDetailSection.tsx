import { ChevronDown, ChevronRight } from "lucide-react";

type Props = {
  title: string;
  open: boolean;
  onToggle: () => void;
  count?: number;
  children: React.ReactNode;
};

/** Disclosure section of the symbol panel (collapsed sections load nothing). */
export function SymbolDetailSection({
  title,
  open,
  onToggle,
  count,
  children,
}: Props): React.JSX.Element {
  const Chevron = open ? ChevronDown : ChevronRight;
  return (
    <section className="border-t py-2">
      <button
        type="button"
        aria-expanded={open}
        onClick={onToggle}
        className="flex w-full items-center gap-1 rounded text-left text-xs font-medium hover:bg-accent"
      >
        <Chevron className="size-3.5 shrink-0" aria-hidden />
        <span>{title}</span>
        {count !== undefined ? (
          <span className="ml-auto tabular-nums text-muted-foreground">
            {count}
          </span>
        ) : null}
      </button>
      {open ? <div className="mt-2">{children}</div> : null}
    </section>
  );
}

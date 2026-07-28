import { useMemo, useState } from "react";
import * as Popover from "@radix-ui/react-popover";
import { Check, ChevronDown } from "lucide-react";
import { cn } from "@/lib/utils";

export type SelectOption = { value: string; label: string; disabled?: boolean };

export function StyledSelect({ value, onChange, options, placeholder, ariaLabel, disabled = false, className }: {
  value: string;
  onChange: (value: string) => void;
  options: SelectOption[];
  placeholder: string;
  ariaLabel: string;
  disabled?: boolean;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const selected = useMemo(() => options.find((option) => option.value === value), [options, value]);

  return <Popover.Root open={open} onOpenChange={setOpen}>
    <Popover.Trigger asChild>
      <button type="button" aria-label={ariaLabel} aria-expanded={open} disabled={disabled} className={cn("select-trigger", className)}>
        <span className="truncate">{selected?.label ?? placeholder}</span>
        <ChevronDown className="size-4 shrink-0 text-muted-foreground transition-transform data-[state=open]:rotate-180" />
      </button>
    </Popover.Trigger>
    <Popover.Portal>
      <Popover.Content align="start" sideOffset={6} className="select-menu">
        <div className="max-h-64 overflow-y-auto p-1">
          {options.map((option) => <button key={option.value} type="button" disabled={option.disabled} onClick={() => { onChange(option.value); setOpen(false); }} className="select-option" data-selected={option.value === value}>
            <span className="min-w-0 flex-1 truncate">{option.label}</span>
            <Check className={option.value === value ? "size-4 opacity-100" : "size-4 opacity-0"} />
          </button>)}
        </div>
      </Popover.Content>
    </Popover.Portal>
  </Popover.Root>;
}

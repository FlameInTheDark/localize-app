import { useMemo, useState } from "react";
import * as Popover from "@radix-ui/react-popover";
import { Command } from "cmdk";
import { Check, ChevronsUpDown, Globe2, Plus, Search } from "lucide-react";
import { cn } from "@/lib/utils";
import { languages, type Language } from "@/lib/languages";

const popularCodes = new Set(["en", "es", "fr", "de", "it", "pt", "ru", "ja", "ko", "zh", "ar", "hi"]);

export function LanguageSelect({ value, onChange, label = "Target language", className, allowAuto = false, allowCustom = true, autoLabel = "Automatic" }: { value: string; onChange: (value: string) => void; label?: string; className?: string; allowAuto?: boolean; allowCustom?: boolean; autoLabel?: string }) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const selected = languages.find((language) => language.code === value);
  const automatic = allowAuto && value === "auto";
  const normalizedQuery = query.trim().toLowerCase();
  const matches = useMemo(() => !normalizedQuery ? languages : languages.filter((language) => `${language.name} ${language.native ?? ""} ${language.code}`.toLowerCase().includes(normalizedQuery)), [normalizedQuery]);
  const popular = useMemo(() => languages.filter((language) => popularCodes.has(language.code)), []);
  const isKnownCode = (allowAuto && normalizedQuery === "auto") || languages.some((language) => language.code === normalizedQuery);
  const choose = (code: string) => { onChange(code); setOpen(false); setQuery(""); };

  return <Popover.Root open={open} onOpenChange={setOpen}>
    <Popover.Trigger asChild>
      <button type="button" role="combobox" aria-label={label} aria-expanded={open} className={cn("language-trigger language-trigger-ghost", className)}>
        <span className="flex min-w-0 items-center gap-2"><Globe2 className="size-4 shrink-0 text-muted-foreground" /><span className="truncate">{automatic ? autoLabel : selected?.name ?? "Select language"}</span>{(automatic || selected) && <span className="language-code">{automatic ? "auto" : selected?.code}</span>}</span>
        <ChevronsUpDown className="size-4 shrink-0 text-muted-foreground opacity-70" />
      </button>
    </Popover.Trigger>
    <Popover.Portal>
      <Popover.Content align="start" sideOffset={6} className="language-menu" onOpenAutoFocus={(event) => event.preventDefault()}>
        <Command shouldFilter={false} loop className="overflow-hidden rounded-[inherit]">
          <div className="flex h-10 items-center gap-2 border-b px-3"><Search className="size-4 shrink-0 text-muted-foreground" /><Command.Input autoFocus value={query} onValueChange={setQuery} placeholder="Search language or ISO code…" className="h-full w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground" /></div>
          <Command.List className="max-h-72 overflow-y-auto p-1">
            {allowAuto && <Command.Group className="language-group"><Command.Item value="automatic auto" onSelect={() => choose("auto")} className="language-option"><Check className={automatic ? "size-4 opacity-100" : "size-4 opacity-0"} /><span className="min-w-0 flex-1 truncate">{autoLabel}</span><span className="language-code">auto</span></Command.Item></Command.Group>}
            {!normalizedQuery && <Command.Group className="language-group"><div className="language-group-label">Popular</div>{popular.map((language) => <LanguageItem key={`popular-${language.code}`} language={language} selected={language.code === value} onSelect={choose} />)}</Command.Group>}
            <Command.Group className="language-group"><div className="language-group-label">{normalizedQuery ? "Results" : "All languages"}</div>{matches.map((language) => <LanguageItem key={language.code} language={language} selected={language.code === value} onSelect={choose} />)}</Command.Group>
            {normalizedQuery && matches.length === 0 && <Command.Empty className="px-3 py-6 text-center text-sm text-muted-foreground">No matching language.</Command.Empty>}
            {allowCustom && normalizedQuery && !isKnownCode && <Command.Group className="language-group"><div className="language-group-label">Custom</div><Command.Item value={`custom-${normalizedQuery}`} onSelect={() => choose(normalizedQuery)} className="language-option"><Plus className="size-4 text-muted-foreground" /><span className="min-w-0 flex-1 truncate">Use <b className="font-mono">{normalizedQuery}</b> as language code</span></Command.Item></Command.Group>}
          </Command.List>
        </Command>
      </Popover.Content>
    </Popover.Portal>
  </Popover.Root>;
}

function LanguageItem({ language, selected, onSelect }: { language: Language; selected: boolean; onSelect: (code: string) => void }) {
  return <Command.Item value={`${language.name} ${language.native ?? ""} ${language.code}`} onSelect={() => onSelect(language.code)} className="language-option"><Check className={selected ? "size-4 opacity-100" : "size-4 opacity-0"} /><span className="min-w-0 flex-1 truncate"><span>{language.name}</span>{language.native && language.native !== language.name && <span className="text-muted-foreground"> · {language.native}</span>}</span><span className="language-code">{language.code}</span></Command.Item>;
}

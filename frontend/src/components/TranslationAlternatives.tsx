import { useMemo, useRef, useState } from "react";
import * as Popover from "@radix-ui/react-popover";
import { Loader2, X } from "lucide-react";
import type { TranslationVariant } from "@/types/api";
import { api } from "@/lib/bridge";
import { locateReplacement, tokenizeTranslation, variantContext, type TextSegment } from "@/lib/textVariants";

type ResolvedVariant = TranslationVariant & { range: { start: number; end: number } };
type ActiveWord = { selected: TextSegment; loading: boolean; variants: ResolvedVariant[]; error: string };

export function TranslationAlternatives({ text, sourceText, language, onReplace }: { text: string; sourceText: string; language: string; onReplace: (range: { start: number; end: number }, replacement: string) => void }) {
  const segments = useMemo(() => tokenizeTranslation(text, language), [text, language]);
  const [active, setActive] = useState<ActiveWord | null>(null);
  const requestID = useRef(0);

  const openAlternatives = (selected: TextSegment) => {
    const id = requestID.current + 1;
    requestID.current = id;
    const context = variantContext(text, language, selected);
    setActive({ selected, loading: true, variants: [], error: "" });
    void api.variants({ sourceText, ...context, selectedText: selected.text, language }).then((result) => {
      if (requestID.current !== id) return;
      const variants = (Array.isArray(result.variants) ? result.variants : []).flatMap((variant) => {
        const range = locateReplacement(text, selected, variant.target);
        return range ? [{ ...variant, range }] : [];
      });
      setActive({ selected, loading: false, variants, error: variants.length ? "" : "No usable alternatives were returned for this word." });
    }).catch((reason: unknown) => {
      if (requestID.current !== id) return;
      setActive({ selected, loading: false, variants: [], error: reason instanceof Error ? reason.message : "Could not get alternatives." });
    });
  };

  const close = () => { requestID.current += 1; setActive(null); };

  return <>{segments.map((segment) => {
    if (!segment.wordLike) return <span key={`${segment.start}-${segment.end}`}>{segment.text}</span>;
    const isOpen = active?.selected.start === segment.start && active.selected.end === segment.end;
    return <Popover.Root key={`${segment.start}-${segment.end}`} open={isOpen} onOpenChange={(open) => { if (!open) close(); }}><Popover.Trigger asChild><button type="button" className="translation-token" onClick={() => openAlternatives(segment)} aria-label={`Show alternatives for ${segment.text}`}>{segment.text}</button></Popover.Trigger><Popover.Portal>{isOpen && active && <Popover.Content className="alternatives-popover" align="start" sideOffset={6} onOpenAutoFocus={(event) => event.preventDefault()}><header className="alternatives-header"><span>Alternatives</span><button type="button" onClick={close} className="alternatives-close" aria-label="Close alternatives"><X className="size-4" /></button></header><div className="alternatives-list">{active.loading ? <div className="alternatives-state"><Loader2 className="size-4 animate-spin" />Finding alternatives…</div> : active.error ? <div className="alternatives-state text-destructive">{active.error}</div> : active.variants.map((variant, index) => <button key={`${variant.target}-${variant.replacement}-${index}`} type="button" className="alternative-option" onClick={() => { onReplace(variant.range, variant.replacement); close(); }}><span className="block font-medium">{variant.replacement}</span>{variant.target !== active.selected.text && <span className="mt-0.5 block truncate text-xs text-muted-foreground">Replaces “{variant.target}”</span>}</button>)}</div></Popover.Content>}</Popover.Portal></Popover.Root>;
  })}</>;
}

import { FileText, ImagePlus, X } from "lucide-react";
import type { FileSelection } from "@/types/api";
import { bytes } from "@/lib/utils";

export function FilePicker({ kind, selection, onPick, onClear }: { kind: "image" | "document"; selection: FileSelection | null; onPick: () => void; onClear: () => void }) {
  const isImage = kind === "image";
  if (selection) return <div className="flex items-center gap-3 rounded-lg border border-dashed p-3"><span className="flex size-9 items-center justify-center rounded-md bg-muted">{isImage ? <ImagePlus className="size-4" /> : <FileText className="size-4" />}</span><div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{selection.name}</p><p className="text-xs text-muted-foreground">{bytes(selection.size)}</p></div><button onClick={onClear} className="app-button app-button-outline" aria-label="Clear file"><X className="size-4" /></button></div>;
  return <button onClick={onPick} className="flex min-h-36 w-full flex-col items-center justify-center rounded-lg border border-dashed p-5 text-center transition hover:bg-muted/60"><span className="mb-2 flex size-10 items-center justify-center rounded-lg bg-muted">{isImage ? <ImagePlus className="size-5" /> : <FileText className="size-5" />}</span><span className="text-sm font-medium">Choose a {isImage ? "image" : "document"}</span><span className="mt-1 text-xs text-muted-foreground">{isImage ? "PNG, JPEG, WebP, GIF, BMP · up to 20 MB" : "PDF, EPUB, MOBI, DOCX, XLSX, PPTX · up to 100 MB"}</span></button>;
}

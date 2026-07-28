import { useEffect, useState } from "react";
import { Check, Copy, Loader2, Send, ScanText } from "lucide-react";
import type { FileSelection } from "@/types/api";
import { api } from "@/lib/bridge";
import { FilePicker } from "./FilePicker";
import { Empty, ErrorMessage, Loading } from "./ImageTranslator";

export function OCRExtractor({ dropPath, onSend }: { dropPath?: string; onSend: (text: string) => void }) {
  const [file, setFile] = useState<FileSelection | null>(null); const [text, setText] = useState(""); const [busy, setBusy] = useState(false); const [error, setError] = useState(""); const [copied, setCopied] = useState(false);
  const select = async (path?: string) => { try { const value = path ? await api.loadFile(path, "image") : await api.pickFile("image"); if (!value.path) return; setFile(value); setText(""); setError(""); } catch (reason) { setError(reason instanceof Error ? reason.message : "Could not read image"); } };
  useEffect(() => { if (dropPath) void select(dropPath); }, [dropPath]);
  const extract = async () => { if (!file) return; setBusy(true); setError(""); try { setText(await api.ocr(file.path)); } catch (reason) { setError(reason instanceof Error ? reason.message : "OCR extraction failed"); } finally { setBusy(false); } };
  const copy = async () => { if (!text) return; await navigator.clipboard.writeText(text); setCopied(true); setTimeout(() => setCopied(false), 1500); };
  return <div className="desktop-workspace h-full min-h-[30rem] grid-cols-1 md:grid-cols-[.9fr_1.1fr]"><section className="desktop-pane flex min-h-72 flex-col p-4"><FilePicker kind="image" selection={file} onPick={() => void select()} onClear={() => { setFile(null); setText(""); }} />{file?.previewUrl && <img src={file.previewUrl} alt="Selected image" className="mt-4 min-h-0 flex-1 rounded-lg border object-contain" />}</section><section className="desktop-pane flex min-h-72 flex-col"><header className="flex h-11 items-center justify-between gap-2 border-b px-2"><span className="text-sm font-medium">Extracted Text</span><div className="flex gap-1"><button onClick={copy} disabled={!text} className="app-button app-button-secondary">{copied ? <Check className="size-4" /> : <Copy className="size-4" />}</button><button onClick={() => onSend(text)} disabled={!text} className="app-button app-button-secondary"><Send className="size-4" />Send</button><button onClick={extract} disabled={!file || busy} className="app-button app-button-primary">{busy ? <Loader2 className="size-4 animate-spin" /> : <ScanText className="size-4" />}{busy ? "Extracting" : "Extract"}</button></div></header><div className="min-h-52 flex-1 overflow-auto p-4 whitespace-pre-wrap break-words text-sm leading-relaxed">{busy ? <Loading label="Reading the image…" /> : error ? <ErrorMessage message={error} /> : text || <Empty text="Upload an image to extract text." />}</div></section></div>;
}

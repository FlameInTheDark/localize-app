import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { CheckCircle2, FileText, Languages, Loader2, RotateCcw, Save, Upload, X } from "lucide-react";
import { EventsOff, EventsOn } from "../../wailsjs/runtime/runtime";
import { api } from "@/lib/bridge";
import { bytes, cn } from "@/lib/utils";
import { LanguageSelect } from "./LanguageSelect";
import { StyledSelect } from "./StyledSelect";
import type { FileSelection, LocalizationEntry, LocalizationFile, LocalizationFormat, LocalizationProgress, UntranslatedExportMode } from "@/types/api";

type RowStatus = "idle" | "translating" | "translated" | "failed" | "edited";
type LocalRow = LocalizationEntry & { status: RowStatus; error?: string };
type PendingChange = { type: "language"; value: string } | { type: "format"; value: LocalizationFormat };

const formatOptions = [
  { value: "auto", label: "Auto-detect from file extension" }, { value: "i18next-json", label: "i18next JSON (.json)" },
  { value: "yaml", label: "YAML (.yaml, .yml)" }, { value: "properties", label: "Java properties (.properties)" },
  { value: "gettext-po", label: "Gettext / Unreal PO (.po)" }, { value: "source-keyvalues", label: "Source KeyValues (.txt, .vdf)" },
];

export function LocalizationTranslator({ dropPath }: { dropPath?: string }) {
  const [format, setFormat] = useState<LocalizationFormat>("auto");
  const [file, setFile] = useState<FileSelection | null>(null);
  const [loaded, setLoaded] = useState<LocalizationFile | null>(null);
  const [rows, setRows] = useState<LocalRow[]>([]);
  const [language, setLanguage] = useState("en");
  const [busy, setBusy] = useState(false);
  const [progress, setProgress] = useState<LocalizationProgress | null>(null);
  const [error, setError] = useState("");
  const [savedPath, setSavedPath] = useState("");
  const [savePending, setSavePending] = useState(false);
  const [pendingChange, setPendingChange] = useState<PendingChange | null>(null);
  const operationID = useRef("");

  const reset = useCallback(() => { setFile(null); setLoaded(null); setRows([]); setProgress(null); setError(""); setSavedPath(""); operationID.current = ""; }, []);
  const applyEntry = useCallback((entry: LocalizationEntry, status: RowStatus, rowError?: string) => setRows((current) => current.map((row) => row.id === entry.id ? { ...row, ...normalizeEntry(entry), status, error: rowError } : row)), []);

  const load = useCallback(async (path: string, requestedFormat: LocalizationFormat, selected?: FileSelection) => {
    const result = await api.loadLocalizationFile({ path, format: requestedFormat });
    setLoaded(result); setFile(selected ?? { path: result.path, name: result.name, size: 0, mimeType: "" }); setFormat(result.format);
    setRows(result.entries.map((entry) => ({ ...normalizeEntry(entry), status: translated(normalizeEntry(entry)) ? "translated" : "idle" }))); setProgress(null); setError(""); setSavedPath("");
  }, []);

  const select = useCallback(async () => {
    try { const selected = await api.pickLocalizationFile(format); if (selected?.path) await load(selected.path, format, selected); } catch (reason) { setError(message(reason, "Could not load localization file")); }
  }, [format, load]);

  useEffect(() => { if (dropPath) void load(dropPath, format).catch((reason) => setError(message(reason, "Could not load dropped file"))); }, [dropPath, format, load]);

  useEffect(() => {
    const onProgress = (payload: LocalizationProgress) => {
      if (!payload || payload.operationId !== operationID.current) return;
      setProgress(payload);
      if (payload.status === "translated" && payload.entry) applyEntry(payload.entry, "translated");
      if (payload.status === "translating" && payload.entryId) setRows((current) => current.map((row) => row.id === payload.entryId ? { ...row, status: "translating", error: "" } : row));
      if (payload.status === "failed" && payload.entryId) setRows((current) => current.map((row) => row.id === payload.entryId ? { ...row, translation: [], status: "failed", error: payload.error } : row));
    };
    EventsOn("localization:progress", onProgress);
    return () => EventsOff("localization:progress");
  }, [applyEntry]);

  const run = useCallback(async (entryIDs: string[]) => {
    if (!loaded || busy || entryIDs.length === 0) return;
    const id = newOperationID(); operationID.current = id; setBusy(true); setError(""); setSavedPath(""); setProgress({ operationId: id, status: "translating", completed: 0, total: entryIDs.length });
    try {
      const result = await api.translateLocalizationEntries({ operationId: id, path: loaded.path, format: loaded.format, fingerprint: loaded.fingerprint, language, entryIds: entryIDs });
      for (const entry of result.entries) applyEntry(entry, "translated");
      if (result.failed > 0) setError(`${result.failed} row${result.failed === 1 ? "" : "s"} could not be translated. You can retry them individually.`);
    } catch (reason) { setError(message(reason, "Localization translation failed")); }
    finally { setBusy(false); }
  }, [applyEntry, busy, language, loaded]);

  const translatePending = useCallback(() => { const pending = rows.filter((row) => !translated(row) || row.status === "failed").map((row) => row.id); void run(pending); }, [rows, run]);
  const retranslateAll = useCallback(() => { void run(rows.map((row) => row.id)); }, [rows, run]);
  const updateTranslation = useCallback((id: string, category: string, text: string) => setRows((current) => current.map((row) => row.id !== id ? row : { ...row, translation: updateForm(row, category, text), status: "edited", error: "" })), []);

  const applyLanguage = useCallback((next: string) => {
    setLanguage(next); setRows((current) => current.map((row) => ({ ...row, translation: [], status: "idle", error: "" }))); setProgress(null); setSavedPath("");
  }, []);
  const chooseLanguage = useCallback((next: string) => {
    if (next === language) return;
    if (rows.some((row) => forms(row.translation).length > 0)) { setPendingChange({ type: "language", value: next }); return; }
    applyLanguage(next);
  }, [applyLanguage, language, rows]);

  const applyFormat = useCallback((next: LocalizationFormat) => {
    setFormat(next);
    if (file) void load(file.path, next, file).catch((reason) => setError(message(reason, "Could not parse the file as that format")));
  }, [file, load]);
  const chooseFormat = useCallback((next: string) => {
    const selected = next as LocalizationFormat;
    if (selected === format) return;
    if (file) { setPendingChange({ type: "format", value: selected }); return; }
    applyFormat(selected);
  }, [applyFormat, file, format]);

  const confirmPendingChange = useCallback(() => {
    if (!pendingChange) return;
    setPendingChange(null);
    if (pendingChange.type === "language") applyLanguage(pendingChange.value);
    else applyFormat(pendingChange.value);
  }, [applyFormat, applyLanguage, pendingChange]);

  const incomplete = useMemo(() => rows.filter((row) => !translated(row) || row.status === "failed"), [rows]);
  const hasTranslations = useMemo(() => rows.some(translated), [rows]);
  const save = useCallback(async (mode: UntranslatedExportMode) => {
    if (!loaded) return;
    setSavePending(false); setError("");
    try {
      const result = await api.saveLocalizationFile({ path: loaded.path, format: loaded.format, fingerprint: loaded.fingerprint, language, entries: rows, untranslatedMode: mode });
      if (result.path) setSavedPath(result.path);
    } catch (reason) { setError(message(reason, "Could not save translated file")); }
  }, [language, loaded, rows]);

  const requestSave = useCallback(() => { if (!loaded) return; if (incomplete.length > 0) { setSavePending(true); return; } void save("source"); }, [incomplete.length, loaded, save]);
  const percent = progress && progress.total > 0 ? Math.round((progress.completed / progress.total) * 100) : 0;

  return <section className="localization-workspace surface flex h-full min-h-0 flex-1 flex-col overflow-hidden">
    <header className="localization-toolbar">
      <div className="min-w-48"><StyledSelect value={format} onChange={chooseFormat} options={formatOptions} placeholder="Choose file type" ariaLabel="Localization file type" /></div>
      <button type="button" onClick={() => void select()} disabled={busy} className="app-button app-button-outline"><Upload className="size-4" />Select file</button>
      <div className="ml-auto flex min-w-0 items-center gap-2"><LanguageSelect value={language} onChange={chooseLanguage} className="max-w-56" /><button type="button" onClick={translatePending} disabled={!loaded || busy || rows.length === 0} className="app-button app-button-primary">{busy ? <Loader2 className="size-4 animate-spin" /> : <Languages className="size-4" />}{busy ? "Translating" : "Translate"}</button>{hasTranslations ? <button type="button" onClick={retranslateAll} disabled={!loaded || busy || rows.length === 0} className="app-button app-button-outline" title="Translate every entry again, replacing current translations"><RotateCcw className="size-4" />Retranslate all</button> : null}<button type="button" onClick={requestSave} disabled={!loaded || busy} className="app-button app-button-outline"><Save className="size-4" />Save</button></div>
    </header>
    {file ? <div className="localization-file"><FileText className="size-4 shrink-0 text-muted-foreground" /><span className="min-w-0 flex-1 truncate text-sm font-medium">{file.name}</span>{file.size > 0 ? <span className="text-xs text-muted-foreground">{bytes(file.size)}</span> : null}<button type="button" onClick={reset} disabled={busy} className="app-button app-button-outline px-2" aria-label="Clear localization file"><X className="size-4" /></button></div> : <button type="button" onClick={() => void select()} className="localization-empty"><Upload className="size-6" /><span className="text-sm font-semibold">Select a localization file</span><span>JSON, YAML, Properties, Gettext PO, or Source KeyValues</span></button>}
    {progress ? <div className="localization-progress" aria-live="polite"><div className="flex items-center justify-between gap-3 text-xs"><span className="min-w-0 truncate">{progress.status === "complete" ? "Translation complete" : progress.status === "failed" ? progress.error ?? "A row failed" : "Translating localization entries…"}</span><span className="font-mono">{progress.completed} / {progress.total}</span></div><div className="mt-2 h-1.5 overflow-hidden rounded-full bg-muted"><div className="h-full rounded-full bg-foreground transition-[width] duration-200" style={{ width: `${percent}%` }} /></div></div> : null}
    {error ? <p className="localization-notice localization-error">{error}</p> : null}
    {savedPath ? <p className="localization-notice"><CheckCircle2 className="size-4" />Saved to {savedPath}</p> : null}
    {loaded ? <LocalizationTable rows={rows} busy={busy} onTranslate={(id) => void run([id])} onEdit={updateTranslation} /> : null}
    {savePending ? <IncompleteSaveModal count={incomplete.length} onCancel={() => setSavePending(false)} onSave={(mode) => void save(mode)} /> : null}
    {pendingChange ? <LocalizationChangeModal change={pendingChange} onCancel={() => setPendingChange(null)} onConfirm={confirmPendingChange} /> : null}
  </section>;
}

function LocalizationTable({ rows, busy, onTranslate, onEdit }: { rows: LocalRow[]; busy: boolean; onTranslate: (id: string) => void; onEdit: (id: string, category: string, text: string) => void }) {
  return <div className="localization-table-scroll min-h-0 flex-1 overflow-auto overscroll-contain"><table className="localization-table"><thead><tr><th>Key</th><th>Original</th><th>Translation</th><th aria-label="Actions" /></tr></thead><tbody>{rows.map((row) => <tr key={row.id} className={cn(row.status === "failed" && "localization-row-error")}><td className="localization-key">{row.key}<RowState row={row} /></td><td><Forms forms={row.source} /></td><td><EditableForms row={row} onEdit={onEdit} /></td><td className="localization-actions"><button type="button" onClick={() => onTranslate(row.id)} disabled={busy || row.status === "translating"} className="app-button app-button-outline" title={translated(row) ? "Retranslate row" : "Translate row"}>{row.status === "translating" ? <Loader2 className="size-4 animate-spin" /> : translated(row) ? <RotateCcw className="size-4" /> : <Languages className="size-4" />}<span className="sr-only">Translate row</span></button>{row.error ? <p className="mt-1 max-w-40 text-xs text-destructive">{row.error}</p> : null}</td></tr>)}</tbody></table></div>;
}

function Forms({ forms }: { forms: LocalizationEntry["source"] }) { return <div className="space-y-2">{forms.map((form) => <div key={form.category}><span className="localization-form-label">{forms.length > 1 ? form.category : null}</span><p className="whitespace-pre-wrap break-words text-sm text-muted-foreground">{form.text}</p></div>)}</div>; }
function EditableForms({ row, onEdit }: { row: LocalRow; onEdit: (id: string, category: string, text: string) => void }) { const targetForms = forms(row.translation); const values = targetForms.length > 0 ? targetForms : forms(row.source).map((form) => ({ category: form.category, text: "" })); return <div className="space-y-2">{values.map((form) => <label key={form.category} className="block"><span className="localization-form-label">{values.length > 1 ? form.category : null}</span><textarea value={form.text} onChange={(event) => onEdit(row.id, form.category, event.target.value)} className="localization-translation-field" rows={Math.min(6, Math.max(1, form.text.split("\n").length))} aria-label={`${row.key} ${form.category} translation`} /></label>)}</div>; }
function RowState({ row }: { row: LocalRow }) { const label = row.status === "translated" ? "Translated" : row.status === "failed" ? "Error" : row.status === "translating" ? "Translating" : row.status === "edited" ? "Edited" : "Untranslated"; return <span className={cn("localization-state", `localization-state-${row.status}`)}>{label}</span>; }
function LocalizationChangeModal({ change, onCancel, onConfirm }: { change: PendingChange; onCancel: () => void; onConfirm: () => void }) { const languageChange = change.type === "language"; const title = languageChange ? "Change target language?" : "Reload file with a new format?"; const detail = languageChange ? "This clears every translated value so translations from different languages cannot be mixed." : "This reloads the selected file and clears its current translations."; return <div className="voice-dialog-backdrop" role="dialog" aria-modal="true" aria-labelledby="localization-change-title"><div className="voice-dialog max-w-lg"><div className="voice-dialog-header"><div><h2 id="localization-change-title" className="text-base font-semibold">{title}</h2><p className="mt-2 text-sm leading-relaxed text-muted-foreground">{detail}</p></div><button type="button" onClick={onCancel} className="app-button app-button-outline px-2" aria-label="Close"><X className="size-4" /></button></div><div className="mt-5 flex flex-wrap justify-end gap-2"><button type="button" onClick={onCancel} className="app-button app-button-outline">Cancel</button><button type="button" onClick={onConfirm} className="app-button app-button-primary">Continue</button></div></div></div>; }
function IncompleteSaveModal({ count, onCancel, onSave }: { count: number; onCancel: () => void; onSave: (mode: UntranslatedExportMode) => void }) { return <div className="voice-dialog-backdrop" role="dialog" aria-modal="true" aria-labelledby="incomplete-save-title"><div className="voice-dialog max-w-lg"><div className="voice-dialog-header"><div><h2 id="incomplete-save-title" className="text-base font-semibold">Untranslated entries remain</h2><p className="mt-2 text-sm leading-relaxed text-muted-foreground">{count} row{count === 1 ? " is" : "s are"} incomplete or failed. Choose how the target file should represent them.</p></div><button type="button" onClick={onCancel} className="app-button app-button-outline px-2" aria-label="Close"><X className="size-4" /></button></div><div className="mt-5 flex flex-wrap justify-end gap-2"><button type="button" onClick={onCancel} className="app-button app-button-outline">Cancel</button><button type="button" onClick={() => onSave("empty")} className="app-button app-button-outline">Save with empty translations</button><button type="button" onClick={() => onSave("source")} className="app-button app-button-primary">Save with original text</button></div></div></div>; }
function updateForm(row: LocalRow, category: string, text: string) { const targetForms = forms(row.translation); const values = targetForms.length > 0 ? targetForms : forms(row.source).map((form) => ({ category: form.category, text: "" })); return values.map((form) => form.category === category ? { ...form, text } : form); }
function translated(row: LocalizationEntry) { const targetForms = forms(row.translation); return targetForms.length > 0 && targetForms.every((form) => form.text.trim() !== ""); }
function forms<T>(value: T[] | null | undefined) { return value ?? []; }
function normalizeEntry(entry: LocalizationEntry): LocalizationEntry { return { ...entry, source: forms(entry.source), translation: forms(entry.translation) }; }
function message(reason: unknown, fallback: string) { return reason instanceof Error ? reason.message : fallback; }
function newOperationID() { return globalThis.crypto?.randomUUID?.() ?? `localization-${Date.now()}-${Math.random().toString(16).slice(2)}`; }

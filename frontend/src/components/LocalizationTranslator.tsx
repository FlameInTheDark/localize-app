import { memo, useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from "react";
import { CheckCircle2, ChevronDown, ChevronRight, Copy, Eye, FileText, Languages, List, ListChecks, Loader2, Replace, RotateCcw, Save, Search, Upload, X } from "lucide-react";
import { EventsOff, EventsOn } from "../../wailsjs/runtime/runtime";
import { api } from "@/lib/bridge";
import { bytes, cn } from "@/lib/utils";
import { LanguageSelect } from "./LanguageSelect";
import { StyledSelect } from "./StyledSelect";
import type { FileSelection, LocalizationEntry, LocalizationFile, LocalizationFormat, LocalizationProgress, UntranslatedExportMode } from "@/types/api";

type RowStatus = "idle" | "translating" | "translated" | "failed" | "skipped" | "edited";
type LocalRow = LocalizationEntry & { status: RowStatus; error?: string; skipReason?: string };
type StatusFilter = "all" | "empty" | RowStatus;
type DisplayRowStatus = Exclude<StatusFilter, "all">;
type StatusSummary = { status: DisplayRowStatus; label: string; count: number };
type LocalizationRibbonTab = "file" | "translate" | "review";
type ReplaceScope = "all" | "visible" | "selected";
type PendingChange = { type: "format"; value: LocalizationFormat };
type LocalizationRowGroup = { key: string; rows: LocalRow[]; allRows: LocalRow[] };

const formatOptions = [
  { value: "auto", label: "Auto-detect from file extension" }, { value: "i18next-json", label: "i18next JSON (.json)" },
  { value: "yaml", label: "YAML (.yaml, .yml)" }, { value: "properties", label: "Java properties (.properties)" },
  { value: "gettext-po", label: "Gettext / Unreal PO (.po)" }, { value: "source-keyvalues", label: "Source KeyValues (.txt, .vdf)" },
];

const statusFilterOptions = [
  { value: "all", label: "All statuses" }, { value: "idle", label: "Untranslated" }, { value: "translated", label: "Translated" },
  { value: "edited", label: "Edited" }, { value: "failed", label: "Failed" }, { value: "skipped", label: "Skipped" }, { value: "empty", label: "No text" },
];

const statusSummaryOptions: Array<Omit<StatusSummary, "count">> = [
  { status: "translating", label: "Translating" }, { status: "failed", label: "Failed" }, { status: "edited", label: "Edited" },
  { status: "idle", label: "Untranslated" }, { status: "translated", label: "Translated" }, { status: "skipped", label: "Skipped" }, { status: "empty", label: "No text" },
];

export function LocalizationTranslator({ dropPath }: { dropPath?: string }) {
  const [format, setFormat] = useState<LocalizationFormat>("auto");
  const [file, setFile] = useState<FileSelection | null>(null);
  const [loaded, setLoaded] = useState<LocalizationFile | null>(null);
  const [rows, setRows] = useState<LocalRow[]>([]);
  const [language, setLanguage] = useState("en");
  const [sourceLanguage, setSourceLanguage] = useState("auto");
  const [busy, setBusy] = useState(false);
  const [progress, setProgress] = useState<LocalizationProgress | null>(null);
  const [error, setError] = useState("");
  const [savedPath, setSavedPath] = useState("");
  const [savePending, setSavePending] = useState(false);
  const [pendingChange, setPendingChange] = useState<PendingChange | null>(null);
  const [search, setSearch] = useState("");
  const [replaceFind, setReplaceFind] = useState("");
  const [replaceWith, setReplaceWith] = useState("");
  const [replaceScope, setReplaceScope] = useState<ReplaceScope>("all");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [filterPinnedRowIDs, setFilterPinnedRowIDs] = useState<Set<string>>(() => new Set());
  const [keySeparator, setKeySeparator] = useState(".");
  const [translationRules, setTranslationRules] = useState("");
  const [selectedRows, setSelectedRows] = useState<Set<string>>(() => new Set());
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(() => new Set());
  const [ribbonTab, setRibbonTab] = useState<LocalizationRibbonTab>("file");
  const [rulesOpen, setRulesOpen] = useState(false);
  const operationID = useRef("");
  const busyRef = useRef(false);
  const translationRulesRef = useRef("");

  const reset = useCallback(() => { setFile(null); setLoaded(null); setRows([]); setProgress(null); setError(""); setSavedPath(""); setSearch(""); setReplaceFind(""); setReplaceWith(""); setReplaceScope("all"); setFilterPinnedRowIDs(new Set()); translationRulesRef.current = ""; setTranslationRules(""); setSelectedRows(new Set()); setCollapsedGroups(new Set()); setRulesOpen(false); setRibbonTab("file"); operationID.current = ""; }, []);
  const applyEntry = useCallback((entry: LocalizationEntry, status: RowStatus, skipReason = "") => setRows((current) => current.map((row) => row.id === entry.id ? { ...row, ...normalizeEntry(entry), status, error: "", skipReason } : row)), []);

  const load = useCallback(async (path: string, requestedFormat: LocalizationFormat, selected?: FileSelection) => {
    const result = await api.loadLocalizationFile({ path, format: requestedFormat });
    setLoaded(result); setFile(selected ?? { path: result.path, name: result.name, size: 0, mimeType: "" }); setFormat(result.format);
    setRows(result.entries.map((entry) => ({ ...normalizeEntry(entry), status: translated(normalizeEntry(entry)) ? "translated" : "idle" }))); setProgress(null); setError(""); setSavedPath(""); setSearch(""); setReplaceFind(""); setReplaceWith(""); setReplaceScope("all"); setFilterPinnedRowIDs(new Set()); translationRulesRef.current = ""; setTranslationRules(""); setSelectedRows(new Set()); setCollapsedGroups(new Set()); setRulesOpen(false); setRibbonTab("translate");
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
      if (payload.status === "skipped" && payload.entry) applyEntry(payload.entry, "skipped", payload.error);
      if ((payload.status === "detecting" || payload.status === "translating") && payload.entryId) setRows((current) => current.map((row) => row.id === payload.entryId ? { ...row, status: "translating", error: "", skipReason: "" } : row));
      if (payload.status === "failed" && payload.entryId) setRows((current) => current.map((row) => row.id === payload.entryId ? { ...row, translation: [], status: "failed", error: payload.error, skipReason: "" } : row));
    };
    EventsOn("localization:progress", onProgress);
    return () => EventsOff("localization:progress");
  }, [applyEntry]);

  const run = useCallback(async (entryIDs: string[]) => {
    if (!loaded || busyRef.current || entryIDs.length === 0) return;
    const id = newOperationID(); operationID.current = id; busyRef.current = true; setBusy(true); setError(""); setSavedPath(""); setProgress({ operationId: id, status: "translating", completed: 0, total: entryIDs.length });
    try {
      const result = await api.translateLocalizationEntries({ operationId: id, path: loaded.path, format: loaded.format, fingerprint: loaded.fingerprint, language, sourceLanguage, rules: translationRulesRef.current, entryIds: entryIDs });
      for (const entry of result.entries) applyEntry(entry, "translated");
      if (result.failed > 0) setError(`${result.failed} row${result.failed === 1 ? "" : "s"} could not be translated. You can retry them individually.`);
    } catch (reason) { setError(message(reason, "Localization translation failed")); }
    finally { busyRef.current = false; setBusy(false); }
  }, [applyEntry, language, loaded, sourceLanguage]);

  const translatePending = useCallback(() => { const pending = rows.filter(needsTranslation).map((row) => row.id); void run(pending); }, [rows, run]);
  const retranslateAll = useCallback(() => { void run(rows.filter(hasTranslatableText).map((row) => row.id)); }, [rows, run]);
  const translateRow = useCallback((id: string) => { void run([id]); }, [run]);
  const retainRowInStatusFilter = useCallback((id: string) => { if (statusFilter !== "all") setFilterPinnedRowIDs((current) => new Set(current).add(id)); }, [statusFilter]);
  const updateTranslation = useCallback((id: string, category: string, text: string) => { retainRowInStatusFilter(id); setRows((current) => current.map((row) => row.id !== id ? row : { ...row, translation: updateForm(row, category, text), status: "edited", error: "", skipReason: "" })); }, [retainRowInStatusFilter]);
  const useOriginal = useCallback((id: string) => { retainRowInStatusFilter(id); setRows((current) => current.map((row) => row.id !== id ? row : { ...row, translation: originalForms(row), status: "edited", error: "", skipReason: "" })); }, [retainRowInStatusFilter]);
  const chooseSourceLanguage = useCallback((next: string) => {
    if (next === sourceLanguage) return;
    setSourceLanguage(next); setProgress(null); setError("");
    setRows((current) => current.map((row) => row.status === "skipped" ? { ...row, translation: [], status: "idle", error: "", skipReason: "" } : row));
  }, [sourceLanguage]);

  const applyLanguage = useCallback((next: string) => { setLanguage(next); setProgress(null); setSavedPath(""); }, []);
  const chooseLanguage = useCallback((next: string) => {
    if (next === language) return;
    applyLanguage(next);
  }, [applyLanguage, language]);

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
    applyFormat(pendingChange.value);
  }, [applyFormat, pendingChange]);

  const incomplete = useMemo(() => rows.filter(needsTranslation), [rows]);
  const hasPendingTranslations = useMemo(() => rows.some(needsTranslation), [rows]);
  const hasTranslatableRows = useMemo(() => rows.some(hasTranslatableText), [rows]);
  const hasTranslations = useMemo(() => rows.some((row) => row.status !== "skipped" && hasTranslatableText(row) && translated(row)), [rows]);
  const deferredSearch = useDeferredValue(search);
  const normalizedSearch = deferredSearch.trim().toLocaleLowerCase();
  const matchesVisibleRow = useCallback((row: LocalRow) => (filterPinnedRowIDs.has(row.id) || matchesStatusFilter(row, statusFilter)) && (!normalizedSearch || matchesLocalizationSearch(row, normalizedSearch)), [filterPinnedRowIDs, normalizedSearch, statusFilter]);
  const chooseStatusFilter = useCallback((value: string) => { setStatusFilter(value as StatusFilter); setFilterPinnedRowIDs(new Set()); }, []);
  const rowGroups = useMemo(() => groupLocalizationRows(rows, keySeparator, matchesVisibleRow), [keySeparator, matchesVisibleRow, rows]);
  const visibleRows = useMemo(() => rowGroups.flatMap((group) => group.rows), [rowGroups]);
  const selectedVisibleCount = useMemo(() => visibleRows.reduce((count, row) => count + (selectedRows.has(row.id) ? 1 : 0), 0), [selectedRows, visibleRows]);
  const retainedVisibleCount = useMemo(() => visibleRows.reduce((count, row) => count + (filterPinnedRowIDs.has(row.id) ? 1 : 0), 0), [filterPinnedRowIDs, visibleRows]);
  const selectedTranslatableCount = useMemo(() => rows.reduce((count, row) => count + (selectedRows.has(row.id) && hasTranslatableText(row) ? 1 : 0), 0), [rows, selectedRows]);
  const replaceScopeRows = useMemo(() => replaceScope === "all" ? rows : replaceScope === "visible" ? visibleRows : rows.filter((row) => selectedRows.has(row.id)), [replaceScope, rows, selectedRows, visibleRows]);
  const replaceScopeLabel = replaceScope === "all" ? "all rows" : replaceScope === "visible" ? "visible rows" : "selected rows";
  const replacementStats = useMemo(() => {
    if (replaceFind.length === 0) return { rows: 0, occurrences: 0 };
    let rowsWithMatches = 0; let occurrences = 0;
    for (const row of replaceScopeRows) { const rowOccurrences = forms(row.translation).reduce((count, form) => count + countOccurrences(form.text, replaceFind), 0); if (rowOccurrences > 0) { rowsWithMatches++; occurrences += rowOccurrences; } }
    return { rows: rowsWithMatches, occurrences };
  }, [replaceFind, replaceScopeRows]);
  const statusSummary = useMemo<StatusSummary[]>(() => {
    const counts = new Map<DisplayRowStatus, number>();
    for (const row of rows) { const status = localizationRowStatus(row); counts.set(status, (counts.get(status) ?? 0) + 1); }
    return statusSummaryOptions.map((option) => ({ ...option, count: counts.get(option.status) ?? 0 })).filter((option) => option.count > 0);
  }, [rows]);
  const setSelectionForIDs = useCallback((ids: string[], selected: boolean) => setSelectedRows((current) => {
    const next = new Set(current);
    for (const id of ids) { if (selected) next.add(id); else next.delete(id); }
    return next;
  }), []);
  const toggleRowSelection = useCallback((id: string) => setSelectedRows((current) => {
    const next = new Set(current);
    if (next.has(id)) next.delete(id); else next.add(id);
    return next;
  }), []);
  const toggleGroupSelection = useCallback((ids: string[]) => setSelectedRows((current) => {
    const next = new Set(current);
    const allSelected = ids.every((id) => next.has(id));
    for (const id of ids) { if (allSelected) next.delete(id); else next.add(id); }
    return next;
  }), []);
  const toggleGroup = useCallback((key: string) => setCollapsedGroups((current) => {
    const next = new Set(current);
    if (next.has(key)) next.delete(key); else next.add(key);
    return next;
  }), []);
  const translateSelected = useCallback(() => { void run(rows.filter((row) => selectedRows.has(row.id) && hasTranslatableText(row)).map((row) => row.id)); }, [rows, run, selectedRows]);
  const useOriginalSelected = useCallback(() => setRows((current) => current.map((row) => !selectedRows.has(row.id) || !hasTranslatableText(row) ? row : { ...row, translation: originalForms(row), status: "edited", error: "", skipReason: "" })), [selectedRows]);
  const replaceTranslationText = useCallback(() => {
    if (replaceFind.length === 0 || replacementStats.rows === 0) return;
    const scopeIDs = new Set(replaceScopeRows.map((row) => row.id));
    const matchedIDs = new Set(replaceScopeRows.filter((row) => forms(row.translation).some((form) => form.text.includes(replaceFind))).map((row) => row.id));
    if (statusFilter !== "all") { const visibleIDs = new Set(visibleRows.map((row) => row.id)); setFilterPinnedRowIDs((current) => { const next = new Set(current); for (const id of matchedIDs) if (visibleIDs.has(id)) next.add(id); return next; }); }
    setRows((current) => current.map((row) => !scopeIDs.has(row.id) || !matchedIDs.has(row.id) ? row : { ...row, translation: forms(row.translation).map((form) => ({ ...form, text: form.text.split(replaceFind).join(replaceWith) })), status: "edited", error: "", skipReason: "" }));
  }, [replaceFind, replaceScopeRows, replaceWith, replacementStats.rows, statusFilter, visibleRows]);
  const updateTranslationRules = useCallback((value: string) => { translationRulesRef.current = value; setTranslationRules(value); }, []);
  const save = useCallback(async (mode: UntranslatedExportMode) => {
    if (!loaded) return;
    setSavePending(false); setError("");
    try {
      const result = await api.saveLocalizationFile({ path: loaded.path, format: loaded.format, fingerprint: loaded.fingerprint, language, entries: rows, untranslatedMode: mode });
      if (result.path) setSavedPath(result.path);
    } catch (reason) { setError(message(reason, "Could not save translated file")); }
  }, [language, loaded, rows]);

  const requestSave = useCallback(() => { if (!loaded) return; if (incomplete.length > 0) { setSavePending(true); return; } void save("source"); }, [incomplete.length, loaded, save]);
  return <section className="localization-workspace surface flex h-full min-h-0 flex-1 flex-col overflow-hidden">
    <header className="localization-ribbon" aria-label="Localization toolbar">
      <nav className="localization-ribbon-tabs" role="tablist" aria-label="Localization ribbon tabs">
        <button type="button" role="tab" aria-selected={ribbonTab === "file"} data-active={ribbonTab === "file"} onClick={() => setRibbonTab("file")}>File</button>
        <button type="button" role="tab" aria-selected={ribbonTab === "translate"} data-active={ribbonTab === "translate"} onClick={() => setRibbonTab("translate")} disabled={!loaded}>Translate</button>
        <button type="button" role="tab" aria-selected={ribbonTab === "review"} data-active={ribbonTab === "review"} onClick={() => setRibbonTab("review")} disabled={!loaded}>Review</button>
      </nav>
      <div className="localization-ribbon-commands" role="tabpanel">
        {ribbonTab === "file" ? <><div className="localization-ribbon-group"><div className="localization-ribbon-group-content">{file ? <><div className="localization-ribbon-file-summary"><FileText className="size-5 shrink-0 text-muted-foreground" /><span className="min-w-0"><span className="localization-ribbon-file-name" title={file.name}>{file.name}</span>{file.size > 0 ? <span className="localization-ribbon-file-size">{bytes(file.size)}</span> : null}</span></div><button type="button" onClick={reset} disabled={busy} className="app-button app-button-outline localization-ribbon-command" aria-label="Close localization file" title="Close file"><X className="size-5" /><span>Close</span></button></> : <><button type="button" onClick={() => void select()} disabled={busy} className="app-button app-button-outline localization-ribbon-command localization-ribbon-open"><Upload className="size-5" /><span>Open file</span></button><label className="localization-ribbon-control"><span>File type</span><div className="localization-ribbon-format"><StyledSelect value={format} onChange={chooseFormat} options={formatOptions} placeholder="Choose file type" ariaLabel="Localization file type" /></div></label></>}</div><span className="localization-ribbon-label">File</span></div><div className="localization-ribbon-group"><div className="localization-ribbon-group-content"><button type="button" onClick={requestSave} disabled={!loaded || busy} className="app-button app-button-outline localization-ribbon-command"><Save className="size-5" /><span>Save</span></button></div><span className="localization-ribbon-label">Export</span></div></> : null}
        {ribbonTab === "translate" ? <><div className="localization-ribbon-group"><div className="localization-ribbon-group-content"><label className="localization-ribbon-control"><span>From</span><LanguageSelect value={sourceLanguage} onChange={chooseSourceLanguage} label="Source language" allowAuto allowCustom={false} autoLabel="Automatic" className="w-36" /></label><label className="localization-ribbon-control"><span>To</span><LanguageSelect value={language} onChange={chooseLanguage} className="w-36" /></label></div><span className="localization-ribbon-label">Languages</span></div><div className="localization-ribbon-group"><div className="localization-ribbon-group-content"><button type="button" onClick={translatePending} disabled={!loaded || busy || !hasPendingTranslations} className="app-button app-button-primary localization-ribbon-command localization-ribbon-command-primary">{busy ? <Loader2 className="size-5 animate-spin" /> : <Languages className="size-5" />}<span>{busy ? "Translating" : "Translate"}</span></button><button type="button" onClick={translateSelected} disabled={!loaded || busy || selectedTranslatableCount === 0} className="app-button app-button-outline localization-ribbon-command" title="Translate the selected rows, replacing any existing translations"><Languages className="size-5" /><span>Selected{selectedTranslatableCount > 0 ? ` (${selectedTranslatableCount})` : ""}</span></button><button type="button" onClick={useOriginalSelected} disabled={busy || selectedTranslatableCount === 0} className="app-button app-button-outline localization-ribbon-command" title="Copy original source text into the selected rows"><Copy className="size-5" /><span>Use original</span></button>{hasTranslations ? <button type="button" onClick={retranslateAll} disabled={!loaded || busy || !hasTranslatableRows} className="app-button app-button-outline localization-ribbon-command" title="Translate every entry again, replacing current translations"><RotateCcw className="size-5" /><span>Again</span></button> : null}</div><span className="localization-ribbon-label">Translation</span></div><div className="localization-ribbon-group"><div className="localization-ribbon-group-content"><button type="button" onClick={() => setRulesOpen((open) => !open)} disabled={!loaded} className="app-button app-button-outline localization-ribbon-command" title="Edit translation rules"><FileText className="size-5" /><span>{rulesOpen ? "Hide rules" : translationRules.trim() ? "Rules active" : "Rules"}</span></button></div><span className="localization-ribbon-label">Context</span></div></> : null}
        {ribbonTab === "review" ? <><div className="localization-ribbon-group"><div className="localization-ribbon-group-content"><div className="localization-ribbon-control localization-ribbon-search-control"><span>Search</span><label className="localization-search"><Search className="size-4 shrink-0 text-muted-foreground" /><input type="text" inputMode="search" value={search} onChange={(event) => setSearch(event.target.value)} onKeyDown={(event) => { if (event.key === "Escape") setSearch(""); }} placeholder="Key, original, or translation…" aria-label="Search localization entries" />{search ? <button type="button" onClick={() => setSearch("")} aria-label="Clear localization search" title="Clear search"><X className="size-3.5" /></button> : null}</label></div><label className="localization-ribbon-control"><span>Status</span><StyledSelect value={statusFilter} onChange={chooseStatusFilter} options={statusFilterOptions} placeholder="All statuses" ariaLabel="Filter localization rows by status" className="min-w-36" /></label><label className="localization-ribbon-control"><span>Group separator</span><input value={keySeparator} onChange={(event) => { setKeySeparator(event.target.value); setCollapsedGroups(new Set()); }} placeholder="None" aria-label="Key separator for localization groups" /></label></div><span className="localization-ribbon-label">Find and filter</span></div><div className="localization-ribbon-group"><div className="localization-ribbon-group-content"><div className="localization-ribbon-replace-fields"><input value={replaceFind} onChange={(event) => setReplaceFind(event.target.value)} placeholder="Find in translations" aria-label="Find text in translations" /><input value={replaceWith} onChange={(event) => setReplaceWith(event.target.value)} placeholder="Replace with" aria-label="Replacement text in translations" /></div><div className="localization-ribbon-replace-actions"><div className="localization-ribbon-toggle-group" role="group" aria-label="Replace scope"><button type="button" data-active={replaceScope === "all"} aria-pressed={replaceScope === "all"} onClick={() => setReplaceScope("all")} title="All rows" aria-label="Replace in all rows"><List className="size-4" /><span className="sr-only">All rows</span></button><button type="button" data-active={replaceScope === "visible"} aria-pressed={replaceScope === "visible"} onClick={() => setReplaceScope("visible")} title="Visible rows" aria-label="Replace in visible rows"><Eye className="size-4" /><span className="sr-only">Visible rows</span></button><button type="button" data-active={replaceScope === "selected"} aria-pressed={replaceScope === "selected"} onClick={() => setReplaceScope("selected")} disabled={selectedRows.size === 0} title="Selected rows" aria-label="Replace in selected rows"><ListChecks className="size-4" /><span className="sr-only">Selected rows</span></button></div><button type="button" onClick={replaceTranslationText} disabled={busy || replaceFind.length === 0 || replacementStats.rows === 0} className="app-button app-button-outline" title={`Replace ${replacementStats.occurrences.toLocaleString()} occurrence${replacementStats.occurrences === 1 ? "" : "s"} in ${replacementStats.rows.toLocaleString()} ${replaceScopeLabel}`}><Replace className="size-4" /><span>Replace</span></button></div></div><span className="localization-ribbon-label">Translation text</span></div><div className="localization-ribbon-group"><div className="localization-ribbon-group-content"><button type="button" onClick={() => setSelectionForIDs(visibleRows.map((row) => row.id), true)} disabled={visibleRows.length === 0 || selectedVisibleCount === visibleRows.length} className="app-button app-button-outline localization-ribbon-command"><CheckCircle2 className="size-5" /><span>Select visible</span></button><button type="button" onClick={() => setSelectedRows(new Set())} disabled={selectedRows.size === 0} className="app-button app-button-outline localization-ribbon-command"><X className="size-5" /><span>Clear</span></button></div><span className="localization-ribbon-label">Selection</span></div></> : null}
      </div>
    </header>
    {!file ? <button type="button" onClick={() => void select()} className="localization-empty"><Upload className="size-6" /><span className="text-sm font-semibold">Select a localization file</span><span>JSON, YAML, Properties, Gettext PO, or Source KeyValues</span></button> : null}
    {!loaded && error ? <p className="localization-notice localization-error">{error}</p> : null}
    {loaded && rulesOpen ? <section className="localization-rules"><div className="localization-rules-header"><div><h2>Translation rules{translationRules.trim() ? " · active" : ""}</h2><p>Applied to both batch and individual-row translation actions for this file.</p></div><button type="button" onClick={() => setRulesOpen(false)} className="app-button app-button-outline px-2" aria-label="Close translation rules"><X className="size-4" /></button></div><div className="localization-rules-content"><textarea value={translationRules} onChange={(event) => updateTranslationRules(event.target.value)} disabled={busy} rows={5} placeholder={"Add context, terminology, or rules for this file.\nDescribe what should be preserved, translated, or rendered in a specific way."} aria-label="Localization translation rules" /></div></section> : null}
    {loaded ? <LocalizationTable groups={rowGroups} totalRows={rows.length} retainedVisibleCount={retainedVisibleCount} statusSummary={statusSummary} progress={progress} error={error} savedPath={savedPath} grouped={keySeparator.length > 0} busy={busy} hasFilters={Boolean(search.trim()) || statusFilter !== "all"} selectedRows={selectedRows} collapsedGroups={collapsedGroups} onToggleSelection={toggleRowSelection} onToggleGroupSelection={toggleGroupSelection} onToggleGroup={toggleGroup} onTranslate={translateRow} onUseOriginal={useOriginal} onEdit={updateTranslation} /> : null}
    {savePending ? <IncompleteSaveModal count={incomplete.length} onCancel={() => setSavePending(false)} onSave={(mode) => void save(mode)} /> : null}
    {pendingChange ? <LocalizationChangeModal change={pendingChange} onCancel={() => setPendingChange(null)} onConfirm={confirmPendingChange} /> : null}
  </section>;
}

const LocalizationTable = memo(function LocalizationTable({ groups, totalRows, retainedVisibleCount, statusSummary, progress, error, savedPath, grouped, busy, hasFilters, selectedRows, collapsedGroups, onToggleSelection, onToggleGroupSelection, onToggleGroup, onTranslate, onUseOriginal, onEdit }: { groups: LocalizationRowGroup[]; totalRows: number; retainedVisibleCount: number; statusSummary: StatusSummary[]; progress: LocalizationProgress | null; error: string; savedPath: string; grouped: boolean; busy: boolean; hasFilters: boolean; selectedRows: Set<string>; collapsedGroups: Set<string>; onToggleSelection: (id: string) => void; onToggleGroupSelection: (ids: string[]) => void; onToggleGroup: (key: string) => void; onTranslate: (id: string) => void; onUseOriginal: (id: string) => void; onEdit: (id: string, category: string, text: string) => void }) {
  const visibleCount = groups.reduce((count, group) => count + group.rows.length, 0);
  const percent = progress && progress.total > 0 ? Math.round((progress.completed / progress.total) * 100) : 0;
  const progressLabel = progress?.status === "complete" ? "Translation complete" : progress?.status === "failed" ? progress.error ?? "A row failed" : progress?.status === "detecting" ? "Checking source language…" : progress?.status === "skipped" ? progress.error ?? "Skipped a non-matching source language" : "Translating localization entries…";
  return <div className="flex min-h-0 flex-1 flex-col"><div className="localization-table-scroll min-h-0 flex-1 overflow-auto overscroll-contain"><table className="localization-table"><thead><tr><th>Key</th><th>Original</th><th>Translation</th><th aria-label="Actions" /></tr></thead><tbody>{visibleCount === 0 ? <tr><td colSpan={4} className="localization-search-empty">{hasFilters ? "No entries match the current filters." : "This file has no localization entries."}</td></tr> : groups.map((group) => <LocalizationTableGroup key={group.key} group={group} grouped={grouped} busy={busy} selectedRows={selectedRows} collapsed={collapsedGroups.has(group.key)} onToggleSelection={onToggleSelection} onToggleGroupSelection={onToggleGroupSelection} onToggleGroup={onToggleGroup} onTranslate={onTranslate} onUseOriginal={onUseOriginal} onEdit={onEdit} />)}</tbody></table></div><footer className="localization-table-status"><div className="localization-table-status-counts"><span>{visibleCount.toLocaleString()} of {totalRows.toLocaleString()} entries</span>{selectedRows.size > 0 ? <span>{selectedRows.size.toLocaleString()} selected</span> : null}{retainedVisibleCount > 0 ? <span>{retainedVisibleCount.toLocaleString()} kept visible</span> : null}{statusSummary.map((item) => <span key={item.status} className={`localization-table-status-count localization-table-status-count-${item.status}`}>{item.count.toLocaleString()} {item.label}</span>)}</div>{error ? <div className="localization-table-status-message localization-table-status-error" aria-live="polite">{error}</div> : savedPath ? <div className="localization-table-status-message" aria-live="polite"><CheckCircle2 className="size-3.5 shrink-0" />Saved to {savedPath}</div> : progress ? <div className="localization-table-status-message" aria-live="polite">{progress.status === "translating" ? <Loader2 className="size-3.5 shrink-0 animate-spin" /> : null}<span className="truncate">{progressLabel}</span><span className="font-mono">{progress.completed} / {progress.total}</span><span className="localization-table-status-progress"><span style={{ width: `${percent}%` }} /></span></div> : null}</footer></div>;
});

const LocalizationTableGroup = memo(function LocalizationTableGroup({ group, grouped, busy, selectedRows, collapsed, onToggleSelection, onToggleGroupSelection, onToggleGroup, onTranslate, onUseOriginal, onEdit }: { group: LocalizationRowGroup; grouped: boolean; busy: boolean; selectedRows: Set<string>; collapsed: boolean; onToggleSelection: (id: string) => void; onToggleGroupSelection: (ids: string[]) => void; onToggleGroup: (key: string) => void; onTranslate: (id: string) => void; onUseOriginal: (id: string) => void; onEdit: (id: string, category: string, text: string) => void }) {
  const allIDs = group.allRows.map((row) => row.id);
  const selectedCount = allIDs.reduce((count, id) => count + (selectedRows.has(id) ? 1 : 0), 0);
  return <>{grouped ? <tr className="localization-group-row"><td colSpan={4}><div className="localization-group-header"><button type="button" onClick={() => onToggleGroup(group.key)} className="localization-group-toggle" aria-expanded={!collapsed}><span className="localization-group-chevron">{collapsed ? <ChevronRight className="size-4" /> : <ChevronDown className="size-4" />}</span><span className="font-mono text-xs font-semibold">{group.key}</span><span className="text-xs text-muted-foreground">{group.rows.length} visible · {group.allRows.length} total</span></button><SelectionCheckbox checked={selectedCount === allIDs.length && allIDs.length > 0} indeterminate={selectedCount > 0 && selectedCount < allIDs.length} disabled={busy || allIDs.length === 0} onChange={() => onToggleGroupSelection(allIDs)} ariaLabel={`Select all ${group.key} entries`} /></div></td></tr> : null}{!collapsed && group.rows.map((row) => <LocalizationTableRow key={row.id} row={row} busy={busy} selected={selectedRows.has(row.id)} onToggleSelection={onToggleSelection} onTranslate={onTranslate} onUseOriginal={onUseOriginal} onEdit={onEdit} />)}</>;
});

const LocalizationTableRow = memo(function LocalizationTableRow({ row, busy, selected, onToggleSelection, onTranslate, onUseOriginal, onEdit }: { row: LocalRow; busy: boolean; selected: boolean; onToggleSelection: (id: string) => void; onTranslate: (id: string) => void; onUseOriginal: (id: string) => void; onEdit: (id: string, category: string, text: string) => void }) {
  const noText = !hasTranslatableText(row);
  return <tr className={cn(row.status === "failed" && "localization-row-error")}><td className="localization-key"><div className="flex items-start gap-2"><SelectionCheckbox checked={selected} disabled={busy} onChange={() => onToggleSelection(row.id)} ariaLabel={`Select ${row.key}`} /><span className="min-w-0 flex-1">{row.key}<RowState row={row} /></span></div></td><td><Forms forms={row.source} /></td><td><EditableForms row={row} onEdit={onEdit} /></td><td className="localization-actions"><div className="localization-action-buttons"><button type="button" onClick={() => onTranslate(row.id)} disabled={busy || noText || row.status === "translating"} className="app-button app-button-outline" title={noText ? "Nothing to translate" : translated(row) ? "Retranslate row" : "Translate row"}>{row.status === "translating" ? <Loader2 className="size-4 animate-spin" /> : translated(row) ? <RotateCcw className="size-4" /> : <Languages className="size-4" />}<span className="sr-only">Translate row</span></button><button type="button" onClick={() => onUseOriginal(row.id)} disabled={busy || noText} className="app-button app-button-outline" title="Use original source text"><Copy className="size-4" /><span className="sr-only">Use original source text</span></button></div>{row.error ? <p className="mt-1 max-w-40 text-xs text-destructive">{row.error}</p> : null}</td></tr>;
});

function SelectionCheckbox({ checked, indeterminate = false, disabled = false, onChange, ariaLabel }: { checked: boolean; indeterminate?: boolean; disabled?: boolean; onChange: () => void; ariaLabel: string }) { const input = useRef<HTMLInputElement>(null); useEffect(() => { if (input.current) input.current.indeterminate = indeterminate; }, [indeterminate]); return <input ref={input} type="checkbox" checked={checked} disabled={disabled} onChange={onChange} aria-label={ariaLabel} className="localization-selection-control" />; }

function Forms({ forms }: { forms: LocalizationEntry["source"] }) { return <div className="space-y-2">{forms.map((form) => <div key={form.category}><span className="localization-form-label">{forms.length > 1 ? form.category : null}</span><p className="whitespace-pre-wrap break-words text-sm text-muted-foreground">{form.text}</p></div>)}</div>; }
function EditableForms({ row, onEdit }: { row: LocalRow; onEdit: (id: string, category: string, text: string) => void }) { const targetForms = forms(row.translation); const values = targetForms.length > 0 ? targetForms : forms(row.source).map((form) => ({ category: form.category, text: "" })); return <div className="space-y-2">{values.map((form) => <label key={form.category} className="block"><span className="localization-form-label">{values.length > 1 ? form.category : null}</span><textarea value={form.text} onChange={(event) => onEdit(row.id, form.category, event.target.value)} className="localization-translation-field" rows={Math.min(6, Math.max(1, form.text.split("\n").length))} aria-label={`${row.key} ${form.category} translation`} /></label>)}</div>; }
function RowState({ row }: { row: LocalRow }) { const status = localizationRowStatus(row); const label = status === "empty" ? "No text" : status === "translated" ? "Translated" : status === "failed" ? "Error" : status === "skipped" ? "Skipped" : status === "translating" ? "Translating" : status === "edited" ? "Edited" : "Untranslated"; return <span className={cn("localization-state", `localization-state-${status === "empty" ? "idle" : status}`)} title={status === "skipped" ? row.skipReason : undefined}>{label}</span>; }
function LocalizationChangeModal({ onCancel, onConfirm }: { change: PendingChange; onCancel: () => void; onConfirm: () => void }) { return <div className="voice-dialog-backdrop" role="dialog" aria-modal="true" aria-labelledby="localization-change-title"><div className="voice-dialog max-w-lg"><div className="voice-dialog-header"><div><h2 id="localization-change-title" className="text-base font-semibold">Reload file with a new format?</h2><p className="mt-2 text-sm leading-relaxed text-muted-foreground">This reloads the selected file and clears its current translations.</p></div><button type="button" onClick={onCancel} className="app-button app-button-outline px-2" aria-label="Close"><X className="size-4" /></button></div><div className="mt-5 flex flex-wrap justify-end gap-2"><button type="button" onClick={onCancel} className="app-button app-button-outline">Cancel</button><button type="button" onClick={onConfirm} className="app-button app-button-primary">Continue</button></div></div></div>; }
function IncompleteSaveModal({ count, onCancel, onSave }: { count: number; onCancel: () => void; onSave: (mode: UntranslatedExportMode) => void }) { return <div className="voice-dialog-backdrop" role="dialog" aria-modal="true" aria-labelledby="incomplete-save-title"><div className="voice-dialog max-w-lg"><div className="voice-dialog-header"><div><h2 id="incomplete-save-title" className="text-base font-semibold">Untranslated entries remain</h2><p className="mt-2 text-sm leading-relaxed text-muted-foreground">{count} row{count === 1 ? " is" : "s are"} incomplete or failed. Choose how the target file should represent them.</p></div><button type="button" onClick={onCancel} className="app-button app-button-outline px-2" aria-label="Close"><X className="size-4" /></button></div><div className="mt-5 flex flex-wrap justify-end gap-2"><button type="button" onClick={onCancel} className="app-button app-button-outline">Cancel</button><button type="button" onClick={() => onSave("empty")} className="app-button app-button-outline">Save with empty translations</button><button type="button" onClick={() => onSave("source")} className="app-button app-button-primary">Save with original text</button></div></div></div>; }
function updateForm(row: LocalRow, category: string, text: string) { const targetForms = forms(row.translation); const values = targetForms.length > 0 ? targetForms : forms(row.source).map((form) => ({ category: form.category, text: "" })); return values.map((form) => form.category === category ? { ...form, text } : form); }
function originalForms(row: LocalizationEntry) { const targetForms = forms(row.translation); const categories = targetForms.length > 0 ? targetForms.map((form) => form.category) : forms(row.source).map((form) => form.category); return categories.map((category) => ({ category, text: sourceTextForCategory(forms(row.source), category) })); }
function translated(row: LocalizationEntry) { const targetForms = forms(row.translation); return targetForms.length > 0 && targetForms.every((form) => form.text.trim() !== ""); }
function hasTranslatableText(row: LocalizationEntry) { return forms(row.source).some((form) => form.text.trim() !== ""); }
function needsTranslation(row: LocalRow) { return row.status !== "skipped" && hasTranslatableText(row) && (!translated(row) || row.status === "failed"); }
function localizationRowStatus(row: LocalRow): DisplayRowStatus { return hasTranslatableText(row) ? row.status : "empty"; }
function matchesStatusFilter(row: LocalRow, filter: StatusFilter) { return filter === "all" || localizationRowStatus(row) === filter; }
function matchesLocalizationSearch(row: LocalizationEntry, query: string) { if (row.key.toLocaleLowerCase().includes(query)) return true; for (const form of forms(row.source)) { if (form.category.toLocaleLowerCase().includes(query) || form.text.toLocaleLowerCase().includes(query)) return true; } for (const form of forms(row.translation)) { if (form.category.toLocaleLowerCase().includes(query) || form.text.toLocaleLowerCase().includes(query)) return true; } return false; }
function groupLocalizationRows(rows: LocalRow[], separator: string, matches: (row: LocalRow) => boolean): LocalizationRowGroup[] { const groups = new Map<string, LocalizationRowGroup>(); const groupKey = (key: string) => { if (!separator) return ""; const index = key.lastIndexOf(separator); return index > 0 ? key.slice(0, index) : "Top-level keys"; }; for (const row of rows) { const key = groupKey(row.key); const group = groups.get(key) ?? { key, rows: [], allRows: [] }; group.allRows.push(row); if (matches(row)) group.rows.push(row); groups.set(key, group); } return [...groups.values()].filter((group) => group.rows.length > 0); }
function sourceTextForCategory(source: LocalizationEntry["source"], category: string) { for (const form of source) if (form.category === category) return form.text; for (const form of source) if (form.category === "other") return form.text; return source[0]?.text ?? ""; }
function countOccurrences(text: string, search: string) { let count = 0; let from = 0; while (from < text.length) { const index = text.indexOf(search, from); if (index < 0) break; count++; from = index + search.length; } return count; }
function forms<T>(value: T[] | null | undefined) { return value ?? []; }
function normalizeEntry(entry: LocalizationEntry): LocalizationEntry { return { ...entry, source: forms(entry.source), translation: forms(entry.translation) }; }
function message(reason: unknown, fallback: string) { return reason instanceof Error ? reason.message : fallback; }
function newOperationID() { return globalThis.crypto?.randomUUID?.() ?? `localization-${Date.now()}-${Math.random().toString(16).slice(2)}`; }

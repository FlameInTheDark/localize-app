import { lazy, Suspense, useEffect, useState } from "react";
import { ArrowLeft, Download, FileText, Files, Image as ImageIcon, ScanText, Settings, Type } from "lucide-react";
import { EventsOff, EventsOn } from "../wailsjs/runtime/runtime";
import type { OperationProgress, UpdateAvailability } from "@/types/api";
import { TextTranslator } from "@/components/TextTranslator";
import { DocumentTranslator } from "@/components/DocumentTranslator";
import { ImageTranslator } from "@/components/ImageTranslator";
import { OCRExtractor } from "@/components/OCRExtractor";
import { LocalizationTranslator } from "@/components/LocalizationTranslator";
import { SelectionContextMenu } from "@/components/SelectionContextMenu";
import { WindowControls } from "@/components/WindowControls";
import { api } from "@/lib/bridge";
import { useOperationStore, useWorkspaceStore, type WorkspaceTab } from "@/stores/workspace";

const SettingsView = lazy(() => import("@/components/SettingsSheet").then(({ SettingsView }) => ({ default: SettingsView })));
const tabs: Array<{ id: WorkspaceTab; label: string; Icon: typeof Type }> = [
  { id: "text", label: "Text", Icon: Type }, { id: "document", label: "Document", Icon: FileText },
  { id: "image", label: "Image", Icon: ImageIcon }, { id: "ocr", label: "OCR", Icon: ScanText }, { id: "localization", label: "Localization", Icon: Files },
];

export function App() {
  const tab = useWorkspaceStore((state) => state.tab);
  const setTab = useWorkspaceStore((state) => state.setTab);
  const ocrText = useWorkspaceStore((state) => state.ocrText);
  const setOCRText = useWorkspaceStore((state) => state.setOCRText);
  const setProgress = useOperationStore((state) => state.setProgress);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [theme, setTheme] = useState<ThemeMode>(getSavedTheme);
  const [drop, setDrop] = useState<{ path: string; token: number } | null>(null);
  const [availableUpdate, setAvailableUpdate] = useState<UpdateAvailability | null>(null);
  const [applicationVersion, setApplicationVersion] = useState("dev");

  useEffect(() => {
    EventsOn("operation:progress", (payload: OperationProgress) => setProgress(payload));
    return () => EventsOff("operation:progress");
  }, [setProgress]);

  useEffect(() => {
    EventsOn("wails:file-drop", (...data: unknown[]) => {
      const paths = data.find((entry): entry is string[] => Array.isArray(entry) && entry.every((value) => typeof value === "string"));
      if (paths?.[0]) setDrop({ path: paths[0], token: Date.now() });
    });
    return () => EventsOff("wails:file-drop");
  }, []);

  useEffect(() => {
    let active = true;
    void api.updateAvailability().then((update) => {
      if (active && update.available) setAvailableUpdate(update);
    }).catch(() => undefined);
    EventsOn("update:available", (update: UpdateAvailability) => {
      if (update.available) setAvailableUpdate(update);
    });
    return () => {
      active = false;
      EventsOff("update:available");
    };
  }, []);

  useEffect(() => {
    void api.applicationVersion().then(setApplicationVersion).catch(() => setApplicationVersion("dev"));
  }, []);

  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const apply = () => document.documentElement.classList.toggle("dark", theme === "dark" || (theme === "system" && media.matches));
    apply();
    localStorage.setItem("localize-theme", theme);
    if (theme !== "system") return undefined;
    media.addEventListener("change", apply);
    return () => media.removeEventListener("change", apply);
  }, [theme]);

  const dropFor = (target: WorkspaceTab) => drop && tab === target ? drop.path : undefined;

  return <div className="app-frame relative flex h-full min-h-0 flex-col overflow-hidden">
    <div aria-hidden className="pointer-events-none absolute inset-0 -z-10 overflow-hidden"><div className="absolute left-1/2 top-[-12rem] h-[28rem] w-[60rem] -translate-x-1/2 rounded-full bg-foreground/[.04] blur-3xl" /><div className="ambient-grid absolute inset-0" /></div>
    <header className="window-titlebar sticky top-0 z-30 bg-background/80 backdrop-blur-xl"><div className="grid h-11 w-full grid-cols-[1fr_auto_1fr] items-center pl-2 pr-0"><nav className="mode-switcher justify-self-start">{settingsOpen ? <button onClick={() => setSettingsOpen(false)} className="titlebar-nav-action"><ArrowLeft className="size-4" />Back</button> : tabs.map(({ id, label, Icon }) => <button key={id} data-active={tab === id} onClick={() => setTab(id)} className="tab-button"><Icon className="size-4" /><span className="hidden sm:inline">{label}</span></button>)}</nav><span className="window-app-name select-none text-sm font-semibold tracking-tight">Localize</span><nav className="window-actions flex items-center justify-self-end">{availableUpdate?.available ? <button type="button" onClick={() => void api.openLatestRelease()} className="window-update-control" title={`Open Localize ${availableUpdate.version} release`}><Download className="size-3.5" /><span>Update</span></button> : null}<button onClick={() => setSettingsOpen(true)} className="window-control" aria-label="Open settings"><Settings className="size-3.5" /></button><WindowControls /></nav></div></header>
    <main className="flex min-h-0 w-full flex-1 flex-col p-2 sm:p-3"><div className="flex h-full min-h-0 w-full flex-1 flex-col"><div className="min-h-0 flex-1">{settingsOpen ? <Suspense fallback={<div className="surface h-full animate-pulse" />}><SettingsView theme={theme} onThemeChange={setTheme} version={applicationVersion} /></Suspense> : <>{tab === "text" && <TextWorkspace seed={ocrText} clearSeed={() => setOCRText("")} />}{tab === "document" && <DocumentTranslator dropPath={dropFor("document")} />}{tab === "image" && <ImageTranslator dropPath={dropFor("image")} />}{tab === "ocr" && <OCRExtractor dropPath={dropFor("ocr")} onSend={(text) => { setOCRText(text); setTab("text"); }} />}{tab === "localization" && <LocalizationTranslator dropPath={dropFor("localization")} />}</>}</div></div></main>
    <SelectionContextMenu />
  </div>;
}

export type ThemeMode = "light" | "dark" | "system";

function getSavedTheme(): ThemeMode {
  const value = localStorage.getItem("localize-theme");
  return value === "light" || value === "dark" || value === "system" ? value : "system";
}

function TextWorkspace({ seed, clearSeed }: { seed: string; clearSeed: () => void }) {
  if (!seed) return <TextTranslator />;
  return <div><div className="mb-3 flex items-center justify-between rounded-lg border bg-muted/40 px-3 py-2 text-sm"><span>OCR text is ready to translate.</span><button className="app-button app-button-outline" onClick={clearSeed}>Dismiss</button></div><TextTranslator key={seed} initialInput={seed} /></div>;
}

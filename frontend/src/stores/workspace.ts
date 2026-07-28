import { create } from "zustand";
import type { OperationProgress } from "@/types/api";

export type WorkspaceTab = "text" | "document" | "image" | "ocr";

type WorkspaceState = {
  tab: WorkspaceTab;
  ocrText: string;
  setTab: (tab: WorkspaceTab) => void;
  setOCRText: (text: string) => void;
};

type OperationState = {
  progress: OperationProgress | null;
  setProgress: (progress: OperationProgress | null) => void;
};

export const useWorkspaceStore = create<WorkspaceState>((set) => ({
  tab: "text",
  ocrText: "",
  setTab: (tab) => set({ tab }),
  setOCRText: (ocrText) => set({ ocrText }),
}));

export const useOperationStore = create<OperationState>((set) => ({
  progress: null,
  setProgress: (progress) => set({ progress }),
}));

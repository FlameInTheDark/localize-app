export type ProviderKind = "ollama" | "llamacpp";
export type RuntimeMode = "auto" | "cpu" | "cuda";

export interface ModelAssignment { id: string; path?: string; projectionPath?: string; supportsVision: boolean; }
export interface Settings {
  version: number;
  activeProvider: ProviderKind;
  ollama: { endpoint: string; executable?: string; translation: ModelAssignment; vision: ModelAssignment };
  llamaCpp: { runtimeVersion?: string; runtimeMode: RuntimeMode; contextSize: number; modelDir?: string; translation: ModelAssignment; vision: ModelAssignment };
  whisperCpp: { runtimeVersion?: string; runtimeMode: RuntimeMode; modelDir?: string; model: WhisperModelAssignment; language: string; microphoneId?: string; microphoneName?: string };
  mupdf: { version?: string };
  prompts: PromptSettings;
}
export interface WhisperModelAssignment { id: string; path?: string; }
export interface PromptSettings { translation: string; detection: string; ocr: string; wordVariants: string; }
export interface ModelInfo { id: string; name: string; path?: string; projectionPath?: string; size: number; modifiedAt?: string; family?: string; parameters?: string; quantization?: string; supportsVision: boolean; running: boolean; }
export interface ProviderStatus { provider: ProviderKind; available: boolean; running: boolean; message: string; models: ModelInfo[]; }
export interface UpdateAvailability { available: boolean; version: string; url: string; }
export interface FileSelection { path: string; name: string; size: number; mimeType: string; previewUrl?: string; }
export interface ImageResult { original: string; translation: string; }
export interface DocumentSegment { ordinal: number; original: string; translation: string; }
export interface DocumentResult { segments: DocumentSegment[]; detected: number; extracted: number; translated: number; }
export interface TranslationVariantsRequest { sourceText: string; targetContext: string; markedTargetContext: string; selectedText: string; language: string; }
export interface TranslationVariant { target: string; replacement: string; }
export interface TranslationVariantsResult { variants: TranslationVariant[]; }
export interface HuggingFaceModel { id: string; author: string; downloads: number; likes: number; lastModified: string; tags: string[]; pipelineTag: string; }
export interface HuggingFaceFile { name: string; size: number; oid?: string; mmproj: boolean; }
export interface HuggingFaceInstallRequest { repository: string; modelFile: string; mmprojFile?: string; }
export interface LlamaCppRuntimeArtifact { url?: string; size: number; sha256?: string; }
export interface LlamaCppRelease { version: string; publishedAt?: string; cpu: LlamaCppRuntimeArtifact; cuda: LlamaCppRuntimeArtifact; }
export interface LlamaCppRuntimeInstallRequest { version: string; mode: Exclude<RuntimeMode, "auto">; }
export interface LlamaCppInstalledRuntime { version: string; cpuInstalled: boolean; cudaInstalled: boolean; }
export interface LlamaCppRuntimeStatus { root: string; selectedVersion?: string; installed: LlamaCppInstalledRuntime[]; }
export interface WhisperCppRelease { version: string; publishedAt?: string; cpu: LlamaCppRuntimeArtifact; cuda: LlamaCppRuntimeArtifact; }
export interface WhisperCppRuntimeInstallRequest { version: string; mode: Exclude<RuntimeMode, "auto">; }
export interface WhisperCppInstalledRuntime { version: string; cpuInstalled: boolean; cudaInstalled: boolean; }
export interface WhisperCppRuntimeStatus { root: string; selectedVersion?: string; installed: WhisperCppInstalledRuntime[]; }
export interface MuPDFRelease { version: string; publishedAt?: string; artifact: LlamaCppRuntimeArtifact; }
export interface MuPDFRuntimeInstallRequest { version: string; }
export interface MuPDFInstalledRuntime { version: string; }
export interface MuPDFRuntimeStatus { root: string; selectedVersion?: string; installed: MuPDFInstalledRuntime[]; }
export interface WhisperModel { id: string; name: string; path?: string; size: number; sha256?: string; installed: boolean; multilingual: boolean; }
export interface WhisperModelInstallRequest { id: string; }
export interface WhisperStatus { available: boolean; running: boolean; message: string; runtime?: string; model?: string; }
export interface AudioTranscriptionRequest { path: string; language?: string; }
export interface CapturedAudioTranscriptionRequest { wavBase64: string; language?: string; }
export interface AudioTranscriptionResult { text: string; language?: string; }
export interface OperationProgress { operationId: string; kind: string; stage: string; message: string; completed: number; total: number; speedBytesPerSecond: number; }

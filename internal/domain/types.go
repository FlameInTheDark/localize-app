package domain

import (
	"path/filepath"
	"strings"
)

type ProviderKind string

const (
	ProviderOllama   ProviderKind = "ollama"
	ProviderLlamaCpp ProviderKind = "llamacpp"
)

// IsTranslateGemmaModel reports whether an identifier belongs to the current
// TranslateGemma family. These GGUFs embed a specialised chat template which
// older llama.cpp releases cannot initialise for OpenAI-compatible requests.
// Keep the compatibility decision in the domain so the process manager and
// inference adapter cannot drift apart.
func IsTranslateGemmaModel(identifier string) bool {
	return strings.Contains(strings.ToLower(identifier), "translategemma")
}

type RuntimeMode string

const (
	RuntimeAuto   RuntimeMode = "auto"
	RuntimeCPU    RuntimeMode = "cpu"
	RuntimeCUDA   RuntimeMode = "cuda"
	RuntimeVulkan RuntimeMode = "vulkan"
	RuntimeHIP    RuntimeMode = "hip"
)

type ModelRole string

const (
	ModelRoleTranslation ModelRole = "translation"
	ModelRoleVision      ModelRole = "vision"
)

type ModelAssignment struct {
	ID             string `json:"id"`
	Path           string `json:"path,omitempty"`
	ProjectionPath string `json:"projectionPath,omitempty"`
	SupportsVision bool   `json:"supportsVision"`
}
type OllamaSettings struct {
	Endpoint    string          `json:"endpoint"`
	Executable  string          `json:"executable,omitempty"`
	Translation ModelAssignment `json:"translation"`
	Vision      ModelAssignment `json:"vision"`
}
type LlamaCppSettings struct {
	RuntimeVersion string          `json:"runtimeVersion,omitempty"`
	RuntimeMode    RuntimeMode     `json:"runtimeMode"`
	ContextSize    int             `json:"contextSize"`
	ModelDir       string          `json:"modelDir,omitempty"`
	Translation    ModelAssignment `json:"translation"`
	Vision         ModelAssignment `json:"vision"`
}

// WhisperModelAssignment identifies one locally installed Whisper model.
// Unlike inference models it is only usable by whisper.cpp transcription.
type WhisperModelAssignment struct {
	ID   string `json:"id"`
	Path string `json:"path,omitempty"`
}

// WhisperCppSettings keeps speech recognition independent from the inference
// provider selected for translation.
type WhisperCppSettings struct {
	RuntimeVersion string                 `json:"runtimeVersion,omitempty"`
	RuntimeMode    RuntimeMode            `json:"runtimeMode"`
	ModelDir       string                 `json:"modelDir,omitempty"`
	Model          WhisperModelAssignment `json:"model"`
	Language       string                 `json:"language"`
	MicrophoneID   string                 `json:"microphoneId,omitempty"`
	MicrophoneName string                 `json:"microphoneName,omitempty"`
}

// MuPDFSettings identifies the explicitly installed MuPDF command-line
// runtime used to extract and render documents.
type MuPDFSettings struct {
	Version string `json:"version,omitempty"`
}

type Settings struct {
	Version        int                `json:"version"`
	ActiveProvider ProviderKind       `json:"activeProvider"`
	Ollama         OllamaSettings     `json:"ollama"`
	LlamaCpp       LlamaCppSettings   `json:"llamaCpp"`
	WhisperCpp     WhisperCppSettings `json:"whisperCpp"`
	MuPDF          MuPDFSettings      `json:"mupdf"`
	Prompts        PromptSettings     `json:"prompts"`
}

func DefaultSettings(modelDir string) Settings {
	return Settings{Version: 7, ActiveProvider: ProviderOllama, Ollama: OllamaSettings{Endpoint: "http://127.0.0.1:11434", Translation: ModelAssignment{ID: "translategemma:latest", SupportsVision: true}, Vision: ModelAssignment{ID: "glm-ocr:latest", SupportsVision: true}}, LlamaCpp: LlamaCppSettings{RuntimeMode: RuntimeAuto, ContextSize: 8192, ModelDir: modelDir}, WhisperCpp: WhisperCppSettings{RuntimeMode: RuntimeAuto, ModelDir: filepath.Join(filepath.Dir(modelDir), "whisper-models"), Language: "auto"}, Prompts: DefaultPromptSettings()}
}

type ProviderStatus struct {
	Provider  ProviderKind `json:"provider"`
	Available bool         `json:"available"`
	Running   bool         `json:"running"`
	Message   string       `json:"message"`
	Models    []ModelInfo  `json:"models"`
}

// UpdateAvailability identifies a newer Localize release for the desktop title bar.
type UpdateAvailability struct {
	Available bool   `json:"available"`
	Version   string `json:"version"`
	URL       string `json:"url"`
}

type ModelInfo struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Path           string `json:"path,omitempty"`
	ProjectionPath string `json:"projectionPath,omitempty"`
	Size           int64  `json:"size"`
	ModifiedAt     string `json:"modifiedAt,omitempty"`
	Family         string `json:"family,omitempty"`
	Parameters     string `json:"parameters,omitempty"`
	Quantization   string `json:"quantization,omitempty"`
	SupportsVision bool   `json:"supportsVision"`
	Running        bool   `json:"running"`
}
type OperationProgress struct {
	OperationID         string `json:"operationId"`
	Kind                string `json:"kind"`
	Stage               string `json:"stage"`
	Message             string `json:"message"`
	Completed           int64  `json:"completed"`
	Total               int64  `json:"total"`
	SpeedBytesPerSecond int64  `json:"speedBytesPerSecond"`
}
type LlamaCppRuntimeArtifact struct {
	URL    string `json:"url,omitempty"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}
type LlamaCppRelease struct {
	Version     string                  `json:"version"`
	PublishedAt string                  `json:"publishedAt,omitempty"`
	CPU         LlamaCppRuntimeArtifact `json:"cpu"`
	CUDA        LlamaCppRuntimeArtifact `json:"cuda"`
	Vulkan      LlamaCppRuntimeArtifact `json:"vulkan"`
	HIP         LlamaCppRuntimeArtifact `json:"hip"`
}
type LlamaCppRuntimeInstallRequest struct {
	Version string      `json:"version"`
	Mode    RuntimeMode `json:"mode"`
}
type LlamaCppInstalledRuntime struct {
	Version         string `json:"version"`
	CPUInstalled    bool   `json:"cpuInstalled"`
	CUDAInstalled   bool   `json:"cudaInstalled"`
	VulkanInstalled bool   `json:"vulkanInstalled"`
	HIPInstalled    bool   `json:"hipInstalled"`
}
type LlamaCppRuntimeStatus struct {
	Root            string                     `json:"root"`
	SelectedVersion string                     `json:"selectedVersion,omitempty"`
	Installed       []LlamaCppInstalledRuntime `json:"installed"`
}

type WhisperCppRelease struct {
	Version     string                  `json:"version"`
	PublishedAt string                  `json:"publishedAt,omitempty"`
	CPU         LlamaCppRuntimeArtifact `json:"cpu"`
	CUDA        LlamaCppRuntimeArtifact `json:"cuda"`
}
type WhisperCppRuntimeInstallRequest struct {
	Version string      `json:"version"`
	Mode    RuntimeMode `json:"mode"`
}
type WhisperCppInstalledRuntime struct {
	Version       string `json:"version"`
	CPUInstalled  bool   `json:"cpuInstalled"`
	CUDAInstalled bool   `json:"cudaInstalled"`
}
type WhisperCppRuntimeStatus struct {
	Root            string                       `json:"root"`
	SelectedVersion string                       `json:"selectedVersion,omitempty"`
	Installed       []WhisperCppInstalledRuntime `json:"installed"`
}

// MuPDFRelease describes one curated official Windows x64 MuPDF archive.
type MuPDFRelease struct {
	Version     string                  `json:"version"`
	PublishedAt string                  `json:"publishedAt,omitempty"`
	Artifact    LlamaCppRuntimeArtifact `json:"artifact"`
}

type MuPDFRuntimeInstallRequest struct {
	Version string `json:"version"`
}

type MuPDFInstalledRuntime struct {
	Version string `json:"version"`
}

type MuPDFRuntimeStatus struct {
	Root            string                  `json:"root"`
	SelectedVersion string                  `json:"selectedVersion,omitempty"`
	Installed       []MuPDFInstalledRuntime `json:"installed"`
}

type WhisperModel struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Path         string `json:"path,omitempty"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256,omitempty"`
	Installed    bool   `json:"installed"`
	Multilingual bool   `json:"multilingual"`
}
type WhisperModelInstallRequest struct {
	ID string `json:"id"`
}
type WhisperStatus struct {
	Available bool   `json:"available"`
	Running   bool   `json:"running"`
	Message   string `json:"message"`
	Runtime   string `json:"runtime,omitempty"`
	Model     string `json:"model,omitempty"`
}
type AudioTranscriptionRequest struct {
	Path     string `json:"path"`
	Language string `json:"language,omitempty"`
}
type CapturedAudioTranscriptionRequest struct {
	WAVBase64 string `json:"wavBase64"`
	Language  string `json:"language,omitempty"`
}
type AudioTranscriptionResult struct {
	Text     string `json:"text"`
	Language string `json:"language,omitempty"`
}
type FileSelection struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	MimeType   string `json:"mimeType"`
	PreviewURL string `json:"previewUrl,omitempty"`
}
type TranslateRequest struct {
	Text     string `json:"text"`
	Language string `json:"language"`
}
type TranslationVariantsRequest struct {
	SourceText          string `json:"sourceText"`
	TargetContext       string `json:"targetContext"`
	MarkedTargetContext string `json:"markedTargetContext"`
	SelectedText        string `json:"selectedText"`
	Language            string `json:"language"`
}
type TranslationVariant struct {
	Target      string `json:"target"`
	Replacement string `json:"replacement"`
}
type TranslationVariantsResult struct {
	Variants []TranslationVariant `json:"variants"`
}
type ImageRequest struct {
	Path     string `json:"path"`
	Language string `json:"language"`
}
type DocumentRequest struct {
	Path     string `json:"path"`
	Language string `json:"language"`
}
type ImageResult struct {
	Original    string `json:"original"`
	Translation string `json:"translation"`
}
type DocumentSegment struct {
	Ordinal     int    `json:"ordinal"`
	Original    string `json:"original"`
	Translation string `json:"translation"`
}
type DocumentResult struct {
	Segments   []DocumentSegment `json:"segments"`
	Detected   int               `json:"detected"`
	Extracted  int               `json:"extracted"`
	Translated int               `json:"translated"`
}

// LocalizationFormat identifies a source localization-file syntax.
type LocalizationFormat string

const (
	LocalizationFormatAuto            LocalizationFormat = "auto"
	LocalizationFormatI18NextJSON     LocalizationFormat = "i18next-json"
	LocalizationFormatYAML            LocalizationFormat = "yaml"
	LocalizationFormatProperties      LocalizationFormat = "properties"
	LocalizationFormatGettextPO       LocalizationFormat = "gettext-po"
	LocalizationFormatSourceKeyValues LocalizationFormat = "source-keyvalues"
)

type LocalizationForm struct {
	Category string `json:"category"`
	Text     string `json:"text"`
}

// LocalizationEntry is a source value, or a grouped plural value, extracted
// from a localization file. Source values are immutable in the UI; only the
// translation forms are supplied back when the user saves a target file.
type LocalizationEntry struct {
	ID          string             `json:"id"`
	Key         string             `json:"key"`
	Source      []LocalizationForm `json:"source"`
	Translation []LocalizationForm `json:"translation"`
	Plural      bool               `json:"plural"`
}

type LocalizationFileRequest struct {
	Path   string             `json:"path"`
	Format LocalizationFormat `json:"format"`
}

type LocalizationFile struct {
	Path        string              `json:"path"`
	Name        string              `json:"name"`
	Format      LocalizationFormat  `json:"format"`
	Fingerprint string              `json:"fingerprint"`
	Entries     []LocalizationEntry `json:"entries"`
}

type LocalizationTranslationRequest struct {
	OperationID    string             `json:"operationId"`
	Path           string             `json:"path"`
	Format         LocalizationFormat `json:"format"`
	Fingerprint    string             `json:"fingerprint"`
	Language       string             `json:"language"`
	SourceLanguage string             `json:"sourceLanguage"`
	Rules          string             `json:"rules"`
	EntryIDs       []string           `json:"entryIds"`
}

type LocalizationTranslationResult struct {
	Entries    []LocalizationEntry `json:"entries"`
	Translated int                 `json:"translated"`
	Skipped    int                 `json:"skipped"`
	Failed     int                 `json:"failed"`
	Total      int                 `json:"total"`
}

type UntranslatedExportMode string

const (
	UntranslatedExportSource UntranslatedExportMode = "source"
	UntranslatedExportEmpty  UntranslatedExportMode = "empty"
)

type LocalizationSaveRequest struct {
	Path             string                 `json:"path"`
	Format           LocalizationFormat     `json:"format"`
	Fingerprint      string                 `json:"fingerprint"`
	Language         string                 `json:"language"`
	Entries          []LocalizationEntry    `json:"entries"`
	UntranslatedMode UntranslatedExportMode `json:"untranslatedMode"`
}

type LocalizationSaveResult struct {
	Path string `json:"path"`
}

// LocalizationProgress is sent on the localization:progress Wails event.
// Translation text stays in memory and is delivered only to the local UI.
type LocalizationProgress struct {
	OperationID string             `json:"operationId"`
	EntryID     string             `json:"entryId,omitempty"`
	Status      string             `json:"status"`
	Completed   int                `json:"completed"`
	Total       int                `json:"total"`
	Entry       *LocalizationEntry `json:"entry,omitempty"`
	Error       string             `json:"error,omitempty"`
}
type HuggingFaceModel struct {
	ID           string   `json:"id"`
	Author       string   `json:"author"`
	Downloads    int64    `json:"downloads"`
	Likes        int      `json:"likes"`
	LastModified string   `json:"lastModified"`
	Tags         []string `json:"tags"`
	PipelineTag  string   `json:"pipelineTag"`
}
type HuggingFaceFile struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	OID    string `json:"oid,omitempty"`
	MMProj bool   `json:"mmproj"`
}
type HuggingFaceInstallRequest struct {
	Repository string `json:"repository"`
	ModelFile  string `json:"modelFile"`
	MMProjFile string `json:"mmprojFile,omitempty"`
}

package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/FlameInTheDark/localize-app/internal/catalog"
	"github.com/FlameInTheDark/localize-app/internal/documents"
	"github.com/FlameInTheDark/localize-app/internal/inference"
	"github.com/FlameInTheDark/localize-app/internal/operations"
	llamaruntime "github.com/FlameInTheDark/localize-app/internal/runtime"
	"github.com/FlameInTheDark/localize-app/internal/settings"
	"github.com/FlameInTheDark/localize-app/internal/translation"
	"github.com/wailsapp/wails/v2/pkg/options"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const maxImageSize = 20 * 1024 * 1024
const maxDocumentSize = 100 * 1024 * 1024
const maxAudioSize = 1024 * 1024 * 1024
const maxCapturedAudioSize = 40 * 1024 * 1024

type Desktop struct {
	ctx             context.Context
	store           *settings.Store
	operations      *operations.Hub
	llama           *llamaruntime.LlamaManager
	runtimes        *llamaruntime.LlamaCatalog
	whisper         *llamaruntime.WhisperRunner
	whisperRuntimes *llamaruntime.WhisperCatalog
	mupdf           *llamaruntime.MuPDFCatalog
	hf              *catalog.HuggingFace
	whisperModels   *catalog.WhisperModels
	whisperTempRoot string
	translate       *translation.Service
	sequence        atomic.Uint64
}

func New() (*Desktop, error) {
	dataRoot, err := localDataRoot()
	if err != nil {
		return nil, fmt.Errorf("find application data directory: %w", err)
	}
	appDataDir := filepath.Join(dataRoot, "Localize")
	store, err := settings.New(appDataDir)
	if err != nil {
		return nil, err
	}
	operationsHub := &operations.Hub{}
	runtimeDir := filepath.Join(appDataDir, "runtime", "llama.cpp")
	whisperRuntimeDir := filepath.Join(appDataDir, "runtime", "whisper.cpp")
	mupdfRuntimeDir := filepath.Join(appDataDir, "runtime", "mupdf")
	whisperTempRoot := filepath.Join(appDataDir, "temp", "whisper")
	if err := os.RemoveAll(whisperTempRoot); err != nil {
		return nil, fmt.Errorf("clean previous Whisper temporary files: %w", err)
	}
	desktop := &Desktop{store: store, operations: operationsHub, llama: llamaruntime.NewLlamaManager(runtimeDir, operationsHub), runtimes: llamaruntime.NewLlamaCatalog(runtimeDir, operationsHub), whisper: llamaruntime.NewWhisperRunner(whisperRuntimeDir, whisperTempRoot, operationsHub), whisperRuntimes: llamaruntime.NewWhisperCatalog(whisperRuntimeDir, operationsHub), mupdf: llamaruntime.NewMuPDFCatalog(mupdfRuntimeDir, operationsHub), hf: catalog.NewHuggingFace(operationsHub), whisperModels: catalog.NewWhisperModels(operationsHub), whisperTempRoot: whisperTempRoot}
	desktop.translate = translation.New(desktop.inferenceClient, func() PromptSettings { return desktop.store.Get().Prompts })
	return desktop, nil
}

func (d *Desktop) Startup(ctx context.Context) {
	d.ctx = ctx
	d.operations.SetEmitter(func(progress OperationProgress) { wailsruntime.EventsEmit(ctx, "operation:progress", progress) })
}

func (d *Desktop) Shutdown(context.Context) { d.llama.Stop() }
func (d *Desktop) SecondInstance(options.SecondInstanceData) {
	if d.ctx != nil {
		wailsruntime.WindowUnminimise(d.ctx)
		wailsruntime.WindowShow(d.ctx)
	}
}

func (d *Desktop) GetSettings() Settings             { return d.store.Get() }
func (d *Desktop) GetDefaultPrompts() PromptSettings { return DefaultPromptSettings() }
func (d *Desktop) SaveSettings(next Settings) error {
	if next.ActiveProvider != ProviderOllama && next.ActiveProvider != ProviderLlamaCpp {
		return fmt.Errorf("unknown inference provider")
	}
	if _, err := inference.NormalizeEndpoint(next.Ollama.Endpoint); err != nil {
		return err
	}
	if next.LlamaCpp.ContextSize < 1024 || next.LlamaCpp.ContextSize > 131072 {
		return fmt.Errorf("context size must be between 1024 and 131072")
	}
	if next.LlamaCpp.RuntimeMode != RuntimeAuto && next.LlamaCpp.RuntimeMode != RuntimeCPU && next.LlamaCpp.RuntimeMode != RuntimeCUDA {
		return fmt.Errorf("unknown llama.cpp runtime mode")
	}
	if !filepath.IsAbs(next.LlamaCpp.ModelDir) {
		return fmt.Errorf("llama.cpp model directory must be an absolute path")
	}
	if err := os.MkdirAll(next.LlamaCpp.ModelDir, 0o700); err != nil {
		return fmt.Errorf("create llama.cpp model directory: %w", err)
	}
	if next.WhisperCpp.RuntimeMode != RuntimeAuto && next.WhisperCpp.RuntimeMode != RuntimeCPU && next.WhisperCpp.RuntimeMode != RuntimeCUDA {
		return fmt.Errorf("unknown Whisper.cpp runtime mode")
	}
	if !filepath.IsAbs(next.WhisperCpp.ModelDir) {
		return fmt.Errorf("whisper model directory must be an absolute path")
	}
	if err := os.MkdirAll(next.WhisperCpp.ModelDir, 0o700); err != nil {
		return fmt.Errorf("create Whisper model directory: %w", err)
	}
	next.WhisperCpp.Language = strings.TrimSpace(strings.ToLower(next.WhisperCpp.Language))
	if next.WhisperCpp.Language == "" {
		next.WhisperCpp.Language = "auto"
	}
	if err := next.Prompts.Validate(); err != nil {
		return err
	}
	if err := d.store.Save(next); err != nil {
		return err
	}
	d.llama.Stop()
	return nil
}

func (d *Desktop) GetProviderStatus() ProviderStatus {
	ctx, cancel := context.WithTimeout(d.context(), 4*time.Second)
	defer cancel()
	settings := d.store.Get()
	if settings.ActiveProvider == ProviderLlamaCpp {
		runtimeStatus, err := d.runtimes.Status(settings.LlamaCpp.RuntimeVersion)
		if err != nil {
			return ProviderStatus{Provider: ProviderLlamaCpp, Message: err.Error()}
		}
		available := runtimeInstalled(runtimeStatus, settings.LlamaCpp.RuntimeVersion, settings.LlamaCpp.RuntimeMode)
		message := d.llama.Describe()
		if !d.llama.Running() && !available {
			message = "Install and select a llama.cpp runtime in Settings"
		} else if !d.llama.Running() {
			message = "Runtime " + settings.LlamaCpp.RuntimeVersion + " is installed and ready to start"
		}
		return ProviderStatus{Provider: ProviderLlamaCpp, Available: available, Running: d.llama.Running(), Message: message}
	}
	client := inference.NewOllama(settings.Ollama.Endpoint)
	if err := client.Health(ctx); err != nil {
		return ProviderStatus{Provider: ProviderOllama, Message: "Ollama unavailable: " + err.Error()}
	}
	models, err := client.ListModels(ctx)
	if err != nil {
		return ProviderStatus{Provider: ProviderOllama, Available: true, Message: err.Error()}
	}
	return ProviderStatus{Provider: ProviderOllama, Available: true, Running: true, Message: "Connected", Models: models}
}

func (d *Desktop) ListActiveProviderModels() ([]ModelInfo, error) {
	ctx, cancel := context.WithTimeout(d.context(), 8*time.Second)
	defer cancel()
	settings := d.store.Get()
	if settings.ActiveProvider == ProviderLlamaCpp {
		return catalog.LocalModels(settings.LlamaCpp.ModelDir)
	}
	return inference.NewOllama(settings.Ollama.Endpoint).ListModels(ctx)
}

func (d *Desktop) ListOllamaModels() ([]ModelInfo, error) {
	ctx, cancel := context.WithTimeout(d.context(), 20*time.Second)
	defer cancel()
	return inference.NewOllama(d.store.Get().Ollama.Endpoint).ListModels(ctx)
}

func (d *Desktop) PullOllamaModel(model string) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return fmt.Errorf("model name is required")
	}
	operationID := d.nextOperation("ollama")
	ctx := d.context()
	return inference.NewOllama(d.store.Get().Ollama.Endpoint).Pull(ctx, model, operationID, d.operations.Emit)
}
func (d *Desktop) DeleteOllamaModel(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("model name is required")
	}
	ctx, cancel := context.WithTimeout(d.context(), 30*time.Second)
	defer cancel()
	if err := inference.NewOllama(d.store.Get().Ollama.Endpoint).Delete(ctx, id); err != nil {
		return err
	}
	current := d.store.Get()
	if current.Ollama.Translation.ID == id {
		current.Ollama.Translation = ModelAssignment{}
	}
	if current.Ollama.Vision.ID == id {
		current.Ollama.Vision = ModelAssignment{}
	}
	return d.store.Save(current)
}

func (d *Desktop) OpenOllamaCatalog() {
	wailsruntime.BrowserOpenURL(d.context(), "https://ollama.com/search")
}
func (d *Desktop) SearchHuggingFaceModels(query string) ([]HuggingFaceModel, error) {
	return d.hf.Search(d.context(), query)
}
func (d *Desktop) GetHuggingFaceFiles(repository string) ([]HuggingFaceFile, error) {
	return d.hf.Files(d.context(), repository)
}
func (d *Desktop) InstallHuggingFaceModel(request HuggingFaceInstallRequest) (ModelInfo, error) {
	settings := d.store.Get()
	return d.hf.Install(d.context(), request, settings.LlamaCpp.ModelDir, d.nextOperation("huggingface"))
}
func (d *Desktop) ListLocalLlamaModels() ([]ModelInfo, error) {
	return catalog.LocalModels(d.store.Get().LlamaCpp.ModelDir)
}
func (d *Desktop) PickLlamaModelDirectory() (string, error) {
	path, err := wailsruntime.OpenDirectoryDialog(d.context(), wailsruntime.OpenDialogOptions{Title: "Choose llama.cpp model directory"})
	if err != nil || path == "" {
		return "", err
	}
	return filepath.Clean(path), nil
}
func (d *Desktop) ListLlamaCppReleases() ([]LlamaCppRelease, error) {
	ctx, cancel := context.WithTimeout(d.context(), 20*time.Second)
	defer cancel()
	return d.runtimes.List(ctx)
}
func (d *Desktop) GetLlamaCppRuntimeStatus() (LlamaCppRuntimeStatus, error) {
	return d.runtimes.Status(d.store.Get().LlamaCpp.RuntimeVersion)
}
func (d *Desktop) InstallLlamaCppRuntime(request LlamaCppRuntimeInstallRequest) error {
	d.llama.Stop()
	return d.runtimes.Install(d.context(), request, d.nextOperation("llamacpp-install"))
}
func (d *Desktop) DeleteLocalLlamaModel(id string) error {
	d.llama.Stop()
	current := d.store.Get()
	if err := catalog.DeleteLocalModel(current.LlamaCpp.ModelDir, id); err != nil {
		return err
	}
	if current.LlamaCpp.Translation.ID == id {
		current.LlamaCpp.Translation = ModelAssignment{}
	}
	if current.LlamaCpp.Vision.ID == id {
		current.LlamaCpp.Vision = ModelAssignment{}
	}
	return d.store.Save(current)
}

func (d *Desktop) ListWhisperCppReleases() ([]WhisperCppRelease, error) {
	ctx, cancel := context.WithTimeout(d.context(), 20*time.Second)
	defer cancel()
	return d.whisperRuntimes.List(ctx)
}

func (d *Desktop) GetWhisperCppRuntimeStatus() (WhisperCppRuntimeStatus, error) {
	return d.whisperRuntimes.Status(d.store.Get().WhisperCpp.RuntimeVersion)
}

func (d *Desktop) InstallWhisperCppRuntime(request WhisperCppRuntimeInstallRequest) error {
	return d.whisperRuntimes.Install(d.context(), request, d.nextOperation("whispercpp-install"))
}

func (d *Desktop) ListMuPDFReleases() ([]MuPDFRelease, error) {
	ctx, cancel := context.WithTimeout(d.context(), 20*time.Second)
	defer cancel()
	return d.mupdf.List(ctx)
}

func (d *Desktop) GetMuPDFRuntimeStatus() (MuPDFRuntimeStatus, error) {
	return d.mupdf.Status(d.store.Get().MuPDF.Version)
}

func (d *Desktop) InstallMuPDFRuntime(request MuPDFRuntimeInstallRequest) error {
	return d.mupdf.Install(d.context(), request, d.nextOperation("mupdf-install"))
}

func (d *Desktop) GetWhisperStatus() WhisperStatus {
	ctx, cancel := context.WithTimeout(d.context(), 10*time.Second)
	defer cancel()
	return d.whisper.Status(ctx, d.store.Get().WhisperCpp)
}

func (d *Desktop) ListWhisperModels() ([]WhisperModel, error) {
	settings := d.store.Get()
	ctx, cancel := context.WithTimeout(d.context(), 30*time.Second)
	defer cancel()
	return d.whisperModels.List(ctx, settings.WhisperCpp.ModelDir)
}

func (d *Desktop) InstallWhisperModel(request WhisperModelInstallRequest) (WhisperModel, error) {
	settings := d.store.Get()
	return d.whisperModels.Install(d.context(), request, settings.WhisperCpp.ModelDir, d.nextOperation("whisper-model"))
}

func (d *Desktop) DeleteWhisperModel(id string) error {
	current := d.store.Get()
	if err := d.whisperModels.Delete(current.WhisperCpp.ModelDir, id); err != nil {
		return err
	}
	if current.WhisperCpp.Model.ID == id {
		current.WhisperCpp.Model = WhisperModelAssignment{}
	}
	return d.store.Save(current)
}

func (d *Desktop) PickWhisperModelDirectory() (string, error) {
	path, err := wailsruntime.OpenDirectoryDialog(d.context(), wailsruntime.OpenDialogOptions{Title: "Choose Whisper model directory"})
	if err != nil || path == "" {
		return "", err
	}
	return filepath.Clean(path), nil
}

func (d *Desktop) PickFile(kind string) (FileSelection, error) {
	var filters []wailsruntime.FileFilter
	switch kind {
	case "image":
		filters = []wailsruntime.FileFilter{{DisplayName: "Images", Pattern: "*.png;*.jpg;*.jpeg;*.webp;*.gif;*.bmp"}}
	case "document":
		filters = []wailsruntime.FileFilter{{DisplayName: "Documents", Pattern: "*.pdf;*.epub;*.mobi;*.docx;*.xlsx;*.pptx"}}
	case "audio":
		filters = []wailsruntime.FileFilter{{DisplayName: "Audio supported by Whisper", Pattern: "*.wav;*.mp3;*.flac;*.ogg"}}
	default:
		return FileSelection{}, fmt.Errorf("unknown file type")
	}
	path, err := wailsruntime.OpenFileDialog(d.context(), wailsruntime.OpenDialogOptions{Title: "Select " + kind, Filters: filters})
	if err != nil || path == "" {
		return FileSelection{}, err
	}
	return d.LoadFile(path, kind)
}

func (d *Desktop) LoadFile(path, kind string) (FileSelection, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileSelection{}, err
	}
	if info.IsDir() {
		return FileSelection{}, fmt.Errorf("select a file, not a directory")
	}
	limit := int64(maxDocumentSize)
	if kind == "image" {
		limit = maxImageSize
	}
	if kind == "audio" {
		limit = maxAudioSize
	}
	if info.Size() > limit {
		return FileSelection{}, fmt.Errorf("file is too large (maximum %d MB)", limit/(1024*1024))
	}
	selection := FileSelection{Path: path, Name: filepath.Base(path), Size: info.Size(), MimeType: mime.TypeByExtension(filepath.Ext(path))}
	if kind == "image" {
		data, err := os.ReadFile(path)
		if err != nil {
			return FileSelection{}, err
		}
		detected := http.DetectContentType(data)
		if !strings.HasPrefix(detected, "image/") {
			return FileSelection{}, fmt.Errorf("unsupported image type")
		}
		selection.MimeType = detected
		selection.PreviewURL = "data:" + detected + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	if kind == "document" && !documents.IsSupported(path) {
		return FileSelection{}, fmt.Errorf("unsupported document type")
	}
	if kind == "audio" && !supportedAudio(path) {
		return FileSelection{}, fmt.Errorf("unsupported audio type; choose WAV, MP3, FLAC, or OGG")
	}
	return selection, nil
}

func (d *Desktop) TranslateText(request TranslateRequest) (string, error) {
	return d.translate.Translate(d.context(), request.Text, request.Language)
}
func (d *Desktop) GetTranslationVariants(request TranslationVariantsRequest) (TranslationVariantsResult, error) {
	ctx, cancel := context.WithTimeout(d.context(), 45*time.Second)
	defer cancel()
	return d.translate.Variants(ctx, request)
}
func (d *Desktop) DetectLanguage(text string) ([]string, error) {
	return d.translate.Detect(d.context(), text)
}

func (d *Desktop) TranscribeAudio(request AudioTranscriptionRequest) (AudioTranscriptionResult, error) {
	selection, err := d.LoadFile(request.Path, "audio")
	if err != nil {
		return AudioTranscriptionResult{}, err
	}
	return d.whisper.Transcribe(d.context(), d.store.Get().WhisperCpp, selection.Path, request.Language, d.nextOperation("whisper-transcribe"))
}

func (d *Desktop) TranscribeCapturedAudio(request CapturedAudioTranscriptionRequest) (AudioTranscriptionResult, error) {
	data, err := base64.StdEncoding.DecodeString(request.WAVBase64)
	if err != nil {
		return AudioTranscriptionResult{}, fmt.Errorf("decode recorded audio: %w", err)
	}
	if len(data) < 44 || len(data) > maxCapturedAudioSize {
		return AudioTranscriptionResult{}, fmt.Errorf("recorded audio must be a valid WAV file smaller than %d MB", maxCapturedAudioSize/(1024*1024))
	}
	valid, hasSpeech := capturedWAVHasSpeech(data)
	if !valid {
		return AudioTranscriptionResult{}, fmt.Errorf("recorded audio must be a 16-bit PCM WAV file")
	}
	if !hasSpeech {
		return AudioTranscriptionResult{}, nil
	}
	if err := os.MkdirAll(d.whisperTempRoot, 0o700); err != nil {
		return AudioTranscriptionResult{}, err
	}
	directory, err := os.MkdirTemp(d.whisperTempRoot, "recording-")
	if err != nil {
		return AudioTranscriptionResult{}, err
	}
	defer func() { _ = os.RemoveAll(directory) }()
	path := filepath.Join(directory, "recording.wav")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return AudioTranscriptionResult{}, err
	}
	return d.whisper.Transcribe(d.context(), d.store.Get().WhisperCpp, path, request.Language, d.nextOperation("whisper-transcribe"))
}

func (d *Desktop) ExtractImageText(path string) (string, error) {
	data, err := d.readImage(path)
	if err != nil {
		return "", err
	}
	return d.translate.OCR(d.context(), data)
}
func (d *Desktop) TranslateImage(request ImageRequest) (ImageResult, error) {
	data, err := d.readImage(request.Path)
	if err != nil {
		return ImageResult{}, err
	}
	original, err := d.translate.OCR(d.context(), data)
	if err != nil {
		return ImageResult{}, err
	}
	translated, err := d.translate.Translate(d.context(), original, request.Language)
	if err != nil {
		return ImageResult{}, err
	}
	return ImageResult{Original: original, Translation: translated}, nil
}

func (d *Desktop) TranslateDocument(request DocumentRequest) (DocumentResult, error) {
	if strings.TrimSpace(request.Language) == "" {
		return DocumentResult{}, fmt.Errorf("target language is required")
	}
	selection, err := d.LoadFile(request.Path, "document")
	if err != nil {
		return DocumentResult{}, err
	}
	mutool, err := d.mupdf.Mutool(d.store.Get().MuPDF.Version)
	if err != nil {
		return DocumentResult{}, err
	}
	extractor := documents.New(mutool)
	operationID := d.nextOperation("document")
	d.operations.Emit(OperationProgress{OperationID: operationID, Kind: "document", Stage: "extract", Message: "Extracting document text"})
	segments, err := extractor.Extract(d.context(), selection.Path)
	if err != nil {
		return DocumentResult{}, err
	}
	if len(segments) == 0 {
		d.operations.Emit(OperationProgress{OperationID: operationID, Kind: "document", Stage: "ocr", Message: "No embedded text found; recognizing rendered pages"})
		pages, err := extractor.Render(d.context(), selection.Path)
		if err != nil {
			return DocumentResult{}, err
		}
		defer func() { _ = os.RemoveAll(filepath.Dir(pages[0])) }()
		segments = make([]DocumentSegment, 0, len(pages))
		for index, page := range pages {
			image, err := os.ReadFile(page)
			if err != nil {
				return DocumentResult{}, fmt.Errorf("read rendered page %d: %w", index+1, err)
			}
			text, err := d.translate.OCR(d.context(), image)
			if err != nil {
				return DocumentResult{}, fmt.Errorf("recognize page %d: %w", index+1, err)
			}
			if text = strings.TrimSpace(text); text != "" {
				segments = append(segments, DocumentSegment{Ordinal: len(segments) + 1, Original: text})
			}
			d.operations.Emit(OperationProgress{OperationID: operationID, Kind: "document", Stage: "ocr", Message: fmt.Sprintf("Recognized page %d of %d", index+1, len(pages)), Completed: int64(index + 1), Total: int64(len(pages))})
		}
		if len(segments) == 0 {
			return DocumentResult{}, fmt.Errorf("no readable text was found in the document or rendered pages")
		}
	}
	result := DocumentResult{Segments: segments, Detected: len(segments), Extracted: len(segments)}
	for index := range result.Segments {
		chunks := translation.SplitText(result.Segments[index].Original, 4000)
		translated := make([]string, 0, len(chunks))
		for _, chunk := range chunks {
			text, err := d.translate.Translate(d.context(), chunk, request.Language)
			if err != nil {
				return result, err
			}
			translated = append(translated, text)
		}
		result.Segments[index].Translation = strings.Join(translated, "\n\n")
		result.Translated++
		d.operations.Emit(OperationProgress{OperationID: operationID, Kind: "document", Stage: "translate", Message: fmt.Sprintf("Translated section %d of %d", index+1, len(result.Segments)), Completed: int64(index + 1), Total: int64(len(result.Segments))})
	}
	return result, nil
}

func (d *Desktop) inferenceClient(ctx context.Context, vision bool) (inference.Client, string, error) {
	settings := d.store.Get()
	if settings.ActiveProvider == ProviderOllama {
		assignment := settings.Ollama.Translation
		if vision {
			assignment = settings.Ollama.Vision
		}
		if assignment.ID == "" {
			return nil, "", fmt.Errorf("select a %s Ollama model in Settings", roleName(vision))
		}
		client := inference.NewOllama(settings.Ollama.Endpoint)
		if err := client.Health(ctx); err != nil {
			return nil, "", fmt.Errorf("ollama is unavailable: %w", err)
		}
		if err := client.EnsureModel(ctx, assignment.ID); err != nil {
			return nil, "", err
		}
		return client, assignment.ID, nil
	}
	assignment := settings.LlamaCpp.Translation
	if vision {
		assignment = settings.LlamaCpp.Vision
	}
	if vision && !assignment.SupportsVision {
		return nil, "", fmt.Errorf("select a vision-capable llama.cpp model and projection file in Settings")
	}
	endpoint, err := d.llama.Endpoint(ctx, settings.LlamaCpp, assignment, d.nextOperation("llamacpp"))
	if err != nil {
		return nil, "", err
	}
	return inference.NewLlamaCpp(endpoint), assignment.ID, nil
}

func (d *Desktop) readImage(path string) ([]byte, error) {
	selection, err := d.LoadFile(path, "image")
	if err != nil {
		return nil, err
	}
	return os.ReadFile(selection.Path)
}
func (d *Desktop) context() context.Context {
	if d.ctx != nil {
		return d.ctx
	}
	return context.Background()
}
func (d *Desktop) nextOperation(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixMilli(), d.sequence.Add(1))
}
func roleName(vision bool) string {
	if vision {
		return "vision/OCR"
	}
	return "translation"
}

func localDataRoot() (string, error) {
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		return local, nil
	}
	return os.UserCacheDir()
}

func runtimeInstalled(status LlamaCppRuntimeStatus, version string, mode RuntimeMode) bool {
	for _, installed := range status.Installed {
		if installed.Version != version {
			continue
		}
		switch mode {
		case RuntimeCPU:
			return installed.CPUInstalled
		case RuntimeCUDA:
			return installed.CUDAInstalled
		default:
			if _, err := exec.LookPath("nvidia-smi"); err == nil && installed.CUDAInstalled {
				return true
			}
			return installed.CPUInstalled
		}
	}
	return false
}

func supportedAudio(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".wav", ".mp3", ".flac", ".ogg":
		return true
	default:
		return false
	}
}

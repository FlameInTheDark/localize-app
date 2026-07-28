package app

import "github.com/FlameInTheDark/localize-app/internal/domain"

type ProviderKind = domain.ProviderKind
type RuntimeMode = domain.RuntimeMode
type ModelRole = domain.ModelRole
type ModelAssignment = domain.ModelAssignment
type OllamaSettings = domain.OllamaSettings
type LlamaCppSettings = domain.LlamaCppSettings
type WhisperCppSettings = domain.WhisperCppSettings
type MuPDFSettings = domain.MuPDFSettings
type WhisperModelAssignment = domain.WhisperModelAssignment
type Settings = domain.Settings
type PromptSettings = domain.PromptSettings
type ProviderStatus = domain.ProviderStatus
type ModelInfo = domain.ModelInfo
type OperationProgress = domain.OperationProgress
type FileSelection = domain.FileSelection
type TranslateRequest = domain.TranslateRequest
type TranslationVariantsRequest = domain.TranslationVariantsRequest
type TranslationVariant = domain.TranslationVariant
type TranslationVariantsResult = domain.TranslationVariantsResult
type ImageRequest = domain.ImageRequest
type DocumentRequest = domain.DocumentRequest
type ImageResult = domain.ImageResult
type DocumentSegment = domain.DocumentSegment
type DocumentResult = domain.DocumentResult
type HuggingFaceModel = domain.HuggingFaceModel
type HuggingFaceFile = domain.HuggingFaceFile
type HuggingFaceInstallRequest = domain.HuggingFaceInstallRequest
type LlamaCppRelease = domain.LlamaCppRelease
type LlamaCppRuntimeInstallRequest = domain.LlamaCppRuntimeInstallRequest
type LlamaCppInstalledRuntime = domain.LlamaCppInstalledRuntime
type LlamaCppRuntimeStatus = domain.LlamaCppRuntimeStatus
type WhisperCppRelease = domain.WhisperCppRelease
type WhisperCppRuntimeInstallRequest = domain.WhisperCppRuntimeInstallRequest
type WhisperCppRuntimeStatus = domain.WhisperCppRuntimeStatus
type MuPDFRelease = domain.MuPDFRelease
type MuPDFRuntimeInstallRequest = domain.MuPDFRuntimeInstallRequest
type MuPDFInstalledRuntime = domain.MuPDFInstalledRuntime
type MuPDFRuntimeStatus = domain.MuPDFRuntimeStatus
type WhisperModel = domain.WhisperModel
type WhisperModelInstallRequest = domain.WhisperModelInstallRequest
type WhisperStatus = domain.WhisperStatus
type AudioTranscriptionRequest = domain.AudioTranscriptionRequest
type CapturedAudioTranscriptionRequest = domain.CapturedAudioTranscriptionRequest
type AudioTranscriptionResult = domain.AudioTranscriptionResult

const (
	ProviderOllama       = domain.ProviderOllama
	ProviderLlamaCpp     = domain.ProviderLlamaCpp
	RuntimeAuto          = domain.RuntimeAuto
	RuntimeCPU           = domain.RuntimeCPU
	RuntimeCUDA          = domain.RuntimeCUDA
	ModelRoleTranslation = domain.ModelRoleTranslation
	ModelRoleVision      = domain.ModelRoleVision
)

func DefaultSettings(modelDir string) Settings { return domain.DefaultSettings(modelDir) }
func DefaultPromptSettings() PromptSettings    { return domain.DefaultPromptSettings() }

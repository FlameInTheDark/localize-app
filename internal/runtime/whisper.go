package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/FlameInTheDark/localize-app/internal/domain"
	"github.com/FlameInTheDark/localize-app/internal/operations"
)

// WhisperRunner owns only short-lived whisper-cli child processes. Models and
// runtime binaries remain user-managed files; every operation receives a
// unique temporary directory which is removed before returning.
type WhisperRunner struct {
	runtimeDir string
	tempRoot   string
	operations *operations.Hub
}

func NewWhisperRunner(runtimeDir, tempRoot string, hub *operations.Hub) *WhisperRunner {
	return &WhisperRunner{runtimeDir: runtimeDir, tempRoot: tempRoot, operations: hub}
}

func (r *WhisperRunner) Status(ctx context.Context, settings domain.WhisperCppSettings) domain.WhisperStatus {
	if strings.TrimSpace(settings.Model.ID) == "" || strings.TrimSpace(settings.Model.Path) == "" {
		return domain.WhisperStatus{Message: "Install and select a Whisper model in Settings"}
	}
	if _, err := os.Stat(settings.Model.Path); err != nil {
		return domain.WhisperStatus{Message: "Configured Whisper model is missing: " + err.Error(), Model: settings.Model.ID}
	}
	mode, binary, err := r.chooseBinary(settings)
	if err != nil {
		return domain.WhisperStatus{Message: err.Error(), Model: settings.Model.ID}
	}
	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := probeWhisper(probeCtx, binary); err != nil {
		return domain.WhisperStatus{Message: "Whisper runtime is unavailable: " + err.Error(), Runtime: settings.RuntimeVersion, Model: settings.Model.ID}
	}
	return domain.WhisperStatus{Available: true, Message: "Whisper.cpp " + string(mode) + " runtime is ready", Runtime: settings.RuntimeVersion, Model: settings.Model.ID}
}

func (r *WhisperRunner) Transcribe(ctx context.Context, settings domain.WhisperCppSettings, path, language, operationID string) (domain.AudioTranscriptionResult, error) {
	if goruntime.GOOS != "windows" {
		return domain.AudioTranscriptionResult{}, errors.New("whisper.cpp transcription is currently available on Windows only")
	}
	if strings.TrimSpace(settings.Model.ID) == "" || strings.TrimSpace(settings.Model.Path) == "" {
		return domain.AudioTranscriptionResult{}, errors.New("install and select a Whisper model in Settings before transcribing")
	}
	if _, err := os.Stat(settings.Model.Path); err != nil {
		return domain.AudioTranscriptionResult{}, fmt.Errorf("configured Whisper model is missing: %w", err)
	}
	if _, err := os.Stat(path); err != nil {
		return domain.AudioTranscriptionResult{}, fmt.Errorf("audio file is unavailable: %w", err)
	}
	if strings.TrimSpace(language) == "" {
		language = settings.Language
	}
	if strings.TrimSpace(language) == "" {
		language = "auto"
	}
	mode, binary, err := r.chooseBinary(settings)
	if err != nil {
		return domain.AudioTranscriptionResult{}, err
	}
	r.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "whispercpp-transcription", Stage: "starting", Message: "Starting Whisper transcription"})
	result, err := r.transcribeWith(ctx, binary, mode, settings.Model.Path, path, language, operationID)
	if err == nil || settings.RuntimeMode != domain.RuntimeAuto || mode != domain.RuntimeCUDA {
		return result, err
	}
	cpu := r.binary(settings.RuntimeVersion, domain.RuntimeCPU)
	if !fileExists(cpu) {
		return domain.AudioTranscriptionResult{}, err
	}
	r.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "whispercpp-transcription", Stage: "fallback", Message: "CUDA transcription failed; retrying on CPU"})
	return r.transcribeWith(ctx, cpu, domain.RuntimeCPU, settings.Model.Path, path, language, operationID)
}

func (r *WhisperRunner) chooseBinary(settings domain.WhisperCppSettings) (domain.RuntimeMode, string, error) {
	if goruntime.GOOS != "windows" {
		return "", "", errors.New("the managed Whisper runtime is currently Windows-only")
	}
	version := strings.TrimSpace(settings.RuntimeVersion)
	if version == "" {
		return "", "", errors.New("install and select a Whisper.cpp runtime version in Settings before transcribing")
	}
	mode := settings.RuntimeMode
	if mode == domain.RuntimeAuto {
		if _, err := exec.LookPath("nvidia-smi"); err == nil {
			cuda := r.binary(version, domain.RuntimeCUDA)
			if fileExists(cuda) {
				probeCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				probeErr := probeWhisper(probeCtx, cuda)
				cancel()
				if probeErr == nil {
					return domain.RuntimeCUDA, cuda, nil
				}
			}
		}
		mode = domain.RuntimeCPU
	}
	binary := r.binary(version, mode)
	if !fileExists(binary) {
		return "", "", fmt.Errorf("%s Whisper.cpp runtime %s is not installed; install it in Settings", mode, version)
	}
	return mode, binary, nil
}

func (r *WhisperRunner) binary(version string, mode domain.RuntimeMode) string {
	return filepath.Join(r.runtimeDir, version, string(mode), "whisper-cli.exe")
}

func (r *WhisperRunner) transcribeWith(ctx context.Context, binary string, mode domain.RuntimeMode, model, audio, language, operationID string) (domain.AudioTranscriptionResult, error) {
	if err := os.MkdirAll(r.tempRoot, 0o700); err != nil {
		return domain.AudioTranscriptionResult{}, fmt.Errorf("create Whisper temporary directory: %w", err)
	}
	temporary, err := os.MkdirTemp(r.tempRoot, "transcription-")
	if err != nil {
		return domain.AudioTranscriptionResult{}, err
	}
	defer func() { _ = os.RemoveAll(temporary) }()
	output := filepath.Join(temporary, "result")
	args := whisperArguments(model, audio, output, language, mode)
	commandCtx, cancel := context.WithTimeout(ctx, 12*time.Minute)
	defer cancel()
	command := exec.CommandContext(commandCtx, binary, args...)
	command.Dir = filepath.Dir(binary)
	outputLog := &processOutput{}
	command.Stdout, command.Stderr = io.MultiWriter(os.Stderr, outputLog), io.MultiWriter(os.Stderr, outputLog)
	if err := command.Run(); err != nil {
		if commandCtx.Err() != nil {
			return domain.AudioTranscriptionResult{}, fmt.Errorf("whisper transcription cancelled or timed out: %w", commandCtx.Err())
		}
		return domain.AudioTranscriptionResult{}, fmt.Errorf("whisper transcription failed: %w. Runtime output: %s", err, outputLog.String())
	}
	data, err := os.ReadFile(output + ".json")
	if err != nil {
		return domain.AudioTranscriptionResult{}, fmt.Errorf("read Whisper transcription result: %w", err)
	}
	result, err := parseWhisperResult(data)
	if err != nil {
		return domain.AudioTranscriptionResult{}, err
	}
	r.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "whispercpp-transcription", Stage: "complete", Message: "Whisper transcription is ready"})
	return result, nil
}

// whisperArguments keeps the CLI safety policy in one place. In particular,
// suppressing non-speech tokens and lowering the no-speech threshold avoids
// well-known Whisper hallucinations (for example, "Thank you") in quiet audio.
func whisperArguments(model, audio, output, language string, mode domain.RuntimeMode) []string {
	args := []string{
		"-m", model,
		"-f", audio,
		"-oj",
		"-of", output,
		"-np",
		"-nt",
		"-sns",
		"-nth", "0.35",
		"-l", language,
	}
	if mode == domain.RuntimeCPU {
		args = append(args, "-ng")
	}
	return args
}

func probeWhisper(ctx context.Context, binary string) error {
	command := exec.CommandContext(ctx, binary, "--version")
	command.Dir = filepath.Dir(binary)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	if !strings.Contains(strings.ToLower(string(output)), "whisper.cpp") {
		return errors.New("did not report a whisper.cpp version")
	}
	return nil
}

func parseWhisperResult(data []byte) (domain.AudioTranscriptionResult, error) {
	var parsed struct {
		Result struct {
			Language      string `json:"language"`
			Transcription []struct {
				Text string `json:"text"`
			} `json:"transcription"`
		} `json:"result"`
		Transcription []struct {
			Text string `json:"text"`
		} `json:"transcription"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return domain.AudioTranscriptionResult{}, fmt.Errorf("decode Whisper JSON result: %w", err)
	}
	segments := parsed.Result.Transcription
	if len(segments) == 0 {
		segments = parsed.Transcription
	}
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		if text := strings.TrimSpace(segment.Text); text != "" {
			parts = append(parts, text)
		}
	}
	text := strings.TrimSpace(strings.Join(parts, " "))
	if text == "" {
		return domain.AudioTranscriptionResult{}, errors.New("whisper returned no speech")
	}
	return domain.AudioTranscriptionResult{Text: text, Language: parsed.Result.Language}, nil
}

package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

type Store struct {
	mu       sync.RWMutex
	path     string
	settings domain.Settings
}

func New(appDir string) (*Store, error) {
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return nil, fmt.Errorf("create application data directory: %w", err)
	}
	modelDir := filepath.Join(appDir, "models")
	if err := os.MkdirAll(modelDir, 0o700); err != nil {
		return nil, fmt.Errorf("create model directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(appDir, "whisper-models"), 0o700); err != nil {
		return nil, fmt.Errorf("create whisper model directory: %w", err)
	}
	store := &Store{path: filepath.Join(appDir, "settings.json"), settings: domain.DefaultSettings(modelDir)}
	if data, err := os.ReadFile(store.path); err == nil {
		if err := json.Unmarshal(data, &store.settings); err != nil {
			return nil, fmt.Errorf("read settings: %w", err)
		}
		store.settings = migrate(store.settings, modelDir)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	return store, nil
}

func migrate(settings domain.Settings, modelDir string) domain.Settings {
	defaults := domain.DefaultSettings(modelDir)
	if settings.Version < 1 {
		settings = defaults
	}
	if settings.ActiveProvider == "" {
		settings.ActiveProvider = defaults.ActiveProvider
	}
	if settings.Ollama.Endpoint == "" {
		settings.Ollama.Endpoint = defaults.Ollama.Endpoint
	}
	if settings.LlamaCpp.ModelDir == "" {
		settings.LlamaCpp.ModelDir = modelDir
	}
	if settings.LlamaCpp.RuntimeMode == "" {
		settings.LlamaCpp.RuntimeMode = domain.RuntimeAuto
	}
	if settings.LlamaCpp.ContextSize < 1024 {
		settings.LlamaCpp.ContextSize = 8192
	}
	if settings.WhisperCpp.ModelDir == "" {
		settings.WhisperCpp.ModelDir = defaults.WhisperCpp.ModelDir
	}
	if settings.WhisperCpp.RuntimeMode == "" {
		settings.WhisperCpp.RuntimeMode = domain.RuntimeAuto
	}
	if strings.TrimSpace(settings.WhisperCpp.Language) == "" {
		settings.WhisperCpp.Language = "auto"
	}
	if strings.TrimSpace(settings.Prompts.Translation) == "" {
		settings.Prompts.Translation = defaults.Prompts.Translation
	}
	if strings.TrimSpace(settings.Prompts.Detection) == "" {
		settings.Prompts.Detection = defaults.Prompts.Detection
	}
	if strings.TrimSpace(settings.Prompts.OCR) == "" {
		settings.Prompts.OCR = defaults.Prompts.OCR
	}
	if strings.TrimSpace(settings.Prompts.WordVariants) == "" {
		settings.Prompts.WordVariants = defaults.Prompts.WordVariants
	}
	if settings.Version < 5 && (settings.Prompts.WordVariants == domain.LegacyWordVariantsPrompt || settings.Prompts.WordVariants == domain.PreviousWordVariantsPrompt) {
		settings.Prompts.WordVariants = defaults.Prompts.WordVariants
	}
	settings.Version = 7
	return settings
}

func (s *Store) Get() domain.Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

func (s *Store) Save(next domain.Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next = migrate(next, s.settings.LlamaCpp.ModelDir)
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace settings: %w", err)
	}
	s.settings = next
	return nil
}

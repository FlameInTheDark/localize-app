package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FlameInTheDark/localize-app/internal/domain"
)

func LocalModels(root string) ([]domain.ModelInfo, error) {
	models := make([]domain.ModelInfo, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".gguf") || strings.Contains(strings.ToLower(entry.Name()), "mmproj") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		projection := projectionFor(filepath.Dir(path))
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		models = append(models, domain.ModelInfo{ID: strings.TrimSuffix(filepath.ToSlash(relative), ".gguf"), Name: entry.Name(), Path: path, ProjectionPath: projection, Size: info.Size(), ModifiedAt: info.ModTime().Format(time.RFC3339), SupportsVision: projection != ""})
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	return models, err
}

func DeleteLocalModel(root, id string) error {
	id = filepath.FromSlash(strings.TrimSuffix(id, ".gguf"))
	path := filepath.Clean(filepath.Join(root, id+".gguf"))
	relative, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(relative, "..") || filepath.IsAbs(relative) {
		return fmt.Errorf("invalid local model")
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func projectionFor(directory string) string {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return ""
	}
	var projection string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".gguf") && (strings.Contains(name, "mmproj") || strings.Contains(name, "projector")) {
			if projection != "" {
				return "" // A compatible projection cannot be inferred when there is more than one.
			}
			projection = filepath.Join(directory, entry.Name())
		}
	}
	return projection
}

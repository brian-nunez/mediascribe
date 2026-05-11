package artifacts

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Manager struct {
	Root string
}

func (m Manager) JobDir(jobID string) string {
	return filepath.Join(m.Root, jobID)
}

func (m Manager) EnsureJobDir(jobID string) (string, error) {
	dir := m.JobDir(jobID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func WriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteString(path, value string) error {
	return os.WriteFile(path, []byte(value), 0o644)
}

func ReadString(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

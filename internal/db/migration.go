package db

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func (s *Store) RunDefaultMigrations(ctx context.Context) error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("no migrations found")
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, "migrations/"+entry.Name())
	}
	sort.Strings(files)

	for _, file := range files {
		payload, err := migrationFS.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, string(payload)); err != nil {
			return fmt.Errorf("apply %s: %w", file, err)
		}
	}
	return nil
}

func (s *Store) CombinedMigrationSQL() (string, error) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return "", err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, "migrations/"+entry.Name())
	}
	sort.Strings(files)
	chunks := make([]string, 0, len(files))
	for _, file := range files {
		payload, err := migrationFS.ReadFile(file)
		if err != nil {
			return "", err
		}
		chunks = append(chunks, string(payload))
	}
	return strings.Join(chunks, "\n"), nil
}

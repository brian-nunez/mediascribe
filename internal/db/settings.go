package db

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Store) GetGlobalSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM global_settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

func (s *Store) SetGlobalSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO global_settings (key, value, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET
	value = excluded.value,
	updated_at = excluded.updated_at
`, key, value, nowRFC3339())
	return err
}

package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const CurrentSchemaVersion = 1

type migrationMeta struct {
	Version   int       `json:"version"`
	AppliedAt time.Time `json:"applied_at"`
}

type Migration struct {
	Version int
	Name    string
	Apply   func(KVStore) error
}

var migrations = []Migration{}

func RegisterMigration(version int, name string, apply func(KVStore) error) {
	migrations = append(migrations, Migration{
		Version: version,
		Name:    name,
		Apply:   apply,
	})
}

func RunMigrations(store KVStore, dataDir string) error {
	metaPath := filepath.Join(dataDir, "migration.json")

	current := 0
	if data, err := os.ReadFile(metaPath); err == nil {
		var meta migrationMeta
		if err := json.Unmarshal(data, &meta); err == nil {
			current = meta.Version
		}
	}

	if current == CurrentSchemaVersion {
		return nil
	}

	for _, m := range migrations {
		if m.Version <= current {
			continue
		}

		if err := m.Apply(store); err != nil {
			return fmt.Errorf("migration %d (%s) failed: %w", m.Version, m.Name, err)
		}

		meta := migrationMeta{
			Version:   m.Version,
			AppliedAt: time.Now().UTC(),
		}

		data, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal migration meta: %w", err)
		}

		if err := os.WriteFile(metaPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write migration meta: %w", err)
		}
	}

	return nil
}

func CheckSchemaVersion(dataDir string) (int, error) {
	metaPath := filepath.Join(dataDir, "migration.json")

	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		return 0, nil
	}

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read migration meta: %w", err)
	}

	var meta migrationMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return 0, fmt.Errorf("failed to parse migration meta: %w", err)
	}

	return meta.Version, nil
}

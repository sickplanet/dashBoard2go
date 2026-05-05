package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
)

//go:embed *.sql
var fs embed.FS

// Migrate ensures all SQL migration files are natively applied in version order.
// This is the optimal long-term strategy for SQLite updates!
func Migrate(db *sql.DB) error {
	// Ensure the base settings table exists first
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS system_settings (
key TEXT PRIMARY KEY,
value TEXT NOT NULL,
updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`)
	if err != nil {
		return fmt.Errorf("failed to init system_settings: %w", err)
	}

	// Fetch current version
	var currentVersion int
	err = db.QueryRow("SELECT value FROM system_settings WHERE key = 'db_version'").Scan(&currentVersion)
	if err == sql.ErrNoRows {
		currentVersion = 0
		db.Exec("INSERT INTO system_settings (key, value) VALUES ('db_version', '0')")
	} else if err != nil {
		return fmt.Errorf("failed to fetch db_version: %w", err)
	}

	files, err := fs.ReadDir(".")
	if err != nil {
		return fmt.Errorf("failed to read embedded migrations: %w", err)
	}

	var sqlFiles []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".sql") {
			sqlFiles = append(sqlFiles, f.Name())
		}
	}
	sort.Strings(sqlFiles)

	for _, file := range sqlFiles {
		// Expects format like "0001_initial.sql", "0002_add_something.sql"
		parts := strings.Split(file, "_")
		if len(parts) == 0 {
			continue
		}

		fileVer, err := strconv.Atoi(parts[0])
		if err != nil {
			log.Printf("Warning: Migration file %s has invalid version prefix, skipping.", file)
			continue
		}

		if fileVer > currentVersion {
			log.Printf("Applying Database Migration: %s", file)

			content, err := fs.ReadFile(file)
			if err != nil {
				return fmt.Errorf("failed to read migration %s: %w", file, err)
			}

			// Apply file in transaction
			tx, err := db.Begin()
			if err != nil {
				return err
			}

			_, err = tx.Exec(string(content))
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to execute migration %s: %w", file, err)
			}

			_, err = tx.Exec("UPDATE system_settings SET value = ?, updated_at = CURRENT_TIMESTAMP WHERE key = 'db_version'", strconv.Itoa(fileVer))
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to update db_version for %s: %w", file, err)
			}

			err = tx.Commit()
			if err != nil {
				return err
			}
			currentVersion = fileVer
		}
	}

	return nil
}

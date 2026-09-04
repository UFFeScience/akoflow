package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

const DefaultPath = "storage/database.db"

func Open(path string) (*sql.DB, error) {
	return open(path, false)
}

func OpenReadOnly(path string) (*sql.DB, error) {
	return open(path, true)
}

func open(path string, readOnly bool) (*sql.DB, error) {
	if path == "" {
		path = os.Getenv("AKOFLOW_DATABASE_PATH")
	}
	if path == "" {
		path = DefaultPath
	}
	path, err := normalizePath(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	location := (&url.URL{Scheme: "file", Path: path}).String()
	query := "?_busy_timeout=10000&_foreign_keys=on"
	if readOnly {
		// Keep the analytical connection read-only while allowing it to observe
		// new WAL commits made by the operational connection.
		query += "&mode=ro"
	} else {
		journalMode := strings.ToUpper(strings.TrimSpace(os.Getenv("AKOFLOW_SQLITE_JOURNAL_MODE")))
		if journalMode == "" {
			journalMode = "WAL"
		}
		switch journalMode {
		case "WAL", "DELETE", "TRUNCATE", "PERSIST":
		default:
			return nil, fmt.Errorf("unsupported SQLite journal mode %q", journalMode)
		}
		query += "&_journal_mode=" + url.QueryEscape(journalMode) + "&_synchronous=FULL"
	}
	dsn := location + query
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// SQLite has a single writer. A desktop control plane benefits more from
	// deterministic catalog/event commits than from competing pooled writers.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect database: %w", err)
	}
	if readOnly {
		if _, err := db.Exec("PRAGMA query_only = ON"); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("configure read-only database: %w", err)
		}
	}
	return db, nil
}

func normalizePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if len(path) >= 2 {
		if first, last := path[0], path[len(path)-1]; (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			path = path[1 : len(path)-1]
		}
	}
	path = os.ExpandEnv(path)
	if path == "" {
		return "", fmt.Errorf("database path is empty after environment expansion")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve database path: %w", err)
	}
	return filepath.Clean(absolute), nil
}

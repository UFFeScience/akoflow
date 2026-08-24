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
	dsn := location + "?_busy_timeout=10000&_journal_mode=WAL&_foreign_keys=on"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect database: %w", err)
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

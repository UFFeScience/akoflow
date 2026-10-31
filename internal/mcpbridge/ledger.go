package mcpbridge

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ledger struct {
	db        *sql.DB
	namespace string
}

func stateDirectory(path string) (string, error) {
	if path == "" {
		cache, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(cache, "akoflow", "mcp")
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return "", err
	}
	return path, nil
}

func newLedger(directory, namespace string) (*ledger, error) {
	directory, err := stateDirectory(directory)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(directory, "requests.db")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	file.Close()
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS receipts (namespace TEXT NOT NULL, id TEXT NOT NULL, fingerprint TEXT" +
		" NOT NULL, result BLOB, created_at INTEGER NOT NULL, PRIMARY KEY(namespace,id))")
	if err != nil {
		db.Close()
		return nil, err
	}
	return &ledger{db: db, namespace: namespace}, nil
}

func (l *ledger) begin(id, fingerprint string) (*mcp.CallToolResult, error) {
	// A committed reservation survives process crashes. Pending outcomes are never
	// automatically retried, since the daemon may already have accepted the work.
	result, err := l.db.Exec("INSERT OR IGNORE INTO receipts(namespace,id,fingerprint,created_at) VALUES(?,?,?,?)", l.namespace, id, fingerprint, time.Now().Unix())
	if err != nil {
		return nil, errors.New("could not reserve durable request receipt")
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if count == 1 {
		return nil, nil
	}
	var stored string
	var raw []byte
	err = l.db.QueryRow("SELECT fingerprint,result FROM receipts WHERE namespace=? AND id=?", l.namespace, id).Scan(&stored, &raw)
	if err != nil {
		return nil, errors.New("could not read request receipt")
	}
	if stored != fingerprint {
		return nil, errors.New("requestId was already used with different operation arguments")
	}
	if len(raw) == 0 {
		return nil, errors.New("request is pending or its outcome is unknown after interruption; inspect daemon state instead of resubmitting")
	}
	var previous mcp.CallToolResult
	if err = json.Unmarshal(raw, &previous); err != nil {
		return nil, errors.New("invalid saved request receipt")
	}
	var wire struct {
		StructuredContent json.RawMessage `json:"structuredContent"`
	}
	if err = json.Unmarshal(raw, &wire); err != nil {
		return nil, err
	}
	if len(wire.StructuredContent) > 0 {
		if err = decodeNumberJSON(wire.StructuredContent, &previous.StructuredContent); err != nil {
			return nil, err
		}
	}
	return &previous, nil
}

func (l *ledger) finish(id string, result *mcp.CallToolResult) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = l.db.Exec("UPDATE receipts SET result=? WHERE namespace=? AND id=?", raw, l.namespace, id)
	return err
}

func (l *ledger) Close() error { return l.db.Close() }

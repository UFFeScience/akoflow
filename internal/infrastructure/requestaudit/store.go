// Package requestaudit keeps installation HTTP history separate from archived domain data.
package requestaudit

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Store struct{ db *sql.DB }
type Entry struct {
	ID         int64  `json:"id"`
	Time       string `json:"time"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Status     int    `json:"status"`
	DurationMS int64  `json:"durationMs"`
	IP         string `json:"ip"`
	Actor      string `json:"actor"`
	Client     string `json:"client"`
}

func New(db *sql.DB) (*Store, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS api_request_log (
 id INTEGER PRIMARY KEY AUTOINCREMENT, time TEXT NOT NULL, method TEXT NOT NULL,
 path TEXT NOT NULL, status INTEGER NOT NULL, duration_ms INTEGER NOT NULL,
 ip TEXT NOT NULL, actor TEXT NOT NULL, client TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS api_request_log_time ON api_request_log(time,id);
 CREATE TABLE IF NOT EXISTS api_request_log_settings (id INTEGER PRIMARY KEY CHECK(id=1), retention_days INTEGER NOT NULL);
 INSERT OR IGNORE INTO api_request_log_settings VALUES(1,30);`)
	return &Store{db: db}, err
}
func (s *Store) Retention(ctx context.Context) (int, error) {
	var days int
	err := s.db.QueryRowContext(ctx, `SELECT retention_days FROM api_request_log_settings WHERE id=1`).Scan(&days)
	return days, err
}
func (s *Store) Cleanup(ctx context.Context) error {
	days, err := s.Retention(ctx)
	if err != nil {
		return err
	}
	// Bound each transaction so cleanup cannot monopolize the SQLite writer.
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `DELETE FROM api_request_log WHERE id IN (SELECT id FROM api_request_log WHERE time < ? LIMIT 1000)`, cutoff)
	return err
}
func (s *Store) RunCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if err := s.Cleanup(ctx); err != nil && ctx.Err() == nil {
			slog.Error("request audit cleanup failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (s *Store) Routes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/akoflow-api/request-logs/":
			if r.Method != http.MethodGet {
				w.WriteHeader(405)
				return
			}
			s.list(w, r)
		case "/akoflow-api/request-log-settings/":
			s.settings(w, r)
		default:
			next.ServeHTTP(w, r)
		}
	})
}
func respond(w http.ResponseWriter, value any, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		http.Error(w, "request audit storage unavailable", 503)
		return
	}
	_ = json.NewEncoder(w).Encode(value)
}
func (s *Store) list(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			http.Error(w, "limit must be 1–100", 400)
			return
		}
		limit = n
	}
	before := int64(0)
	if v := r.URL.Query().Get("before"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1 {
			http.Error(w, "invalid cursor", 400)
			return
		}
		before = n
	}
	query := `SELECT id,time,method,path,status,duration_ms,ip,actor,client FROM api_request_log`
	args := []any{}
	if before > 0 {
		query += ` WHERE id < ?`
		args = append(args, before)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer rows.Close()
	items := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Time, &e.Method, &e.Path, &e.Status, &e.DurationMS, &e.IP, &e.Actor, &e.Client); err != nil {
			respond(w, nil, err)
			return
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		respond(w, nil, err)
		return
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		next = strconv.FormatInt(items[len(items)-1].ID, 10)
	}
	respond(w, map[string]any{"items": items, "nextCursor": next}, nil)
}
func (s *Store) settings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		days, err := s.Retention(r.Context())
		respond(w, map[string]int{"retentionDays": days}, err)
	case http.MethodPut:
		var input struct {
			RetentionDays int `json:"retentionDays"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || input.RetentionDays < 1 || input.RetentionDays > 3650 {
			http.Error(w, "retentionDays must be 1–3650", 400)
			return
		}
		_, err := s.db.ExecContext(r.Context(), `UPDATE api_request_log_settings SET retention_days=? WHERE id=1`, input.RetentionDays)
		respond(w, input, err)
	default:
		w.Header().Set("Allow", "GET, PUT")
		w.WriteHeader(405)
	}
}

// responseWriter preserves streaming and upgrade capabilities through ResponseController.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *responseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	if status >= 100 && status < 200 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *responseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *responseWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}
func (s *Store) Middleware(next http.Handler, identity func(*http.Request) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		rw := &responseWriter{ResponseWriter: w}
		defer func() {
			status := rw.status
			if status == 0 {
				status = 200
			}
			if recovered := recover(); recovered != nil {
				status = 500
				s.record(r, started, status, identity(r))
				panic(recovered)
			}
			s.record(r, started, status, identity(r))
		}()
		next.ServeHTTP(rw, r)
	})
}
func (s *Store) record(r *http.Request, started time.Time, status int, actor string) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	// Query strings, bodies, cookies and authorization headers are never persisted.
	path := r.URL.Path
	if len(path) > 2048 {
		path = path[:2048]
	}
	client := strings.Map(func(c rune) rune {
		if c < 32 || c == 127 {
			return -1
		}
		return c
	}, r.Header.Get("X-Akoflow-Client"))
	if len(client) > 128 {
		client = client[:128]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = s.db.ExecContext(ctx, `INSERT INTO api_request_log(time,method,path,status,duration_ms,ip,actor,client) VALUES(?,?,?,?,?,?,?,?)`, started.UTC().Format(time.RFC3339Nano), r.Method, path, status, time.Since(started).Milliseconds(), ip, actor, client)
	if err != nil {
		slog.Error("request audit write failed", "error", fmt.Sprint(err))
	}
}

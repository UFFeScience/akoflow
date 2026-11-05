package requestaudit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestRequestsPaginationAndPrivacy(t *testing.T) {
	s := testStore(t)
	h := s.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }), func(*http.Request) string { return "unauthenticated" })
	for i := 0; i < 4; i++ {
		r := httptest.NewRequest("POST", "/akoflow-api/test/?token=secret", strings.NewReader("password=secret"))
		r.RemoteAddr = "192.0.2.5:1234"
		r.Header.Set("Authorization", "Bearer secret")
		r.Header.Set("X-Forwarded-For", "1.1.1.1")
		h.ServeHTTP(httptest.NewRecorder(), r)
	}
	page := func(cursor string) ([]Entry, string) {
		t.Helper()
		w := httptest.NewRecorder()
		s.list(w, httptest.NewRequest("GET", "/?limit=2"+cursor, nil))
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if strings.Contains(w.Body.String(), "secret") {
			t.Fatal("secret persisted")
		}
		var result struct {
			Items []Entry `json:"items"`
			Next  string  `json:"nextCursor"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Items, result.Next
	}
	first, next := page("")
	if len(first) != 2 || next == "" || first[0].ID <= first[1].ID || first[0].IP != "192.0.2.5" || first[0].Status != 401 {
		t.Fatalf("bad page: %+v %s", first, next)
	}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/new", nil))
	second, next := page("&before=" + next)
	if len(second) != 2 || next != "" || second[0].ID >= first[1].ID {
		t.Fatalf("unstable pagination %+v", second)
	}
	for _, query := range []string{"?limit=101", "?before=-2", "?limit=bad"} {
		w := httptest.NewRecorder()
		s.list(w, httptest.NewRequest("GET", "/"+query, nil))
		if w.Code != 400 {
			t.Fatal(query, w.Code)
		}
	}
}
func TestRetentionPersistenceAndCleanup(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	w := httptest.NewRecorder()
	s.settings(w, httptest.NewRequest("PUT", "/", strings.NewReader(`{"retentionDays":7}`)))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	again, err := New(s.db)
	if err != nil {
		t.Fatal(err)
	}
	days, err := again.Retention(ctx)
	if err != nil || days != 7 {
		t.Fatal(days, err)
	}
	h := s.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), func(*http.Request) string { return "anonymous" })
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	_, err = s.db.Exec(`UPDATE api_request_log SET time=?`, time.Now().AddDate(0, 0, -8).UTC().Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM api_request_log`).Scan(&count)
	if count != 0 {
		t.Fatal(count)
	}
	for _, body := range []string{`{"retentionDays":0}`, `{"retentionDays":3651}`, `{"retentionDays":2,"other":true}`} {
		w := httptest.NewRecorder()
		s.settings(w, httptest.NewRequest("PUT", "/", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatal(body, w.Code)
		}
	}
}
func TestStreamingAndPanic(t *testing.T) {
	s := testStore(t)
	h := s.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Fatal(err)
		}
		w.Write([]byte("stream"))
	}), func(*http.Request) string { return "anonymous" })
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/stream", nil))
	if !w.Flushed || w.Body.String() != "stream" {
		t.Fatal(w)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic suppressed")
			}
		}()
		s.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("test") }), func(*http.Request) string { return "anonymous" }).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/panic", nil))
	}()
	var status int
	s.db.QueryRow(`SELECT status FROM api_request_log ORDER BY id DESC LIMIT 1`).Scan(&status)
	if status != 500 {
		t.Fatal(status)
	}
}

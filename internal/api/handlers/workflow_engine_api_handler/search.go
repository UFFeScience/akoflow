package workflow_engine_api_handler

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	domainsearch "github.com/UFFeScience/akoflow/internal/domain/search"
)

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q, err := parseSearchQuery(
		r.URL.Query().Get("q"), r.URL.Query().Get("limit"),
		r.URL.Query().Get("types"), r.URL.Query().Get("cursor"),
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if q.Text == "" {
		writeJSON(w, http.StatusOK, domainsearch.Page{Results: []domainsearch.Result{}})
		return
	}
	if h.search == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("search is unavailable"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 800*time.Millisecond)
	defer cancel()
	page, err := h.search.Search(ctx, q)
	if err != nil {
		status := http.StatusServiceUnavailable
		if errors.Is(err, domainsearch.ErrInvalidCursor) {
			status = http.StatusBadRequest
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		}
		writeError(w, status, errors.New("search could not be completed; retry with a more specific query"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, page)
}

func parseSearchQuery(text, limitValue, types, cursor string) (domainsearch.Query, error) {
	q := domainsearch.Query{Text: strings.TrimSpace(text), Types: domainsearch.Types, Limit: 20, Cursor: cursor}
	if !utf8.ValidString(q.Text) || utf8.RuneCountInString(q.Text) > 128 || len(q.Cursor) > 2048 {
		return q, errors.New("search query or cursor is too long")
	}
	if limitValue != "" {
		limit, err := strconv.Atoi(limitValue)
		if err != nil || limit < 1 || limit > 50 {
			return q, errors.New("limit must be between 1 and 50")
		}
		q.Limit = limit
	}
	if raw := types; raw != "" {
		q.Types = nil
		for _, kind := range strings.Split(raw, ",") {
			kind = strings.TrimSpace(kind)
			if !slices.Contains(domainsearch.Types, kind) {
				return q, errors.New("unknown search entity type")
			}
			if !slices.Contains(q.Types, kind) {
				q.Types = append(q.Types, kind)
			}
		}
	}
	return q, nil
}

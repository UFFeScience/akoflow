package search

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	domainsearch "github.com/UFFeScience/akoflow/internal/domain/search"
)

type cursor struct {
	Query string `json:"q"`
	Score int    `json:"s"`
	Type  string `json:"t"`
	ID    string `json:"i"`
}

func queryKey(q domainsearch.Query) string {
	types := slices.Clone(q.Types)
	slices.Sort(types)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ToLower(q.Text)+"\x00"+strings.Join(types, ","))))
}

func decodeCursor(q domainsearch.Query) (cursor, error) {
	var c cursor
	if q.Cursor == "" {
		return c, nil
	}
	if len(q.Cursor) > 2048 {
		return c, domainsearch.ErrInvalidCursor
	}
	data, err := base64.RawURLEncoding.DecodeString(q.Cursor)
	if err != nil {
		return c, domainsearch.ErrInvalidCursor
	}
	if err = json.Unmarshal(data, &c); err != nil || c.Query != queryKey(q) || c.Score < 1 || c.Score > 3 || c.ID == "" || projections[c.Type] == "" {
		return c, domainsearch.ErrInvalidCursor
	}
	return c, nil
}

func encodeCursor(q domainsearch.Query, item domainsearch.Result) string {
	data, _ := json.Marshal(cursor{Query: queryKey(q), Score: item.Score, Type: item.Type, ID: item.ID})
	return base64.RawURLEncoding.EncodeToString(data)
}

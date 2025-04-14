package repository

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/ovvesley/akoflow/pkg/server/model"
)

type RqliteDatabase struct {
	BaseURL string
}

var CREATED_TABLES = []string{}

func GetInstance() *RqliteDatabase {
	return &RqliteDatabase{
		BaseURL: fmt.Sprintf("http://%s:%s", os.Getenv("DATABASE_HOST"), os.Getenv("DATABASE_HTTP_PORT")),
	}
}

func (r *RqliteDatabase) Exec(statement string) (map[string]any, error) {
	body := []string{statement} // array direto de strings

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(
		r.BaseURL+"/db/execute?pretty&timings",
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rqlite HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %s", string(raw))
	}

	return result, nil
}

func (r *RqliteDatabase) Query(statement string) (map[string]any, error) {
	body := []string{statement}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(
		r.BaseURL+"/db/query?pretty&timings",
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rqlite HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %s", string(raw))
	}

	return result, nil
}

func (r *RqliteDatabase) CreateOrVerifyTable(m model.Model) error {
	tableName := getTableName(m)

	if tableExists(tableName) {
		return nil
	}

	stmt := GenerateCreateTableSQL(tableName, m)
	_, err := r.Exec(stmt)
	if err == nil {
		CREATED_TABLES = append(CREATED_TABLES, tableName)
	}
	return err
}

func tableExists(tableName string) bool {
	if len(CREATED_TABLES) == 0 {
		return false
	}

	for _, t := range CREATED_TABLES {
		if t == tableName {
			return true
		}
	}
	return false
}

func getTableName(model model.Model) string {
	return model.TableName()
}

func GenerateCreateTableSQL(tableName string, model any) string {
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	var columns []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		colName := field.Tag.Get("db")
		sqlType := mapGoTypeToSQLite(field.Type.Name())

		extras := field.Tag.Get("sql") // ex: PRIMARY KEY AUTOINCREMENT
		if extras != "" {
			sqlType += " " + extras
		}

		columns = append(columns, fmt.Sprintf("%s %s", colName, sqlType))
	}

	stmt := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s);", tableName, strings.Join(columns, ", "))
	return stmt
}

func mapGoTypeToSQLite(goType string) string {
	switch goType {
	case "int", "int64":
		return "INTEGER"
	case "float32", "float64":
		return "REAL"
	case "bool":
		return "BOOLEAN"
	case "string":
		return "TEXT"
	default:
		return "TEXT" // fallback
	}
}

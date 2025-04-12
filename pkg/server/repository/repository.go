package repository

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
)

type RqliteDatabase struct {
	BaseURL string // Ex: "http://localhost:4001"
}

var CREATED_TABLES = []string{}

func GetInstance() *RqliteDatabase {
	return &RqliteDatabase{
		BaseURL: "http://localhost:4001", // Ou variável de ambiente/config
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
	body := []string{statement} // mesma estrutura: array direto de strings

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

func (r *RqliteDatabase) CreateOrVerifyTable(model any) error {
	tableName := getTableName(model)

	if tableExists(tableName) {
		return nil
	}

	stmt := GenerateCreateTableSQL(tableName, model)
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

func getTableName(model any) string {
	if tableNamer, ok := model.(interface{ TableName() string }); ok {
		return tableNamer.TableName()
	}

	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

func parseResponse(resp *http.Response) (map[string]any, error) {
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, err
	}

	return result, nil
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

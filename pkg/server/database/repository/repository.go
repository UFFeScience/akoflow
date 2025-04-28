package repository

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/ovvesley/akoflow/pkg/server/database/model"
)

type Database struct {
	file string
}

const File = "storage/database.db"

var CREATED_TABLES = []string{}

func (d *Database) Connect() *sql.DB {
	projectPath, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	dbPath := filepath.Join(projectPath, "..", "..", File)

	createDirectoryIfNotExists(filepath.Dir(dbPath))

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		panic(err)
	}

	return db
}

func createDirectoryIfNotExists(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.Mkdir(path, 0755)
		if err != nil {
			println("Error creating directory", err.Error())
		}
	}

}

func CreateOrVerifyTable(c *sql.DB, m model.Model) (err error) {
	tableName := m.TableName()
	columns := m.GetColumns()

	if tableExists(tableName) {
		return nil
	}

	columnDefinitions := ""
	for i, col := range columns {
		if i > 0 {
			columnDefinitions += ", "
		}
		columnDefinitions += col
	}

	query := "CREATE TABLE IF NOT EXISTS " + tableName + " (" + columnDefinitions + ")"
	exec, err := c.Exec(query)
	if err != nil {
		return
	}

	_, err = exec.RowsAffected()
	if err != nil {
		return
	}

	CREATED_TABLES = append(CREATED_TABLES, tableName)
	return nil
}

func tableExists(tableName string) bool {
	for _, t := range CREATED_TABLES {
		if t == tableName {
			return true
		}
	}
	return false
}

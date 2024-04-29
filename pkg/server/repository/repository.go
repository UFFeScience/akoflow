package repository

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"os"
)

type Database struct {
	file string
}

const File = "storage/database.db"

var CREATED_TABLES = []string{}

func (d *Database) Connect() *sql.DB {

	createDirectoryIfNotExists("storage")

	db, err := sql.Open("sqlite3", File)

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

func CreateOrVerifyTable(c *sql.DB, tableName string, columns string) (err error) {

	if tableExists(tableName) {
		return nil
	}

	exec, err := c.Exec("CREATE TABLE IF NOT EXISTS " + tableName + columns)
	if err != nil {
		return
	}

	_, err = exec.RowsAffected()
	if err != nil {
		return
	}
	err = c.Close()
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

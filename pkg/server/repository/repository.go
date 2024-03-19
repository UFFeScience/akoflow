package repository

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	file string
}

const File = "db.sqlite"

func (d *Database) Connect() *sql.DB {
	db, err := sql.Open("sqlite3", File)

	if err != nil {
		panic(err)
	}

	return db
}

func CreateOrVerifyTable(c *sql.DB, tableName string, columns string) (err error) {
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
	return nil
}

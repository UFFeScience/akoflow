package connector

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

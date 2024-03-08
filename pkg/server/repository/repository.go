package repository

import (
	"database/sql"
)

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

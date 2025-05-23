package database

import "reflect"

// GenericGetColumns retorna as colunas baseadas no `db` tag.
func GenericGetColumns(obj any) []string {
	typ := reflect.TypeOf(obj)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	var columns []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if dbTag, ok := field.Tag.Lookup("db"); ok {
			columns = append(columns, dbTag)
		}
	}
	return columns
}

// GenericGetPrimaryKey retorna a chave primária baseada no tag `sql`.
func GenericGetPrimaryKey(obj any) string {
	typ := reflect.TypeOf(obj)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if sqlTag, ok := field.Tag.Lookup("sql"); ok && (sqlTag == "PRIMARY KEY" || sqlTag == "PRIMARY KEY AUTOINCREMENT") {
			if dbTag, ok := field.Tag.Lookup("db"); ok {
				return dbTag
			}
		}
	}
	return ""
}

// GenericGetClausulePrimaryKey retorna a cláusula da chave primária baseada no tag `sql`.
func GenericGetClausulePrimaryKey(obj any) string {
	typ := reflect.TypeOf(obj)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if sqlTag, ok := field.Tag.Lookup("sql"); ok && (sqlTag == "PRIMARY KEY" || sqlTag == "PRIMARY KEY AUTOINCREMENT") {
			return sqlTag
		}
	}
	return ""
}

// GenericGetColumnType retorna o tipo da coluna baseado no tag `sql`.
func GenericGetColumnType(obj any, column string) string {
	typ := reflect.TypeOf(obj)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if dbTag, ok := field.Tag.Lookup("db"); ok && dbTag == column {
			if sqlTag, ok := field.Tag.Lookup("sql"); ok {
				return sqlTag
			}
		}
	}
	return "TEXT" // Default type if not found
}

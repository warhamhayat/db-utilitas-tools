package engine

import (
	"database/sql"
	"strconv"

	_ "github.com/microsoft/go-mssqldb"
)

func init() {
	Register("sqlserver", func() Engine { return sqlserverEngine{} })
}

type sqlserverEngine struct{}

func (sqlserverEngine) Name() string { return "sqlserver" }

func (sqlserverEngine) Open(dsn string) (*sql.DB, error) {
	return sql.Open("sqlserver", dsn)
}

func (sqlserverEngine) Placeholder(pos int) string {
	return "@p" + strconv.Itoa(pos)
}

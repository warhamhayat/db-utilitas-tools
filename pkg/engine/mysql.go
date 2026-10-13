package engine

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

func init() {
	Register("mysql", func() Engine { return mysqlEngine{} })
}

// mysqlEngine hanya mengurus cara connect + format placeholder.
type mysqlEngine struct{}

func (mysqlEngine) Name() string { return "mysql" }

func (mysqlEngine) Open(dsn string) (*sql.DB, error) {
	return sql.Open("mysql", dsn)
}

func (mysqlEngine) Placeholder(int) string { return "?" }

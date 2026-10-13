package engine

import (
	"database/sql"
	"strconv"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func init() {
	Register("postgres", func() Engine { return postgresEngine{} })
}

type postgresEngine struct{}

func (postgresEngine) Name() string { return "postgres" }

func (postgresEngine) Open(dsn string) (*sql.DB, error) {
	return sql.Open("pgx", dsn)
}

func (postgresEngine) Placeholder(pos int) string {
	return "$" + strconv.Itoa(pos)
}

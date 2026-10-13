package engine

import (
	"database/sql"
	"strconv"

	_ "github.com/sijms/go-ora/v2"
)

func init() {
	Register("oracle", func() Engine { return oracleEngine{} })
}

type oracleEngine struct{}

func (oracleEngine) Name() string { return "oracle" }

func (oracleEngine) Open(dsn string) (*sql.DB, error) {
	return sql.Open("oracle", dsn)
}

func (oracleEngine) Placeholder(pos int) string {
	return ":" + strconv.Itoa(pos)
}

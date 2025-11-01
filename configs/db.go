package configs

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

func NewDB(logger *zap.SugaredLogger, connStr string) *sqlx.DB {
	db, err := sqlx.Open("postgres", connStr)

	if err != nil {
		logger.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		logger.Fatal(err)
	}

	return db
}

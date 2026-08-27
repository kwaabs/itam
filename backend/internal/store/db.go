package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
)

// NewDB opens a Bun DB backed by Postgres (pgdriver).
func NewDB(dsn string, debug bool) (*bun.DB, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	sqldb.SetMaxOpenConns(20)
	sqldb.SetMaxIdleConns(10)
	sqldb.SetConnMaxLifetime(time.Hour)

	db := bun.NewDB(sqldb, pgdialect.New())
	if debug {
		db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(false)))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	return db, nil
}

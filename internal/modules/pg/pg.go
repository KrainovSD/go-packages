package pg

import (
	"context"
	"fmt"

	_ "embed"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed init.sql
var initSql []byte

func Init(db *pgxpool.Pool) error {
	var err error
	if _, err = db.Exec(context.Background(), string(initSql)); err != nil {
		return fmt.Errorf("init db: %w", err)
	}
	return nil
}

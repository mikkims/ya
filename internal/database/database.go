package database

import (
	"context"
	"database/sql"
	"fmt"
)

// Open открывает и проверяет соединение с базой; вызывающий код закрывает возвращённый пул.
func Open(ctx context.Context, driver, dsn string) (*sql.DB, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}

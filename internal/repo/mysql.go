package repo

import (
	"context"
	"database/sql"
)

type MySQLRepo struct {
	DB *sql.DB
}

func (r *MySQLRepo) Insert(ctx context.Context, code, longURL string) error {
	_, err := r.DB.ExecContext(ctx,
		"INSERT INTO short_links(code, long_url) VALUES(?, ?)",
		code, longURL,
	)
	return err
}

func (r *MySQLRepo) GetLongURL(ctx context.Context, code string) (string, bool, error) {
	var longURL string
	err := r.DB.QueryRowContext(ctx,
		"SELECT long_url FROM short_links WHERE code=? LIMIT 1",
		code,
	).Scan(&longURL)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return longURL, true, nil
}

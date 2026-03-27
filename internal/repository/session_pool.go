package repository

import "github.com/jackc/pgx/v5/pgxpool"

func (r *PostgresSessionRepository) Pool() *pgxpool.Pool {
	return r.pool
}

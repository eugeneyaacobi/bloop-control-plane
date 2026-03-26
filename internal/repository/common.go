package repository

import "github.com/jackc/pgx/v5/pgxpool"

type Repositories struct {
	Pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repositories {
	return &Repositories{Pool: pool}
}

// Package store wraps the pgxpool used by the plugin and exposes
// typed accessors for app_config.
package store

import "github.com/jackc/pgx/v5/pgxpool"

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

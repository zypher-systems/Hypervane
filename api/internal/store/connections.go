package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type Connection struct {
	ID              string
	Name            string
	PVEURL          string
	TLSMode         string
	TLSPin          string
	TokenCiphertext []byte
	TokenNonce      []byte
	Enabled         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (s *Store) ConnectionCount(ctx context.Context) (int, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM connections`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Store) ListConnections(ctx context.Context) ([]Connection, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, pve_url, tls_mode, tls_pin, token_ciphertext, token_nonce, enabled, created_at, updated_at
		FROM connections
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Connection
	for rows.Next() {
		c, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetConnection(ctx context.Context, id string) (Connection, error) {
	return scanConnection(s.pool.QueryRow(ctx, `
		SELECT id, name, pve_url, tls_mode, tls_pin, token_ciphertext, token_nonce, enabled, created_at, updated_at
		FROM connections WHERE id = $1
	`, id))
}

func (s *Store) CreateConnection(ctx context.Context, c Connection) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO connections (id, name, pve_url, tls_mode, tls_pin, token_ciphertext, token_nonce, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, c.ID, c.Name, c.PVEURL, c.TLSMode, nullIfEmpty(c.TLSPin), c.TokenCiphertext, c.TokenNonce, c.Enabled)
	return err
}

func (s *Store) UpdateConnection(ctx context.Context, c Connection) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE connections SET
			name = $2,
			pve_url = $3,
			tls_mode = $4,
			tls_pin = $5,
			token_ciphertext = $6,
			token_nonce = $7,
			enabled = $8,
			updated_at = now()
		WHERE id = $1
	`, c.ID, c.Name, c.PVEURL, c.TLSMode, nullIfEmpty(c.TLSPin), c.TokenCiphertext, c.TokenNonce, c.Enabled)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteConnection(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM connections WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MigrateClusterConfig(ctx context.Context, id string) error {
	n, err := s.ConnectionCount(ctx)
	if err != nil || n > 0 {
		return err
	}
	cfg, err := s.Cluster(ctx)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}
	return s.CreateConnection(ctx, Connection{
		ID:              id,
		Name:            "Default",
		PVEURL:          cfg.PVEURL,
		TLSMode:         cfg.TLSMode,
		TLSPin:          cfg.TLSPin,
		TokenCiphertext: cfg.TokenCiphertext,
		TokenNonce:      cfg.TokenNonce,
		Enabled:         true,
	})
}

func scanConnection(row pgx.Row) (Connection, error) {
	var c Connection
	var pin *string
	err := row.Scan(&c.ID, &c.Name, &c.PVEURL, &c.TLSMode, &pin, &c.TokenCiphertext, &c.TokenNonce, &c.Enabled, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Connection{}, ErrNotFound
	}
	if err != nil {
		return Connection{}, err
	}
	if pin != nil {
		c.TLSPin = *pin
	}
	return c, nil
}

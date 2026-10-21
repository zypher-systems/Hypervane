package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

const (
	UserKindLocal = "local"
	UserKindPVE   = "pve"
)

type Store struct {
	pool *pgxpool.Pool
}

type User struct {
	ID           string
	Username     string
	PasswordHash string
	Kind         string
	PVEUserID    string
	CreatedAt    time.Time
}

type Session struct {
	ID               string
	UserID           string
	TokenHash        string
	ExpiresAt        time.Time
	TicketCiphertext []byte
	TicketNonce      []byte
	CSRFCiphertext   []byte
	CSRFNonce        []byte
	TicketExpiresAt  *time.Time
	ConnectionID     string
}

func (s Session) HasTicket() bool {
	return len(s.TicketCiphertext) > 0
}

type ClusterConfig struct {
	PVEURL          string
	TLSMode         string
	TLSPin          string
	TokenCiphertext []byte
	TokenNonce      []byte
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("pgx pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	s := &Store{pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, schemaSQL)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func (s *Store) UserCount(ctx context.Context) (int, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Store) CreateUser(ctx context.Context, id, username, passwordHash string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, username, password_hash, kind) VALUES ($1, $2, $3, 'local')
	`, id, username, passwordHash)
	return err
}

func (s *Store) UpsertPVEUser(ctx context.Context, id, userid string) (User, error) {
	var u User
	var pveID *string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (id, username, password_hash, kind, pve_userid)
		VALUES ($1, $2, '!', 'pve', $2)
		ON CONFLICT (username) DO UPDATE SET
			kind = 'pve',
			pve_userid = EXCLUDED.pve_userid
		RETURNING id, username, password_hash, kind, pve_userid, created_at
	`, id, userid).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Kind, &pveID, &u.CreatedAt)
	if pveID != nil {
		u.PVEUserID = *pveID
	}
	return u, err
}

func scanUser(row pgx.Row) (User, error) {
	var u User
	var pveID *string
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Kind, &pveID, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	if u.Kind == "" {
		u.Kind = UserKindLocal
	}
	if pveID != nil {
		u.PVEUserID = *pveID
	}
	return u, nil
}

func (s *Store) UserByUsername(ctx context.Context, username string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `
		SELECT id, username, password_hash, kind, pve_userid, created_at FROM users WHERE username = $1
	`, username))
}

func (s *Store) UserByID(ctx context.Context, id string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `
		SELECT id, username, password_hash, kind, pve_userid, created_at FROM users WHERE id = $1
	`, id))
}

func (s *Store) CreateSession(ctx context.Context, sess Session) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (
			id, user_id, token_hash, expires_at,
			ticket_ciphertext, ticket_nonce, csrf_ciphertext, csrf_nonce, ticket_expires_at, connection_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, sess.ID, sess.UserID, sess.TokenHash, sess.ExpiresAt,
		nullBytes(sess.TicketCiphertext), nullBytes(sess.TicketNonce),
		nullBytes(sess.CSRFCiphertext), nullBytes(sess.CSRFNonce),
		sess.TicketExpiresAt, nullIfEmpty(sess.ConnectionID))
	return err
}

func (s *Store) UpdateSessionTicket(ctx context.Context, id string, sess Session) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions SET
			ticket_ciphertext = $2,
			ticket_nonce = $3,
			csrf_ciphertext = $4,
			csrf_nonce = $5,
			ticket_expires_at = $6
		WHERE id = $1
	`, id, nullBytes(sess.TicketCiphertext), nullBytes(sess.TicketNonce),
		nullBytes(sess.CSRFCiphertext), nullBytes(sess.CSRFNonce), sess.TicketExpiresAt)
	return err
}

func (s *Store) SessionByTokenHash(ctx context.Context, tokenHash string) (Session, error) {
	var sess Session
	var connID *string
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, expires_at,
			ticket_ciphertext, ticket_nonce, csrf_ciphertext, csrf_nonce, ticket_expires_at, connection_id
		FROM sessions
		WHERE token_hash = $1 AND expires_at > now()
	`, tokenHash).Scan(
		&sess.ID, &sess.UserID, &sess.TokenHash, &sess.ExpiresAt,
		&sess.TicketCiphertext, &sess.TicketNonce, &sess.CSRFCiphertext, &sess.CSRFNonce, &sess.TicketExpiresAt, &connID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	if connID != nil {
		sess.ConnectionID = *connID
	}
	return sess, nil
}

func (s *Store) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`)
	return err
}

func (s *Store) UpsertCluster(ctx context.Context, cfg ClusterConfig) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO cluster_config (id, pve_url, tls_mode, tls_pin, token_ciphertext, token_nonce, updated_at)
		VALUES (1, $1, $2, $3, $4, $5, now())
		ON CONFLICT (id) DO UPDATE SET
			pve_url = EXCLUDED.pve_url,
			tls_mode = EXCLUDED.tls_mode,
			tls_pin = EXCLUDED.tls_pin,
			token_ciphertext = EXCLUDED.token_ciphertext,
			token_nonce = EXCLUDED.token_nonce,
			updated_at = now()
	`, cfg.PVEURL, cfg.TLSMode, nullIfEmpty(cfg.TLSPin), cfg.TokenCiphertext, cfg.TokenNonce)
	return err
}

func (s *Store) Cluster(ctx context.Context) (ClusterConfig, error) {
	var cfg ClusterConfig
	var pin *string
	err := s.pool.QueryRow(ctx, `
		SELECT pve_url, tls_mode, tls_pin, token_ciphertext, token_nonce
		FROM cluster_config WHERE id = 1
	`).Scan(&cfg.PVEURL, &cfg.TLSMode, &pin, &cfg.TokenCiphertext, &cfg.TokenNonce)
	if errors.Is(err, pgx.ErrNoRows) {
		return ClusterConfig{}, ErrNotFound
	}
	if pin != nil {
		cfg.TLSPin = *pin
	}
	return cfg, err
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

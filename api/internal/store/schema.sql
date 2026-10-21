CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'local',
    pve_userid TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    ticket_ciphertext BYTEA,
    ticket_nonce BYTEA,
    csrf_ciphertext BYTEA,
    csrf_nonce BYTEA,
    ticket_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS cluster_config (
    id INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    pve_url TEXT NOT NULL,
    tls_mode TEXT NOT NULL,
    tls_pin TEXT,
    token_ciphertext BYTEA NOT NULL,
    token_nonce BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS connections (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    pve_url TEXT NOT NULL,
    tls_mode TEXT NOT NULL,
    tls_pin TEXT,
    token_ciphertext BYTEA NOT NULL,
    token_nonce BYTEA NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE sessions ADD COLUMN IF NOT EXISTS connection_id UUID;

CREATE INDEX IF NOT EXISTS sessions_expires_at_idx ON sessions (expires_at);

ALTER TABLE users ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'local';
ALTER TABLE users ADD COLUMN IF NOT EXISTS pve_userid TEXT;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS ticket_ciphertext BYTEA;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS ticket_nonce BYTEA;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS csrf_ciphertext BYTEA;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS csrf_nonce BYTEA;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS ticket_expires_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS users_pve_userid_uidx ON users (pve_userid) WHERE pve_userid IS NOT NULL;

CREATE TABLE IF NOT EXISTS node_iops (
    node TEXT NOT NULL,
    ts TIMESTAMPTZ NOT NULL,
    rd DOUBLE PRECISION NOT NULL,
    wr DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (node, ts)
);

CREATE TABLE IF NOT EXISTS guest_iops (
    vmid INTEGER NOT NULL,
    ts TIMESTAMPTZ NOT NULL,
    rd DOUBLE PRECISION NOT NULL,
    wr DOUBLE PRECISION NOT NULL,
    wait DOUBLE PRECISION NOT NULL DEFAULT 0,
    PRIMARY KEY (vmid, ts)
);
ALTER TABLE guest_iops ADD COLUMN IF NOT EXISTS wait DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE node_iops ADD COLUMN IF NOT EXISTS connection_id TEXT NOT NULL DEFAULT '';
ALTER TABLE guest_iops ADD COLUMN IF NOT EXISTS connection_id TEXT NOT NULL DEFAULT '';

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE table_schema = 'public' AND table_name = 'node_iops' AND constraint_name = 'node_iops_pkey'
  ) AND NOT EXISTS (
    SELECT 1 FROM information_schema.key_column_usage
    WHERE table_schema = 'public' AND table_name = 'node_iops' AND constraint_name = 'node_iops_pkey' AND column_name = 'connection_id'
  ) THEN
    ALTER TABLE node_iops DROP CONSTRAINT node_iops_pkey;
    ALTER TABLE node_iops ADD PRIMARY KEY (connection_id, node, ts);
  END IF;
  IF EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE table_schema = 'public' AND table_name = 'guest_iops' AND constraint_name = 'guest_iops_pkey'
  ) AND NOT EXISTS (
    SELECT 1 FROM information_schema.key_column_usage
    WHERE table_schema = 'public' AND table_name = 'guest_iops' AND constraint_name = 'guest_iops_pkey' AND column_name = 'connection_id'
  ) THEN
    ALTER TABLE guest_iops DROP CONSTRAINT guest_iops_pkey;
    ALTER TABLE guest_iops ADD PRIMARY KEY (connection_id, vmid, ts);
  END IF;
END $$;

-- Licensing was removed. Drop any saved key and the install id it was bound to.
DROP TABLE IF EXISTS licenses;
DROP TABLE IF EXISTS install;

CREATE TABLE IF NOT EXISTS audit_events (
    id BIGSERIAL PRIMARY KEY,
    at TIMESTAMPTZ NOT NULL DEFAULT now(),
    username TEXT NOT NULL,
    user_kind TEXT NOT NULL DEFAULT '',
    connection_id TEXT NOT NULL DEFAULT '',
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    status INTEGER NOT NULL,
    upid TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS audit_events_at_idx ON audit_events (at DESC);

CREATE TABLE IF NOT EXISTS metric_history (
    connection_id TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    ts TIMESTAMPTZ NOT NULL,
    cpu DOUBLE PRECISION NOT NULL DEFAULT 0,
    mem DOUBLE PRECISION NOT NULL DEFAULT 0,
    maxmem DOUBLE PRECISION NOT NULL DEFAULT 0,
    disk DOUBLE PRECISION NOT NULL DEFAULT 0,
    maxdisk DOUBLE PRECISION NOT NULL DEFAULT 0,
    netin DOUBLE PRECISION NOT NULL DEFAULT 0,
    netout DOUBLE PRECISION NOT NULL DEFAULT 0,
    PRIMARY KEY (connection_id, resource_id, ts)
);
CREATE INDEX IF NOT EXISTS metric_history_ts_idx ON metric_history (ts);

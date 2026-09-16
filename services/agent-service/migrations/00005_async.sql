-- +goose Up
ALTER TABLE runs ADD COLUMN webhook_url text;

CREATE TABLE run_jobs (
    run_id uuid PRIMARY KEY REFERENCES runs(id) ON DELETE CASCADE,
    available_at timestamptz NOT NULL,
    locked_by text,
    locked_until timestamptz,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    created_at timestamptz NOT NULL
);
CREATE INDEX run_jobs_available ON run_jobs (available_at, run_id);

CREATE TABLE webhook_deliveries (
    id uuid PRIMARY KEY,
    run_id uuid NOT NULL REFERENCES runs(id),
    api_key_id uuid NOT NULL REFERENCES api_keys(id),
    url text NOT NULL,
    event text NOT NULL CHECK (event IN ('run.completed', 'run.failed', 'run.cancelled')),
    payload jsonb NOT NULL,
    status text NOT NULL CHECK (status IN ('pending', 'delivered', 'failed')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at timestamptz,
    locked_until timestamptz,
    last_status_code integer CHECK (last_status_code BETWEEN 100 AND 599),
    last_error text,
    delivered_at timestamptz,
    created_at timestamptz NOT NULL
);
CREATE INDEX webhook_deliveries_due ON webhook_deliveries (status, next_attempt_at, id);
CREATE INDEX webhook_deliveries_run ON webhook_deliveries (run_id, created_at DESC, id DESC);

CREATE TABLE idempotency_keys (
    principal_kind text NOT NULL CHECK (principal_kind IN ('user', 'api_key')),
    principal_id uuid NOT NULL,
    key text NOT NULL CHECK (length(key) BETWEEN 1 AND 255),
    request_hash bytea NOT NULL CHECK (length(request_hash) = 32),
    run_id uuid NOT NULL REFERENCES runs(id) DEFERRABLE INITIALLY DEFERRED,
    response_status integer NOT NULL CHECK (response_status IN (200, 202)),
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (principal_kind, principal_id, key)
);
CREATE INDEX idempotency_keys_expiry ON idempotency_keys (expires_at);

-- +goose Down
DROP TABLE idempotency_keys;
DROP TABLE webhook_deliveries;
DROP TABLE run_jobs;
ALTER TABLE runs DROP COLUMN webhook_url;

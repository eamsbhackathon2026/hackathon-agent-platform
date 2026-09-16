-- +goose Up
CREATE TABLE users (
    id uuid PRIMARY KEY,
    email text NOT NULL,
    password_hash text NOT NULL,
    name text NOT NULL,
    role text NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    status text NOT NULL CHECK (status IN ('active', 'disabled')),
    must_change_password boolean NOT NULL DEFAULT false,
    last_login_at timestamptz,
    created_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX users_email_unique ON users (lower(email));
CREATE INDEX users_page ON users (created_at DESC, id DESC);
CREATE TABLE refresh_tokens (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    family_id uuid NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    replaced_by uuid,
    created_at timestamptz NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id),
    UNIQUE (user_id, family_id, id),
    FOREIGN KEY (user_id, family_id, replaced_by) REFERENCES refresh_tokens(user_id, family_id, id)
);
CREATE INDEX refresh_tokens_family ON refresh_tokens (family_id);
CREATE INDEX refresh_tokens_user ON refresh_tokens (user_id);
CREATE TABLE api_keys (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    prefix text NOT NULL UNIQUE,
    key_hash bytea NOT NULL,
    scopes text[] NOT NULL CHECK (scopes <@ ARRAY['runs:write', 'runs:read']::text[]),
    webhook_secret_ciphertext bytea NOT NULL,
    created_by uuid NOT NULL,
    last_used_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL,
    FOREIGN KEY (created_by) REFERENCES users(id)
);
CREATE INDEX api_keys_page ON api_keys (created_at DESC, id DESC);

-- +goose Down
DROP TABLE api_keys;
DROP TABLE refresh_tokens;
DROP TABLE users;

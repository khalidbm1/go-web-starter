CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL UNIQUE,
    salt bytea NOT NULL,
    password_hash bytea NOT NULL
);
CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL
);
CREATE INDEX sessions_expiry ON sessions(expires_at);
CREATE TABLE records (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title text NOT NULL CHECK (length(title) BETWEEN 1 AND 160),
    notes text NOT NULL DEFAULT '' CHECK (length(notes) <= 4000),
    status text NOT NULL DEFAULT 'planned' CHECK (status IN ('planned','active','done')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX records_owner_order ON records(owner_id, created_at DESC, id DESC);
CREATE TABLE auth_windows (
    bucket bytea PRIMARY KEY,
    began_at timestamptz NOT NULL,
    attempts integer NOT NULL
);
CREATE INDEX auth_windows_age ON auth_windows(began_at);

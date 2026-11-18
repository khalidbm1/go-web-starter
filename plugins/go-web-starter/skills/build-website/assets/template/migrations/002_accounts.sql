ALTER TABLE users
  ADD COLUMN role text NOT NULL DEFAULT 'user' CHECK (role IN ('user','admin')),
  ADD COLUMN active boolean NOT NULL DEFAULT true,
  ADD COLUMN display_name text NOT NULL DEFAULT '' CHECK (length(display_name) <= 80),
  ADD COLUMN bio text NOT NULL DEFAULT '' CHECK (length(bio) <= 500),
  ADD COLUMN created_at timestamptz NOT NULL DEFAULT now();
CREATE INDEX users_signup_order ON users(created_at DESC, id DESC);

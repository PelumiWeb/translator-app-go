-- Accounts (milestone 13, ADR 0015). A user signs in through an identity
-- (Sign in with Apple now; Google and email later) and is then known to the
-- server by a session: a random token the phone keeps, of which the server
-- stores only a hash.
CREATE TABLE users (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name text NOT NULL DEFAULT '',
    -- The ID others find this user by. Stored in lower case, so uniqueness
    -- ignores case; NULL until chosen.
    handle       text UNIQUE CHECK (handle = lower(handle)),
    language     text NOT NULL DEFAULT '', -- BCP 47: the language they speak
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE identities (
    provider   text NOT NULL CHECK (provider IN ('apple', 'google', 'email')),
    subject    text NOT NULL, -- the provider's stable ID for the person
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email      text,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, subject)
);
CREATE INDEX identities_user ON identities (user_id);

CREATE TABLE sessions (
    token_hash   bytea PRIMARY KEY, -- SHA-256 of the token; the token itself is never stored
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user ON sessions (user_id);

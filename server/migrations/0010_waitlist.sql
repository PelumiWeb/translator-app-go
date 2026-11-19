-- The waitlist on the website (ADR 0030): people who asked to try Shinar
-- before it is on the stores. Kept apart from accounts: they have not signed
-- up, and most never will until they are invited.
CREATE TABLE waitlist (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email      text NOT NULL,
    name       text NOT NULL DEFAULT '',
    phone      text NOT NULL CHECK (phone IN ('iphone', 'android')),
    created_at timestamptz NOT NULL DEFAULT now(),
    -- When the owner took them for an invitation; NULL while they wait.
    invited_at timestamptz
);
-- One place each, however the address is capitalised.
CREATE UNIQUE INDEX waitlist_email ON waitlist (lower(email));
CREATE INDEX waitlist_waiting ON waitlist (phone, created_at) WHERE invited_at IS NULL;

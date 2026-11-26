-- What App Review asks for, besides block, report and payment (checkpoint
-- 18.11).

-- Sign in with Apple's refresh token for an identity, kept only to revoke it
-- when the account is deleted (Guideline 5.1.1(v), ADR 0018). NULL for an
-- identity signed in before it was kept, or not with Apple.
ALTER TABLE identities ADD COLUMN apple_refresh_token text;

-- The person agreed that their words go to the services that help: which
-- wording they agreed to, and when (Guideline 5.1.2(i)). Empty until then.
ALTER TABLE users ADD COLUMN services_consent text NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN services_consented_at timestamptz;

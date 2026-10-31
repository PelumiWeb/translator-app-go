-- Each user's voice sample (milestone 13, ADR 0016): at most one per user,
-- kept with the version of the consent wording they agreed to, and when.
-- The audio itself is a file in its own directory, apart from job audio,
-- named by blob_key. Deleting a row leaves the file, so the server deletes
-- both.
CREATE TABLE voices (
    user_id         uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    blob_key        text NOT NULL,
    consent_version text NOT NULL,
    consented_at    timestamptz NOT NULL DEFAULT now()
);

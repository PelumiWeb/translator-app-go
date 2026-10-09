-- A client may send the same upload twice: its connection dropped after the
-- server received the request but before the response arrived, so it cannot
-- know whether the job exists. It sends a key of its own choosing with each
-- attempt, and the server creates at most one job per key.
ALTER TABLE jobs ADD COLUMN idempotency_key text;

-- Unique only among rows that have a key: jobs created without one are
-- unaffected. The uniqueness is what makes "at most one" hold even when two
-- attempts arrive at the same moment.
CREATE UNIQUE INDEX jobs_idempotency_key ON jobs (idempotency_key) WHERE idempotency_key IS NOT NULL;

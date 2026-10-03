CREATE TABLE jobs (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status       text NOT NULL CHECK (status IN ('queued', 'processing', 'done', 'failed')),
    source_lang  text NOT NULL,
    audio_path   text NOT NULL,
    attempts     int  NOT NULL DEFAULT 0,
    max_attempts int  NOT NULL DEFAULT 3,
    run_at       timestamptz NOT NULL DEFAULT now(), -- do not run before this time
    locked_until timestamptz,                        -- lease expiry while processing
    result_text  text,
    last_error   text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- Partial index: only queued rows are ever searched by the claim query, so
-- finished jobs do not make it slower.
CREATE INDEX jobs_ready ON jobs (run_at) WHERE status = 'queued';

CREATE TABLE job_events (
    job_id     uuid NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    seq        int  NOT NULL, -- 1, 2, 3 ... per job; used as the SSE event id
    type       text NOT NULL,
    payload    jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (job_id, seq)
);

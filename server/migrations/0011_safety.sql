-- Block and report (milestone 16, ADR 0035). Apple asks apps where people
-- contact each other for a way to block someone, and to report them with a
-- timely answer (Guideline 1.2).

-- While either member of a room of two has blocked the other, neither can
-- send into it.
CREATE TABLE blocks (
    blocker_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (blocker_id, blocked_id),
    CHECK (blocker_id <> blocked_id)
);
CREATE INDEX blocks_blocked ON blocks (blocked_id);

-- A report about a person, or one of their messages, for the owner to look
-- into. Reports outlive the accounts they name: deleting either account
-- leaves the report, without the link.
CREATE TABLE reports (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id uuid REFERENCES users(id) ON DELETE SET NULL,
    reported_id uuid REFERENCES users(id) ON DELETE SET NULL,
    -- Who was reported, as "@handle Name" when it was made.
    reported    text NOT NULL,
    reason      text NOT NULL CHECK (reason IN ('spam', 'harassment', 'hate', 'sexual', 'violence', 'self_harm', 'other')),
    note        text NOT NULL DEFAULT '',
    -- The reported message, copied as the reporter saw it, so that deleting it
    -- with its sender's account does not delete what was reported. The copy is
    -- cleared when the report is closed. All NULL for a report about a person.
    message_id   uuid,
    message_kind text,
    message_lang text,
    transcript   text,
    translation  text, -- in the reporter's language, when it was translated
    audio_key    text, -- the copy of the recording, among the reports' files
    created_at  timestamptz NOT NULL DEFAULT now(),
    closed_at   timestamptz,
    outcome     text NOT NULL DEFAULT '' -- what the owner did, in their words
);
CREATE INDEX reports_open ON reports (created_at) WHERE closed_at IS NULL;

-- A suspended account cannot sign in. Suspending it also ends its sessions.
ALTER TABLE users ADD COLUMN suspended_at timestamptz;

-- A fifth kind of job: telling the owner's phones that a report came in.
ALTER TABLE jobs DROP CONSTRAINT jobs_kind_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_kind_check
    CHECK (kind IN ('transcribe', 'synthesize', 'deliver', 'push', 'report'));
ALTER TABLE jobs ADD COLUMN report_id uuid REFERENCES reports(id) ON DELETE CASCADE;

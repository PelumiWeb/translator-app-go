-- Push notifications (checkpoint 14.5, ADR 0026). A signed-in phone gives the
-- server its address at Apple's or Google's push service. The address belongs
-- to the phone's session, so signing out takes it away too: a phone never
-- hears about an account it has signed out of.
CREATE TABLE push_tokens (
    session_hash bytea PRIMARY KEY REFERENCES sessions(token_hash) ON DELETE CASCADE,
    -- 'apns' is Apple's push service; 'apns-sandbox' is its test service,
    -- which builds run from Xcode are registered with; 'fcm' is Firebase
    -- Cloud Messaging, for Android.
    service      text NOT NULL CHECK (service IN ('apns', 'apns-sandbox', 'fcm')),
    -- One app on one phone. If the phone signs in again before its old
    -- session is gone, the address moves to the new session.
    token        text NOT NULL UNIQUE,
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- A fourth kind of job: telling the listeners of one language, on their
-- phones, that a message is ready for them. target_lang says which language;
-- message_id which message.
ALTER TABLE jobs DROP CONSTRAINT jobs_kind_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_kind_check
    CHECK (kind IN ('transcribe', 'synthesize', 'deliver', 'push'));

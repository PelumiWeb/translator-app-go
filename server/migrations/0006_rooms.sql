-- Rooms (milestone 14). A room is a conversation. Its members send voice
-- messages, and each hears the others' in their own language, in the
-- sender's voice. For now a room has two members; group rooms come later.
CREATE TABLE rooms (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- For a room of two: its members' ids, the smaller first, joined by a
    -- space. The same two people always find the same room. NULL for a group.
    pair       text UNIQUE,
    -- Who opened it. Until its first message, only they see it in their list.
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE room_members (
    room_id   uuid NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at timestamptz NOT NULL DEFAULT now(),
    read_at   timestamptz NOT NULL DEFAULT now(), -- messages from others after this are new to them
    PRIMARY KEY (room_id, user_id)
);
CREATE INDEX room_members_user ON room_members (user_id);

-- A voice message as it was said: the sender's recording and their words.
CREATE TABLE messages (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id         uuid NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    -- Deleting an account deletes what its owner said, along with the copies
    -- of it spoken in their voice.
    sender_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    lang            text NOT NULL, -- the language it was said in
    transcript      text NOT NULL,
    audio_key       text NOT NULL,
    duration_ms     int  NOT NULL,
    -- The phone sends the same message again when it cannot tell whether
    -- the first try arrived; the key lets the server keep only one.
    idempotency_key text,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX messages_room ON messages (room_id, created_at);
CREATE UNIQUE INDEX messages_idempotency_key ON messages (sender_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- A message as the listeners of one language hear it: first translated,
-- then spoken in the sender's voice.
CREATE TABLE renditions (
    message_id uuid NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    lang       text NOT NULL,
    status     text NOT NULL CHECK (status IN ('translating', 'speaking', 'ready', 'failed')),
    text       text, -- the translation, once made
    -- Who speaks it, once ready: 'own' is the sender's voice, made for this
    -- language (audio_key); 'original' is the sender's own recording, for
    -- listeners of the language it was said in; 'phone' is the listener's
    -- phone, reading the text, when the sender's voice could not be used.
    voice      text CHECK (voice IN ('own', 'original', 'phone')),
    audio_key  text,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, lang)
);

-- A third kind of job: delivering a message to the listeners of one
-- language. target_lang says which; message_id which message.
ALTER TABLE jobs DROP CONSTRAINT jobs_kind_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_kind_check
    CHECK (kind IN ('transcribe', 'synthesize', 'deliver'));
ALTER TABLE jobs ADD COLUMN message_id uuid REFERENCES messages(id) ON DELETE CASCADE;

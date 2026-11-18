-- Text messages (milestone 15, ADR 0027). A message is now a voice note or
-- typed text. A text message has no recording: its words are the transcript,
-- and its length is 0.
ALTER TABLE messages ADD COLUMN kind text NOT NULL DEFAULT 'voice'
    CHECK (kind IN ('voice', 'text'));
ALTER TABLE messages ALTER COLUMN audio_key DROP NOT NULL;
ALTER TABLE messages ADD CONSTRAINT messages_audio_by_kind
    CHECK ((kind = 'voice') = (audio_key IS NOT NULL));

-- A text message's rendition is ready as soon as its words are, with no
-- voice: renditions.voice stays NULL until someone asks to hear it, and the
-- sender's voice is made then, by the same delivery job.

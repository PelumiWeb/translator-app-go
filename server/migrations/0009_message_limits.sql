-- Daily limits on what one person sends (ADR 0029). The server adds up each
-- sender's messages of the last 24 hours as they send another; this index
-- keeps that a short read.
CREATE INDEX messages_sender_recent ON messages (sender_id, created_at);

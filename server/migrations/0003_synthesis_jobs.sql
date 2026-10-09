-- A second kind of job on the same queue: speech synthesis. A voice sample
-- and a text go in, audio comes out. It uses the same claiming, leases,
-- retries and events as transcription, which is the point of a shared queue.
ALTER TABLE jobs ADD COLUMN kind text NOT NULL DEFAULT 'transcribe'
    CHECK (kind IN ('transcribe', 'synthesize'));

-- For a transcription job, audio_path is the speech to transcribe and
-- source_lang its language. For a synthesis job, audio_path is the voice
-- sample, and these three carry the rest.
ALTER TABLE jobs ADD COLUMN target_lang text; -- language to speak in
ALTER TABLE jobs ADD COLUMN input_text  text; -- what to say
ALTER TABLE jobs ADD COLUMN result_path text; -- the generated audio, once done

-- A synthesis job has no source language.
ALTER TABLE jobs ALTER COLUMN source_lang DROP NOT NULL;

-- Generated audio is deleted some time after it was made; this finds it.
CREATE INDEX jobs_results ON jobs (updated_at) WHERE result_path IS NOT NULL;

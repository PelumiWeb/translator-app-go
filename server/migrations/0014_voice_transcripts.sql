-- The words read in each voice sample. The iPhone shows the passage in the
-- person's own language and sends the words it showed; the voice engine is
-- given them with the sample. Empty for a sample that came without them
-- (recorded before this, or by Android until milestone 22): the English
-- passage, provider.OnboardingPassage.
ALTER TABLE voices ADD COLUMN transcript text NOT NULL DEFAULT '';

-- A speech job's copy of a kept voice keeps the voice's words with it. NULL
-- for other jobs, and for a voice sent with the request (the English passage).
ALTER TABLE jobs ADD COLUMN voice_text text;

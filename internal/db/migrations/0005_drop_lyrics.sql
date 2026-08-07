-- Removes the lyrics column (lyrics feature was removed).
ALTER TABLE songs DROP COLUMN IF EXISTS lyrics;

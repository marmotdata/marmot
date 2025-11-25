-- Remove search_text from users table
DROP INDEX IF EXISTS idx_users_search;
ALTER TABLE users DROP COLUMN IF EXISTS search_text;

-- Remove search_text from teams table
DROP INDEX IF EXISTS idx_teams_search;
ALTER TABLE teams DROP COLUMN IF EXISTS search_text;

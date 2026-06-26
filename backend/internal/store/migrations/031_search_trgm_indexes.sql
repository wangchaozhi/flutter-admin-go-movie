-- Trigram indexes to keep keyword search fast as the catalog grows. The app and
-- admin video search use LOWER(title) LIKE '%kw%' and comment moderation uses
-- content ILIKE '%kw%'; a plain B-tree cannot serve leading-wildcard matches, so
-- those scans degrade to sequential. pg_trgm GIN indexes make them index-backed.
--
-- pg_trgm is a trusted extension (PG13+), so the database owner can install it
-- without superuser. On a managed instance where it is unavailable, this
-- migration will fail loudly rather than silently shipping slow search.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Matches "LOWER(title) LIKE ?" used by AppListVideosHandler and the admin list.
CREATE INDEX IF NOT EXISTS idx_videos_title_trgm
  ON videos USING gin (lower(title) gin_trgm_ops);

-- Matches "content ILIKE ?" used by the admin comment moderation search.
CREATE INDEX IF NOT EXISTS idx_video_comments_content_trgm
  ON video_comments USING gin (content gin_trgm_ops);

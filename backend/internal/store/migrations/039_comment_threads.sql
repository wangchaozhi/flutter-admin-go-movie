-- Threaded replies on video comments. Until now a user could post many
-- top-level comments but only one star rating. We move to a "one editable
-- review per user per video" model (top-level rows, parent_id IS NULL) plus
-- arbitrary replies (parent_id set). reply_to_user_id powers "@nickname" when
-- replying to a reply. like_count is denormalised for cheap reads, mirroring
-- video_danmaku.like_count.
ALTER TABLE video_comments
  ADD COLUMN IF NOT EXISTS parent_id BIGINT,        -- NULL = top-level review, set = reply
  ADD COLUMN IF NOT EXISTS reply_to_user_id BIGINT, -- @someone when replying to a reply
  ADD COLUMN IF NOT EXISTS like_count INT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_video_comments_parent
  ON video_comments(parent_id, id) WHERE parent_id IS NOT NULL;

-- Data migration: the new model allows only ONE top-level row per (video,user).
-- Collapse any extra historical top-level rows into replies under that user's
-- canonical review (non-destructive: their text is preserved). The canonical
-- review is the rating row if present, else the most recent row.
WITH canon AS (
  SELECT video_id, user_id,
    (array_agg(id ORDER BY (rating > 0) DESC, id DESC))[1] AS keep_id
  FROM video_comments
  WHERE parent_id IS NULL
  GROUP BY video_id, user_id
)
UPDATE video_comments c
SET parent_id = canon.keep_id
FROM canon
WHERE c.parent_id IS NULL
  AND c.video_id = canon.video_id
  AND c.user_id = canon.user_id
  AND c.id <> canon.keep_id;

-- Replace the old "one rating per user" partial index with "one top-level
-- review per user". createVideoComment upserts onto this index.
DROP INDEX IF EXISTS uq_video_comments_user_rating;
CREATE UNIQUE INDEX IF NOT EXISTS uq_video_comments_user_review
  ON video_comments (video_id, user_id)
  WHERE parent_id IS NULL;

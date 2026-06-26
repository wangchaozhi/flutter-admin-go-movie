-- A user may comment many times on a video, but should have only ONE star
-- rating. Without this constraint the same user could submit unlimited ratings
-- and inflate (or deflate) a video's average. We keep the latest rating per
-- (video_id, user_id) and demote older rating rows to comment-only (rating = 0)
-- so their text is preserved, then enforce uniqueness going forward.
UPDATE video_comments c
SET rating = 0
WHERE c.rating > 0
  AND c.id < (
    SELECT MAX(c2.id)
    FROM video_comments c2
    WHERE c2.video_id = c.video_id
      AND c2.user_id = c.user_id
      AND c2.rating > 0
  );

-- Partial unique index: only one rating (rating > 0) row per user per video.
-- Comment-only rows (rating = 0) remain unconstrained, so discussion still
-- allows multiple posts. createVideoComment upserts onto this index.
CREATE UNIQUE INDEX IF NOT EXISTS uq_video_comments_user_rating
  ON video_comments (video_id, user_id)
  WHERE rating > 0;

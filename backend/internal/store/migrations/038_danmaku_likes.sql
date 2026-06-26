-- Likes on danmaku (bullet comments). like_count is denormalised onto
-- video_danmaku for cheap reads; danmaku_likes is the source of truth and its
-- UNIQUE(danmaku_id, user_id) makes a like idempotent (one per user per bullet).
ALTER TABLE video_danmaku ADD COLUMN IF NOT EXISTS like_count INT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS danmaku_likes (
  id BIGSERIAL PRIMARY KEY,
  danmaku_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (danmaku_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_danmaku_likes_user ON danmaku_likes (user_id);

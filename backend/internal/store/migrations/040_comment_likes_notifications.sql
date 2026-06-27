-- Likes on comments (reviews and replies). Mirrors danmaku_likes (038): the
-- UNIQUE(comment_id, user_id) makes a like idempotent and like_count on
-- video_comments is the denormalised cached count.
CREATE TABLE IF NOT EXISTS comment_likes (
  id BIGSERIAL PRIMARY KEY,
  comment_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (comment_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_comment_likes_user ON comment_likes (user_id);

-- In-app notifications for the mobile user. A row is created when someone
-- replies to your review/reply or likes your comment. user_id is the recipient,
-- actor_id is who triggered it. root_comment_id anchors navigation: tapping a
-- notification opens the video and scrolls to its comment section.
CREATE TABLE IF NOT EXISTS user_notifications (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,            -- recipient
  actor_id BIGINT NOT NULL,           -- who triggered it
  type TEXT NOT NULL,                 -- 'reply' | 'like'
  video_id BIGINT NOT NULL,
  comment_id BIGINT,                  -- the new reply, or the liked comment
  root_comment_id BIGINT,             -- top-level review the thread belongs to
  content TEXT NOT NULL DEFAULT '',   -- snippet of the reply text
  is_read BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_notifications_user ON user_notifications (user_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_user_notifications_unread ON user_notifications (user_id) WHERE is_read = FALSE;

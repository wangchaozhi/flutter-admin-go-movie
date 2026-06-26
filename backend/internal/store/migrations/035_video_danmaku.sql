-- Bullet comments (danmaku) overlaid on the player timeline. Each row is one
-- comment anchored to a playback position (time_ms). Kept separate from
-- video_comments because the access pattern is range-by-time, the payload is
-- short, and the volume is much higher.
CREATE TABLE IF NOT EXISTS video_danmaku (
  id BIGSERIAL PRIMARY KEY,
  video_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  content VARCHAR(100) NOT NULL,
  time_ms INT NOT NULL DEFAULT 0,        -- playback position, milliseconds
  color INT NOT NULL DEFAULT 16777215,   -- 0xFFFFFF, white
  mode SMALLINT NOT NULL DEFAULT 0,      -- 0 scroll, 1 top, 2 bottom
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Playback fetches a window of danmaku ordered by position, so index by
-- (video_id, time_ms).
CREATE INDEX IF NOT EXISTS idx_video_danmaku_video_time ON video_danmaku (video_id, time_ms);

-- Series (TV shows, anime, variety) group multiple episodes. An episode is just
-- a regular row in `videos` tagged with series_id + episode_number, so every
-- existing capability (transcode, HLS, VIP gating, comments, danmaku, progress)
-- works per-episode for free. Standalone movies keep series_id = 0.
CREATE TABLE IF NOT EXISTS series (
  id BIGSERIAL PRIMARY KEY,
  title VARCHAR(255) NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  cover_key TEXT NOT NULL DEFAULT '',
  category_id INT NOT NULL DEFAULT 0,
  region VARCHAR(128) NOT NULL DEFAULT '',
  release_year INT NOT NULL DEFAULT 0,
  genres JSONB NOT NULL DEFAULT '[]',
  is_vip BOOLEAN NOT NULL DEFAULT FALSE,
  status VARCHAR(32) NOT NULL DEFAULT 'ongoing', -- ongoing | completed | offline
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Tag videos that belong to a series. 0 = standalone (the default), so existing
-- rows keep their current behaviour.
ALTER TABLE videos ADD COLUMN IF NOT EXISTS series_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE videos ADD COLUMN IF NOT EXISTS episode_number INT NOT NULL DEFAULT 0;

-- Series detail lists episodes ordered by number, so index on (series_id, episode_number).
CREATE INDEX IF NOT EXISTS idx_videos_series ON videos (series_id, episode_number);

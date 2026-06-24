CREATE TABLE IF NOT EXISTS video_media_tracks (
  id BIGSERIAL PRIMARY KEY,
  video_id BIGINT NOT NULL,
  source_key TEXT NOT NULL DEFAULT '',
  source_etag TEXT NOT NULL DEFAULT '',
  source_size BIGINT NOT NULL DEFAULT 0,
  track_type VARCHAR(16) NOT NULL,
  stream_index INT NOT NULL,
  stream_position INT NOT NULL,
  codec_name TEXT NOT NULL DEFAULT '',
  language VARCHAR(32) NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  is_forced BOOLEAN NOT NULL DEFAULT FALSE,
  object_key TEXT NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'ready',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_video_media_tracks_video
  ON video_media_tracks(video_id, track_type, status);

CREATE UNIQUE INDEX IF NOT EXISTS idx_video_media_tracks_source_stream
  ON video_media_tracks(video_id, source_key, source_etag, track_type, stream_index);

CREATE TABLE IF NOT EXISTS video_extract_track_tasks (
  id BIGSERIAL PRIMARY KEY,
  video_id BIGINT NOT NULL,
  source_key TEXT NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'processing',
  status_message TEXT NOT NULL DEFAULT '',
  audio_count INT NOT NULL DEFAULT 0,
  subtitle_count INT NOT NULL DEFAULT 0,
  ready_count INT NOT NULL DEFAULT 0,
  failed_count INT NOT NULL DEFAULT 0,
  error_message TEXT NOT NULL DEFAULT '',
  started_at TIMESTAMP,
  finished_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_video_extract_track_tasks_video
  ON video_extract_track_tasks(video_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_video_extract_track_tasks_status
  ON video_extract_track_tasks(status);

-- 视频处理：音轨/字幕提取历史菜单
-- 注意：id 32 已被 021 的「App 管理」占用，这里用 33。
INSERT INTO admin_menus(id, name, path, parent_id, type, permission) VALUES
  (33, 'video:extract-history', '/videos/extracts', 15, 'menu', '')
ON CONFLICT (id) DO UPDATE SET
  name       = EXCLUDED.name,
  path       = EXCLUDED.path,
  parent_id  = EXCLUDED.parent_id,
  type       = EXCLUDED.type,
  permission = EXCLUDED.permission;

UPDATE admin_roles
SET menu_ids = menu_ids || '[33]'::jsonb
WHERE role_key = 'super_admin'
  AND NOT menu_ids @> '[33]'::jsonb;

SELECT setval(pg_get_serial_sequence('admin_menus','id'), COALESCE((SELECT MAX(id) FROM admin_menus), 1));

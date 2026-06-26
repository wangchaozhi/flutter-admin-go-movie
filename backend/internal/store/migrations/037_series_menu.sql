-- Series management menu (child of 视频管理 / id 15) plus its button permissions.
-- Mirrors the video menu seeding in 005_video_menus.sql.
INSERT INTO admin_menus(id, name, path, parent_id, type, permission, icon, sort_order) VALUES
  (41, '剧集',          '/videos/series', 15, 'menu',   '',              'Layers', 5),
  (42, 'series:create', '',               41, 'button', 'series:create', '',       0),
  (43, 'series:edit',   '',               41, 'button', 'series:edit',   '',       0),
  (44, 'series:delete', '',               41, 'button', 'series:delete', '',       0)
ON CONFLICT (id) DO UPDATE SET
  name       = EXCLUDED.name,
  path       = EXCLUDED.path,
  parent_id  = EXCLUDED.parent_id,
  type       = EXCLUDED.type,
  permission = EXCLUDED.permission,
  icon       = EXCLUDED.icon,
  sort_order = EXCLUDED.sort_order;

UPDATE admin_roles
SET menu_ids = menu_ids || '[41,42,43,44]'::jsonb
WHERE role_key = 'super_admin'
  AND NOT menu_ids @> '[41]'::jsonb;

SELECT setval(pg_get_serial_sequence('admin_menus','id'), COALESCE((SELECT MAX(id) FROM admin_menus), 1));

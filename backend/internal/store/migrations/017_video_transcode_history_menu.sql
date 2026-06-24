INSERT INTO admin_menus(id, name, path, parent_id, type, permission) VALUES
  (31, 'video:transcode-history', '/videos/transcodes', 15, 'menu', '')
ON CONFLICT (id) DO UPDATE SET
  name       = EXCLUDED.name,
  path       = EXCLUDED.path,
  parent_id  = EXCLUDED.parent_id,
  type       = EXCLUDED.type,
  permission = EXCLUDED.permission;

UPDATE admin_roles
SET menu_ids = menu_ids || '[31]'::jsonb
WHERE role_key = 'super_admin'
  AND NOT menu_ids @> '[31]'::jsonb;

SELECT setval(pg_get_serial_sequence('admin_menus','id'), COALESCE((SELECT MAX(id) FROM admin_menus), 1));

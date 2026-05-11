INSERT INTO admin_menus(id, name, path, parent_id, type, permission) VALUES
  (15, 'videos',        '/videos',        0,  'menu',   ''),
  (16, 'video:create',  '',               15, 'button', 'video:create'),
  (17, 'video:edit',    '',               15, 'button', 'video:edit'),
  (18, 'video:delete',  '',               15, 'button', 'video:delete')
ON CONFLICT (id) DO UPDATE SET
  name       = EXCLUDED.name,
  path       = EXCLUDED.path,
  parent_id  = EXCLUDED.parent_id,
  type       = EXCLUDED.type,
  permission = EXCLUDED.permission;

-- 给 super_admin 追加视频菜单权限
UPDATE admin_roles
SET menu_ids = menu_ids || '[15,16,17,18]'::jsonb
WHERE role_key = 'super_admin'
  AND NOT menu_ids @> '[15]'::jsonb;

SELECT setval(pg_get_serial_sequence('admin_menus', 'id'), COALESCE((SELECT MAX(id) FROM admin_menus), 1));

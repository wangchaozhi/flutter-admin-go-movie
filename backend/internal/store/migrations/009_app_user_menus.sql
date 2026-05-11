INSERT INTO admin_menus(id, name, path, parent_id, type, permission) VALUES
  (23, 'app-users',        '/app-users', 0,  'menu',   ''),
  (24, 'app_user:create',  '',           23, 'button', 'app_user:create'),
  (25, 'app_user:edit',    '',           23, 'button', 'app_user:edit'),
  (26, 'app_user:delete',  '',           23, 'button', 'app_user:delete')
ON CONFLICT (id) DO UPDATE SET
  name       = EXCLUDED.name,
  path       = EXCLUDED.path,
  parent_id  = EXCLUDED.parent_id,
  type       = EXCLUDED.type,
  permission = EXCLUDED.permission;

UPDATE admin_roles
SET menu_ids = menu_ids || '[23,24,25,26]'::jsonb
WHERE role_key = 'super_admin'
  AND NOT menu_ids @> '[23]'::jsonb;

SELECT setval(pg_get_serial_sequence('admin_menus','id'), COALESCE((SELECT MAX(id) FROM admin_menus), 1));

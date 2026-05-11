INSERT INTO admin_menus(id, name, path, parent_id, type, permission) VALUES
  (19, 'categories',      '/categories', 0,  'menu',   ''),
  (20, 'category:create', '',            19, 'button', 'category:create'),
  (21, 'category:edit',   '',            19, 'button', 'category:edit'),
  (22, 'category:delete', '',            19, 'button', 'category:delete')
ON CONFLICT (id) DO UPDATE SET
  name       = EXCLUDED.name,
  path       = EXCLUDED.path,
  parent_id  = EXCLUDED.parent_id,
  type       = EXCLUDED.type,
  permission = EXCLUDED.permission;

UPDATE admin_roles
SET menu_ids = menu_ids || '[19,20,21,22]'::jsonb
WHERE role_key = 'super_admin'
  AND NOT menu_ids @> '[19]'::jsonb;

SELECT setval(pg_get_serial_sequence('admin_menus','id'), COALESCE((SELECT MAX(id) FROM admin_menus), 1));

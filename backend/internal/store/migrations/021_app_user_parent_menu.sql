INSERT INTO admin_menus(id, name, path, parent_id, type, permission, icon, sort_order) VALUES
  (32, 'App 管理', '/app', 0, 'menu', '', 'Smartphone', 20)
ON CONFLICT (id) DO UPDATE SET
  name       = EXCLUDED.name,
  path       = EXCLUDED.path,
  parent_id  = EXCLUDED.parent_id,
  type       = EXCLUDED.type,
  permission = EXCLUDED.permission,
  icon       = EXCLUDED.icon,
  sort_order = EXCLUDED.sort_order;

UPDATE admin_menus
SET name = '用户',
    parent_id = 32,
    icon = 'Users',
    sort_order = 10
WHERE id = 23;

UPDATE admin_roles
SET menu_ids = menu_ids || '[32]'::jsonb
WHERE menu_ids @> '[23]'::jsonb
  AND NOT menu_ids @> '[32]'::jsonb;

SELECT setval(pg_get_serial_sequence('admin_menus','id'), COALESCE((SELECT MAX(id) FROM admin_menus), 1));

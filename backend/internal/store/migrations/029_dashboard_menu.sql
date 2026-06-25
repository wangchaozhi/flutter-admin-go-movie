-- Dashboard landing page. A top-level leaf menu (sort_order 0 so it sorts
-- first) granted to both seeded roles; the stats endpoint itself only requires
-- a valid admin session.
INSERT INTO admin_menus(id, name, path, parent_id, type, permission, icon, sort_order) VALUES
  (34, '仪表盘', '/dashboard', 0, 'menu', '', 'LayoutDashboard', 0)
ON CONFLICT (id) DO UPDATE SET
  name       = EXCLUDED.name,
  path       = EXCLUDED.path,
  parent_id  = EXCLUDED.parent_id,
  type       = EXCLUDED.type,
  permission = EXCLUDED.permission,
  icon       = EXCLUDED.icon,
  sort_order = EXCLUDED.sort_order;

UPDATE admin_roles
SET menu_ids = menu_ids || '[34]'::jsonb
WHERE role_key IN ('super_admin', 'operator')
  AND NOT menu_ids @> '[34]'::jsonb;

SELECT setval(pg_get_serial_sequence('admin_menus','id'), COALESCE((SELECT MAX(id) FROM admin_menus), 1));

INSERT INTO admin_menus(id, name, path, parent_id, type, permission) VALUES
  (27, 'payments',        '/payments', 0,  'menu',   ''),
  (28, 'payment:product', '',          27, 'button', 'payment:product'),
  (29, 'payment:order',   '',          27, 'button', 'payment:order'),
  (30, 'payment:refund',  '',          27, 'button', 'payment:refund')
ON CONFLICT (id) DO UPDATE SET
  name       = EXCLUDED.name,
  path       = EXCLUDED.path,
  parent_id  = EXCLUDED.parent_id,
  type       = EXCLUDED.type,
  permission = EXCLUDED.permission;

UPDATE admin_roles
SET menu_ids = menu_ids || missing.ids
FROM (
  SELECT
    role.id,
    COALESCE(jsonb_agg(menu_id) FILTER (WHERE NOT role.menu_ids @> to_jsonb(ARRAY[menu_id])), '[]'::jsonb) AS ids
  FROM admin_roles role
  CROSS JOIN (VALUES (27), (28), (29), (30)) AS required(menu_id)
  WHERE role.role_key = 'super_admin'
  GROUP BY role.id
) AS missing
WHERE admin_roles.id = missing.id
  AND missing.ids <> '[]'::jsonb;

SELECT setval(pg_get_serial_sequence('admin_menus','id'), COALESCE((SELECT MAX(id) FROM admin_menus), 1));

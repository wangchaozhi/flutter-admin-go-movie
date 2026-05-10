INSERT INTO admin_menus(id, name, path, parent_id, type, permission) VALUES
  (1, 'dashboard', '/dashboard', 0, 'menu', ''),
  (2, 'system', '/system', 0, 'menu', ''),
  (3, 'user', '/system/user', 2, 'menu', ''),
  (4, 'role', '/system/role', 2, 'menu', ''),
  (5, 'menu', '/system/menu', 2, 'menu', ''),
  (6, 'user:create', '', 3, 'button', 'user:create'),
  (7, 'user:edit', '', 3, 'button', 'user:edit'),
  (8, 'user:delete', '', 3, 'button', 'user:delete'),
  (9, 'role:create', '', 4, 'button', 'role:create'),
  (10, 'role:edit', '', 4, 'button', 'role:edit'),
  (11, 'role:delete', '', 4, 'button', 'role:delete'),
  (12, 'menu:create', '', 5, 'button', 'menu:create'),
  (13, 'menu:edit', '', 5, 'button', 'menu:edit'),
  (14, 'menu:delete', '', 5, 'button', 'menu:delete')
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  path = EXCLUDED.path,
  parent_id = EXCLUDED.parent_id,
  type = EXCLUDED.type,
  permission = EXCLUDED.permission;

INSERT INTO admin_roles(id, name, role_key, menu_ids) VALUES
  (1, 'super admin', 'super_admin', '[1,2,3,4,5,6,7,8,9,10,11,12,13,14]'::jsonb),
  (2, 'operator', 'operator', '[1,2]'::jsonb)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  role_key = EXCLUDED.role_key,
  menu_ids = EXCLUDED.menu_ids;

INSERT INTO admin_users(id, username, password, nickname, role_ids) VALUES
  (1, 'admin', '123456', 'administrator', '[1]'::jsonb),
  (2, 'operator', '123456', 'operator user', '[2]'::jsonb)
ON CONFLICT (id) DO UPDATE SET
  username = EXCLUDED.username,
  password = EXCLUDED.password,
  nickname = EXCLUDED.nickname,
  role_ids = EXCLUDED.role_ids;

INSERT INTO mobile_users(id, username, password, nickname) VALUES
  (1, 'user', '123456', 'mobile user')
ON CONFLICT (id) DO UPDATE SET
  username = EXCLUDED.username,
  password = EXCLUDED.password,
  nickname = EXCLUDED.nickname;

SELECT setval(pg_get_serial_sequence('admin_menus', 'id'), COALESCE((SELECT MAX(id) FROM admin_menus), 1));
SELECT setval(pg_get_serial_sequence('admin_roles', 'id'), COALESCE((SELECT MAX(id) FROM admin_roles), 1));
SELECT setval(pg_get_serial_sequence('admin_users', 'id'), COALESCE((SELECT MAX(id) FROM admin_users), 1));
SELECT setval(pg_get_serial_sequence('mobile_users', 'id'), COALESCE((SELECT MAX(id) FROM mobile_users), 1));

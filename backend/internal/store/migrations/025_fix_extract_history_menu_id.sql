-- 修复 024 的菜单 id 冲突：024 误用了 id 32（已被 021 的「App 管理」占用），
-- 把「App 管理」覆盖成了提取历史，导致 App 用户(23) 被挂到了视频管理下。
-- 这里恢复 id 32 = 「App 管理」，并把提取历史改用 id 33。
-- 对已应用旧版 024 的库做修复，对全新库则是幂等的再次断言。

-- 1. 恢复「App 管理」父菜单
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

-- 2. 确保 App 用户(23) 仍挂在「App 管理」(32) 之下
UPDATE admin_menus
SET name = '用户', parent_id = 32, icon = 'Users', sort_order = 10
WHERE id = 23;

-- 3. 提取历史改用 id 33（旧版 024 在本库已记录为已应用，不会再跑，这里补建）
INSERT INTO admin_menus(id, name, path, parent_id, type, permission) VALUES
  (33, 'video:extract-history', '/videos/extracts', 15, 'menu', '')
ON CONFLICT (id) DO UPDATE SET
  name       = EXCLUDED.name,
  path       = EXCLUDED.path,
  parent_id  = EXCLUDED.parent_id,
  type       = EXCLUDED.type,
  permission = EXCLUDED.permission;

-- 4. 给 super_admin 授予提取历史菜单(33)，并确保仍持有「App 管理」(32)
UPDATE admin_roles
SET menu_ids = menu_ids || '[33]'::jsonb
WHERE role_key = 'super_admin'
  AND NOT menu_ids @> '[33]'::jsonb;

UPDATE admin_roles
SET menu_ids = menu_ids || '[32]'::jsonb
WHERE role_key = 'super_admin'
  AND NOT menu_ids @> '[32]'::jsonb;

SELECT setval(pg_get_serial_sequence('admin_menus','id'), COALESCE((SELECT MAX(id) FROM admin_menus), 1));

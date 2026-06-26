-- Invitation codes for invite-only mobile sign-up. Admins generate codes (with
-- an optional usage cap, expiry and note); the register endpoint consumes one
-- atomically. used_count is bumped on each successful registration; max_uses = 0
-- means unlimited. status flips to 'disabled' to retire a code without deleting
-- its history.
CREATE TABLE IF NOT EXISTS invite_codes (
  id         BIGSERIAL PRIMARY KEY,
  code       VARCHAR(32)  NOT NULL UNIQUE,
  max_uses   INTEGER      NOT NULL DEFAULT 1,
  used_count INTEGER      NOT NULL DEFAULT 0,
  status     VARCHAR(16)  NOT NULL DEFAULT 'active',
  note       VARCHAR(255) NOT NULL DEFAULT '',
  created_by VARCHAR(64)  NOT NULL DEFAULT '',
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_invite_codes_status ON invite_codes(status);

-- Records which invite code a mobile account registered with, for traceability.
ALTER TABLE mobile_users ADD COLUMN IF NOT EXISTS invite_code VARCHAR(32) NOT NULL DEFAULT '';

-- "邀请码" admin module: a page menu plus two button permissions (generate /
-- disable). Listing is open to any authenticated admin like other read views.
INSERT INTO admin_menus(id, name, path, parent_id, type, permission, icon, sort_order) VALUES
  (38, '邀请码',          '/invite-codes', 0,  'menu',   '',              'Ticket', 18),
  (39, 'invite:create',  '',              38, 'button', 'invite:create', '',       0),
  (40, 'invite:disable', '',              38, 'button', 'invite:disable','',       0)
ON CONFLICT (id) DO UPDATE SET
  name       = EXCLUDED.name,
  path       = EXCLUDED.path,
  parent_id  = EXCLUDED.parent_id,
  type       = EXCLUDED.type,
  permission = EXCLUDED.permission,
  icon       = EXCLUDED.icon,
  sort_order = EXCLUDED.sort_order;

UPDATE admin_roles
SET menu_ids = menu_ids || '[38,39,40]'::jsonb
WHERE role_key = 'super_admin'
  AND NOT menu_ids @> '[38]'::jsonb;

SELECT setval(pg_get_serial_sequence('admin_menus','id'), COALESCE((SELECT MAX(id) FROM admin_menus), 1));

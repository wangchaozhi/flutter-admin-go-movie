ALTER TABLE admin_menus
  ADD COLUMN IF NOT EXISTS icon TEXT NOT NULL DEFAULT '';

UPDATE admin_menus
SET parent_id = 15
WHERE id = 19;

UPDATE admin_menus
SET icon = CASE id
  WHEN 1 THEN 'LayoutDashboard'
  WHEN 2 THEN 'Settings'
  WHEN 3 THEN 'Users'
  WHEN 4 THEN 'Shield'
  WHEN 5 THEN 'Menu'
  WHEN 6 THEN 'Plus'
  WHEN 7 THEN 'Pencil'
  WHEN 8 THEN 'Trash2'
  WHEN 9 THEN 'Plus'
  WHEN 10 THEN 'Pencil'
  WHEN 11 THEN 'Trash2'
  WHEN 12 THEN 'Plus'
  WHEN 13 THEN 'Pencil'
  WHEN 14 THEN 'Trash2'
  WHEN 15 THEN 'Clapperboard'
  WHEN 16 THEN 'Plus'
  WHEN 17 THEN 'Pencil'
  WHEN 18 THEN 'Trash2'
  WHEN 19 THEN 'FolderOpen'
  WHEN 20 THEN 'Plus'
  WHEN 21 THEN 'Pencil'
  WHEN 22 THEN 'Trash2'
  WHEN 23 THEN 'Smartphone'
  WHEN 24 THEN 'Plus'
  WHEN 25 THEN 'Pencil'
  WHEN 26 THEN 'Trash2'
  WHEN 27 THEN 'CreditCard'
  WHEN 28 THEN 'BadgeCheck'
  WHEN 29 THEN 'List'
  WHEN 30 THEN 'RotateCcw'
  WHEN 31 THEN 'History'
  ELSE icon
END
WHERE id BETWEEN 1 AND 31;

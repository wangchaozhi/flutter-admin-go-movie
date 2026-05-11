CREATE TABLE IF NOT EXISTS categories (
  id         SERIAL PRIMARY KEY,
  name       VARCHAR(64) NOT NULL UNIQUE,
  sort_order INT         NOT NULL DEFAULT 0,
  created_at TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE videos ADD COLUMN IF NOT EXISTS category_id INT NOT NULL DEFAULT 0;

INSERT INTO categories (id, name, sort_order) VALUES
  (1, '电影',   1),
  (2, '电视剧', 2),
  (3, '综艺',   3),
  (4, '动漫',   4),
  (5, '纪录片', 5)
ON CONFLICT (id) DO NOTHING;

SELECT setval(pg_get_serial_sequence('categories','id'), COALESCE((SELECT MAX(id) FROM categories), 1));

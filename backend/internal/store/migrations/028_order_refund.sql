-- Refund support: record when and which provider refund settled an order. The
-- status column already allows arbitrary short strings, so 'refunded' needs no
-- constraint change.
ALTER TABLE orders
  ADD COLUMN IF NOT EXISTS refunded_at TIMESTAMP NULL,
  ADD COLUMN IF NOT EXISTS refund_id TEXT NOT NULL DEFAULT '';

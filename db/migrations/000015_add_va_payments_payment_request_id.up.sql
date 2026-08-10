-- Instalments against a variable-bill VA had no dedup key: SaveVAPayment
-- recorded an amount with no reference to the vendor's paymentRequestId, so a
-- retried or double-flagged instalment was counted a second time and credited
-- twice. paymentRequestId is Mandatory and unique per payment in the SNAP
-- spec, which makes it the natural key.
--
-- Nullable, because rows written before this migration have no value to
-- backfill and must not be invented. The UNIQUE index is partial for the same
-- reason: several legacy rows can share NULL without colliding.
ALTER TABLE va_payments ADD COLUMN IF NOT EXISTS payment_request_id VARCHAR(128);

CREATE UNIQUE INDEX IF NOT EXISTS uq_va_payments_payment_request_id
    ON va_payments (payment_request_id)
    WHERE payment_request_id IS NOT NULL;

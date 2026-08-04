-- Ad-hoc inquiry rows (the ones VAUsecase.Inquiry creates when a vendor
-- inquires a virtualAccountNo that has no merchant-created VA yet) used to be
-- persisted with status '00'. Everywhere else in the system '00' means PAID:
-- Payment() refuses to settle a VA that is not '03', and Inquiry() now reports
-- a '00' row as a paid bill (4042414). Left as-is, those legacy rows would
-- block the very payment their inquiry was preparing for.
--
-- Scoped to rows that were never paid: a genuinely settled transaction always
-- carries the payment_request_id and paid_amount written by SavePayment.
UPDATE va_transactions
SET status = '03', updated_at = NOW()
WHERE status = '00'
  AND payment_request_id IS NULL
  AND paid_amount IS NULL;

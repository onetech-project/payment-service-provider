-- Deliberately a no-op. The up migration corrects unpaid inquiry rows that
-- were mislabelled as paid; reverting it would re-introduce that corruption,
-- and the rows it touched are indistinguishable from pending rows that were
-- always correct.
SELECT 1;

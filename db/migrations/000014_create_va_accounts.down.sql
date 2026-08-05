-- The up migration only CREATEs a table -- it alters no existing column and
-- rewrites no existing row -- so dropping the table restores the prior schema
-- exactly. Application code must be rolled back alongside it.
DROP TABLE IF EXISTS va_accounts;

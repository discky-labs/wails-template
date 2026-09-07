-- Notes: the worked example for the repository layer.
--
-- created_at is TEXT, not INTEGER, so the file stays readable in a sqlite3
-- shell. It is written in a FIXED-WIDTH UTC layout (see timeLayout in
-- note_sqlite.go) because SQLite compares TEXT byte by byte: a variable-width
-- format such as RFC3339Nano, which trims trailing zeros from the fraction,
-- would sort "…:00.5Z" BEFORE "…:00Z" and silently corrupt the ordering that
-- cursor pagination depends on.
--
-- STRICT rejects a value of the wrong type at write time rather than coercing
-- it, which turns a mapping bug into an error instead of a surprise.
CREATE TABLE IF NOT EXISTS notes (
    id TEXT NOT NULL PRIMARY KEY,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at TEXT NOT NULL
) STRICT;

-- The keyset index. Both the ORDER BY and the row-value WHERE of
-- listNotesAfterSQL are servable from it, so paging stays O(limit) instead of
-- degrading into a full scan as the table grows.
CREATE INDEX IF NOT EXISTS notes_created_at_id_desc
ON notes (created_at DESC, id DESC);

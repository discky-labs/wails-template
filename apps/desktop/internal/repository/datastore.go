package repository

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	// The pure-Go SQLite driver, registered with database/sql as "sqlite".
	// Pure Go is a requirement here, not a preference: a CGO driver would force
	// CGO_ENABLED=1 on Windows, which scripts/wails-build.sh deliberately
	// avoids so Windows does not need a cross-compile toolchain.
	_ "modernc.org/sqlite"
)

// driverName is what modernc.org/sqlite registers itself as.
const driverName = "sqlite"

// dsnParams tunes every connection the pool opens. All four matter:
//
//   - journal_mode(WAL) — readers do not block the writer, and an interrupted
//     write leaves a recoverable log rather than a torn page.
//   - busy_timeout(5000) — wait for a held lock instead of failing at once.
//   - foreign_keys(ON) — SQLite disables enforcement per connection by default,
//     so it has to be asked for on every one.
//   - _txlock=immediate — BeginTx takes the write lock up front. Without it a
//     transaction that reads before it writes can fail on the upgrade with
//     SQLITE_BUSY, which busy_timeout does NOT retry: that is SQLite's deadlock
//     avoidance, not lock contention, and no amount of waiting clears it.
const dsnParams = "?_pragma=journal_mode(WAL)" +
	"&_pragma=busy_timeout(5000)" +
	"&_pragma=foreign_keys(ON)" +
	"&_txlock=immediate"

// dirPerm keeps the database directory owner-only — notes are user content.
const dirPerm = 0o700

// migrationsDir is the embedded directory holding the schema.
const migrationsDir = "migrations"

// The schema travels inside the binary. A desktop app cannot read migrations
// off disk: there is no operator, no working directory it controls, and the
// files would have to be installed alongside it.
//
//go:embed migrations/*.up.sql
var migrationFiles embed.FS

// ErrNestedTransaction is returned when [SQLite.Atomic] is called on a
// DataStore that is already inside a transaction.
var ErrNestedTransaction = errors.New("repository: nested transaction")

// DataStore is how the rest of the app reaches persistence, and the only way to
// obtain a transaction.
//
// It deliberately has no Close: the composition root holds the concrete
// [SQLite] and closes that. A DataStore handed to an [SQLite.Atomic] callback is
// scoped to the transaction and has no business closing the database.
type DataStore interface {
	Notes() NoteRepository
	Atomic(ctx context.Context, fn func(DataStore) error) error
}

// Querier is the smallest surface the repositories need, and is satisfied by
// both *sql.DB and *sql.Tx.
//
// This one interface is what lets a single adapter serve a plain call and a call
// inside a transaction without knowing which it is in — so no repository method
// is ever written twice.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// SQLite is the SQLite-backed [DataStore].
//
// [Open] returns it as a concrete type so the composition root — and only the
// composition root — can Close it.
type SQLite struct {
	db *sql.DB

	// tx is non-nil only on the value passed to an Atomic callback.
	tx *sql.Tx
}

var _ DataStore = (*SQLite)(nil)

// Open connects to the database at path, creating the file and its parent
// directory if needed, and brings the schema up to date.
//
// Migrating here rather than in a separate command is the desktop constraint:
// there is no operator to run one, so the app either migrates on launch or
// cannot open its own data.
func Open(ctx context.Context, path string) (*SQLite, error) {
	if path == "" {
		return nil, errors.New("repository: path must not be empty")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return nil, fmt.Errorf("repository: create %s: %w", dir, err)
	}

	db, err := sql.Open(driverName, path+dsnParams)
	if err != nil {
		return nil, fmt.Errorf("repository: open %s: %w", path, err)
	}

	// sql.Open is lazy — it validates the DSN and nothing else. Ping is what
	// actually opens the file, so it is where a bad path surfaces.
	if err := db.PingContext(ctx); err != nil {
		return nil, errors.Join(
			fmt.Errorf("repository: connect to %s: %w", path, err),
			db.Close(),
		)
	}

	if err := migrate(ctx, db); err != nil {
		return nil, errors.Join(err, db.Close())
	}

	return &SQLite{db: db}, nil
}

// Notes returns the note repository, bound to the transaction if there is one.
func (s *SQLite) Notes() NoteRepository {
	return &sqliteNoteRepository{q: s.querier()}
}

// Atomic runs fn inside a transaction, committing when it returns nil and
// rolling back otherwise. The DataStore passed to fn is backed by the in-flight
// transaction, so reads inside fn see fn's own writes.
//
// This is the ONLY transaction API. There is no exported Begin/Commit/Rollback
// alongside it, because two ways to manage a transaction is one too many — and
// a nested call is rejected rather than silently folded into the outer one,
// which would make an inner "commit" a lie.
func (s *SQLite) Atomic(ctx context.Context, fn func(DataStore) error) error {
	if s.tx != nil {
		return ErrNestedTransaction
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repository: begin transaction: %w", err)
	}

	defer rollback(ctx, tx, "transaction")

	// fn's error is returned unwrapped on purpose: it belongs to the caller's
	// layer, and prefixing it with "repository:" would misattribute it.
	if err := fn(&SQLite{db: s.db, tx: tx}); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("repository: commit transaction: %w", err)
	}

	return nil
}

// Close releases the database handle.
func (s *SQLite) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("repository: close database: %w", err)
	}

	return nil
}

// querier picks the transaction when there is one, the pool otherwise.
func (s *SQLite) querier() Querier {
	if s.tx != nil {
		return s.tx
	}

	return s.db
}

// migrate applies every embedded migration newer than the recorded version.
//
// PRAGMA user_version is the whole version table: it is an integer in the
// database header, it is transactional, and it costs no dependency. A migration
// tool would bring a CLI and a schema_migrations table to a machine with no
// operator to use either.
func migrate(ctx context.Context, db *sql.DB) error {
	names, err := fs.Glob(migrationFiles, migrationsDir+"/*.up.sql")
	if err != nil {
		return fmt.Errorf("repository: list migrations: %w", err)
	}

	// Zero-padded prefixes make lexicographic order numeric order.
	slices.Sort(names)

	var current int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&current); err != nil {
		return fmt.Errorf("repository: read schema version: %w", err)
	}

	for _, name := range names {
		version, err := migrationVersion(name)
		if err != nil {
			return err
		}

		if version <= current {
			continue
		}

		if err := applyMigration(ctx, db, name, version); err != nil {
			return err
		}
	}

	return nil
}

// applyMigration runs one migration and records its version, both in a single
// transaction — so a failure halfway through leaves neither a half-built schema
// nor a version claiming work that did not happen.
//
// A separate function, not a loop body, because a defer belongs to its function:
// deferring the rollback inside the loop would hold every transaction open
// until the last migration finished.
func applyMigration(ctx context.Context, db *sql.DB, name string, version int) error {
	body, err := migrationFiles.ReadFile(name)
	if err != nil {
		return fmt.Errorf("repository: read %s: %w", name, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repository: begin %s: %w", name, err)
	}

	defer rollback(ctx, tx, name)

	if _, err := tx.ExecContext(ctx, string(body)); err != nil {
		return fmt.Errorf("repository: apply %s: %w", name, err)
	}

	// Interpolated, not bound: PRAGMA accepts no bind parameter. Safe because
	// version is an int returned by migrationVersion, so it cannot carry SQL.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, version)); err != nil {
		return fmt.Errorf("repository: record schema version %d: %w", version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("repository: commit %s: %w", name, err)
	}

	slog.InfoContext(ctx, "applied schema migration", "file", name, "version", version)

	return nil
}

// migrationVersion reads the NNNN_ prefix of a migration filename.
func migrationVersion(name string) (int, error) {
	prefix, _, found := strings.Cut(filepath.Base(name), "_")
	if !found {
		return 0, fmt.Errorf("repository: migration %s is not named NNNN_<slug>.up.sql", name)
	}

	version, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("repository: migration %s has a non-numeric version: %w", name, err)
	}

	if version <= 0 {
		return 0, fmt.Errorf("repository: migration %s must have a version above 0", name)
	}

	return version, nil
}

// rollback undoes tx unless it already committed.
//
// Called from a defer, so it cannot return anything. sql.ErrTxDone is the
// expected outcome on the success path — the commit already happened — and is
// not worth reporting.
func rollback(ctx context.Context, tx *sql.Tx, what string) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		slog.WarnContext(ctx, "could not roll back", "operation", what, "error", err)
	}
}

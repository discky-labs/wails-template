package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/raphaeldiscky/wails-template/apps/desktop/internal/entity"
)

// timeLayout is how a timestamp is stored, and it is FIXED WIDTH on purpose.
//
// SQLite compares TEXT byte by byte, so the stored format has to sort
// chronologically by itself. time.RFC3339Nano does not: it trims trailing zeros
// from the fraction, so "…:00.5Z" sorts BEFORE "…:00Z" even though it is later.
// Nine explicit fractional digits, always UTC, cannot do that.
const timeLayout = "2006-01-02T15:04:05.000000000Z"

// Every SQL statement for the notes table, as consts — so the table's entire
// query surface is readable in one place, and a grep can prove no SQL escaped
// into a service.
const (
	noteColumns = `id, title, body, created_at`

	insertNoteSQL = `INSERT INTO notes (` + noteColumns + `) VALUES (?, ?, ?, ?)`

	getNoteByIDSQL = `SELECT ` + noteColumns + ` FROM notes WHERE id = ?`

	// The first page has no cursor to resume from.
	listNotesSQL = `SELECT ` + noteColumns + ` FROM notes
ORDER BY created_at DESC, id DESC
LIMIT ?`

	// Keyset, never OFFSET. The row-value comparison resumes exactly where the
	// previous page ended, so a note written in between cannot shift the window
	// and make a row repeat or disappear — and it is servable from
	// notes_created_at_id_desc, so the cost of page 100 equals the cost of
	// page 1.
	listNotesAfterSQL = `SELECT ` + noteColumns + ` FROM notes
WHERE (created_at, id) < (?, ?)
ORDER BY created_at DESC, id DESC
LIMIT ?`

	deleteNoteSQL = `DELETE FROM notes WHERE id = ?`
)

// sqliteNoteRepository is the SQLite adapter for [NoteRepository].
//
// Unexported: it is only ever reached through [SQLite.Notes], which is what
// keeps the transaction plumbing out of every call site.
type sqliteNoteRepository struct {
	q Querier
}

var _ NoteRepository = (*sqliteNoteRepository)(nil)

// scanner is the one method *sql.Row and *sql.Rows have in common, so a single
// row-mapping function serves both the get and the list paths.
type scanner interface {
	Scan(dest ...any) error
}

// Insert stores n, generating an id and timestamp when they are unset.
func (r *sqliteNoteRepository) Insert(ctx context.Context, n entity.Note) (entity.Note, error) {
	if n.ID == "" {
		n.ID = uuid.NewString()
	}

	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}

	// UTC before storing, and on the returned value too: a caller building a
	// cursor from this note must produce the same bytes the column holds.
	n.CreatedAt = n.CreatedAt.UTC()

	_, err := r.q.ExecContext(ctx, insertNoteSQL,
		n.ID, n.Title, n.Body, n.CreatedAt.Format(timeLayout))
	if err != nil {
		return entity.Note{}, fmt.Errorf("repository: insert note: %w", err)
	}

	return n, nil
}

// GetByID returns one note, or entity.ErrNoteNotFound.
func (r *sqliteNoteRepository) GetByID(ctx context.Context, id string) (entity.Note, error) {
	note, err := scanNote(r.q.QueryRowContext(ctx, getNoteByIDSQL, id))

	// Translated HERE, in the adapter. sql.ErrNoRows must not leave this
	// package, or every caller above starts depending on database/sql.
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Note{}, fmt.Errorf("%w: %s", entity.ErrNoteNotFound, id)
	}

	if err != nil {
		return entity.Note{}, err
	}

	return note, nil
}

// List returns up to limit notes, newest first, resuming after cursor.
func (r *sqliteNoteRepository) List(
	ctx context.Context, cursor entity.Cursor, limit int,
) ([]entity.Note, error) {
	// A floor only. The ceiling is the service's to apply, because the service
	// is the trust boundary — and because clamping here too would eat the
	// limit+1 probe row the service uses to detect a next page.
	if limit <= 0 {
		limit = entity.DefaultNotePageSize
	}

	var (
		rows *sql.Rows
		err  error
	)

	if cursor.IsZero() {
		rows, err = r.q.QueryContext(ctx, listNotesSQL, limit)
	} else {
		rows, err = r.q.QueryContext(ctx, listNotesAfterSQL,
			cursor.CreatedAt.UTC().Format(timeLayout), cursor.ID, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("repository: list notes: %w", err)
	}

	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.WarnContext(ctx, "could not close note rows", "error", closeErr)
		}
	}()

	notes := make([]entity.Note, 0, limit)

	for rows.Next() {
		note, scanErr := scanNote(rows)
		if scanErr != nil {
			return nil, scanErr
		}

		notes = append(notes, note)
	}

	// Checked separately, and not optional: Next returning false means either
	// "no more rows" or "iteration failed", and only Err tells them apart.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: iterate notes: %w", err)
	}

	return notes, nil
}

// Delete removes one note, or reports entity.ErrNoteNotFound.
func (r *sqliteNoteRepository) Delete(ctx context.Context, id string) error {
	result, err := r.q.ExecContext(ctx, deleteNoteSQL, id)
	if err != nil {
		return fmt.Errorf("repository: delete note: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("repository: count rows deleted for note %s: %w", id, err)
	}

	// Deleting nothing is reported rather than ignored: the caller's list is
	// stale, and telling it so beats a success it did not get.
	if affected == 0 {
		return fmt.Errorf("%w: %s", entity.ErrNoteNotFound, id)
	}

	return nil
}

// scanNote maps one row, converting the stored timestamp back to a time.Time.
//
// The Scan error is wrapped with %w rather than replaced, so a caller can still
// match sql.ErrNoRows through it — which GetByID does.
func scanNote(row scanner) (entity.Note, error) {
	var (
		note      entity.Note
		createdAt string
	)

	if err := row.Scan(&note.ID, &note.Title, &note.Body, &createdAt); err != nil {
		return entity.Note{}, fmt.Errorf("repository: scan note: %w", err)
	}

	parsed, err := time.Parse(timeLayout, createdAt)
	if err != nil {
		return entity.Note{}, fmt.Errorf("repository: parse created_at %q: %w", createdAt, err)
	}

	note.CreatedAt = parsed

	return note, nil
}

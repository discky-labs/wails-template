package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/raphaeldiscky/wails-template/apps/desktop/internal/entity"
	"github.com/raphaeldiscky/wails-template/apps/desktop/internal/repository"
)

// openTemp returns a store over a fresh database file.
//
// A real SQLite file in t.TempDir, not a fake and not :memory: — the file is
// what the app actually uses, and an in-memory database behaves differently
// under WAL and across connections. It needs no container, which is why these
// are ordinary unit tests with no build tag.
func openTemp(t *testing.T) *repository.SQLite {
	t.Helper()

	store, err := repository.Open(t.Context(), filepath.Join(t.TempDir(), "data", "test.db"))
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close() error: %v", err)
		}
	})

	return store
}

// insert stores a note with an explicit timestamp, so ordering in a test does
// not depend on the clock's resolution.
func insert(t *testing.T, store repository.DataStore, title string, at time.Time) entity.Note {
	t.Helper()

	note, err := store.Notes().Insert(t.Context(), entity.Note{Title: title, CreatedAt: at})
	if err != nil {
		t.Fatalf("Insert(%q) error: %v", title, err)
	}

	return note
}

// Open runs migrations, so opening an existing database must be a no-op rather
// than a second attempt at CREATE TABLE. This is the test that would catch a
// user_version that is written but never read.
func TestOpenIsIdempotent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "test.db")

	first, err := repository.Open(t.Context(), path)
	if err != nil {
		t.Fatalf("first Open() error: %v", err)
	}

	saved := insert(t, first, "survives", time.Now())

	if err := first.Close(); err != nil {
		t.Fatalf("first Close() error: %v", err)
	}

	second, err := repository.Open(t.Context(), path)
	if err != nil {
		t.Fatalf("second Open() error: %v", err)
	}

	defer func() {
		if err := second.Close(); err != nil {
			t.Errorf("second Close() error: %v", err)
		}
	}()

	// Reopening must not have dropped anything.
	got, err := second.Notes().GetByID(t.Context(), saved.ID)
	if err != nil {
		t.Fatalf("GetByID() after reopen error: %v", err)
	}

	if got.Title != saved.Title {
		t.Errorf("Title after reopen = %q, want %q", got.Title, saved.Title)
	}
}

// A missing row must surface as the entity sentinel, never as sql.ErrNoRows:
// that translation is the adapter's job, and it is what keeps database/sql out
// of every caller above.
func TestGetByIDMissingIsSentinel(t *testing.T) {
	t.Parallel()

	_, err := openTemp(t).Notes().GetByID(t.Context(), "no-such-id")

	if !errors.Is(err, entity.ErrNoteNotFound) {
		t.Errorf("GetByID() error = %v, want entity.ErrNoteNotFound", err)
	}

	if errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetByID() leaked sql.ErrNoRows to the caller: %v", err)
	}
}

// Deleting nothing is an error, so a stale UI learns its list has moved on.
func TestDeleteMissingIsSentinel(t *testing.T) {
	t.Parallel()

	err := openTemp(t).Notes().Delete(t.Context(), "no-such-id")
	if !errors.Is(err, entity.ErrNoteNotFound) {
		t.Errorf("Delete() error = %v, want entity.ErrNoteNotFound", err)
	}
}

// A timestamp must come back exactly as it went in. The stored layout is
// fixed-width text, so this is really a test that no precision is lost in the
// Format/Parse round trip.
func TestInsertRoundTripsTimestamp(t *testing.T) {
	t.Parallel()

	store := openTemp(t)
	want := time.Date(2026, time.March, 4, 5, 6, 7, 123456789, time.UTC)

	saved := insert(t, store, "precise", want)
	if !saved.CreatedAt.Equal(want) {
		t.Errorf("Insert returned CreatedAt = %v, want %v", saved.CreatedAt, want)
	}

	got, err := store.Notes().GetByID(t.Context(), saved.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}

	if !got.CreatedAt.Equal(want) {
		t.Errorf("CreatedAt after round trip = %v, want %v", got.CreatedAt, want)
	}
}

// The ordering property the fixed-width layout exists for.
//
// A sub-second timestamp with trailing zeros must still sort AFTER the whole
// second before it. With time.RFC3339Nano — which trims the fraction — the
// stored text would be "…:01Z" and "…:01.5Z", and SQLite's byte comparison puts
// '.' (0x2E) before 'Z' (0x5A), silently reversing them.
func TestListOrdersFractionalSecondsCorrectly(t *testing.T) {
	t.Parallel()

	store := openTemp(t)
	base := time.Date(2026, time.January, 1, 0, 0, 1, 0, time.UTC)

	insert(t, store, "on the second", base)
	insert(t, store, "half a second later", base.Add(500*time.Millisecond))

	notes, err := store.Notes().List(t.Context(), entity.Cursor{}, 10)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(notes) != 2 {
		t.Fatalf("List() returned %d notes, want 2", len(notes))
	}

	// Newest first.
	if notes[0].Title != "half a second later" {
		t.Errorf("newest note = %q, want %q", notes[0].Title, "half a second later")
	}
}

// Paging must visit every note exactly once. Timestamps are one millisecond
// apart, which a coarse clock could not distinguish on its own.
func TestListPagesWithoutGapsOrDuplicates(t *testing.T) {
	t.Parallel()

	const (
		total    = 25
		pageSize = 10
	)

	store := openTemp(t)
	base := time.Date(2026, time.February, 2, 0, 0, 0, 0, time.UTC)

	for i := range total {
		insert(t, store, fmt.Sprintf("note-%02d", i), base.Add(time.Duration(i)*time.Millisecond))
	}

	seen := make(map[string]int, total)
	cursor := entity.Cursor{}
	pages := 0

	for {
		notes, err := store.Notes().List(t.Context(), cursor, pageSize)
		if err != nil {
			t.Fatalf("List() error on page %d: %v", pages, err)
		}

		if len(notes) == 0 {
			break
		}

		for _, note := range notes {
			seen[note.ID]++
		}

		last := notes[len(notes)-1]
		cursor = entity.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
		pages++

		// A cursor that fails to advance would loop forever; fail instead.
		if pages > total {
			t.Fatalf("paging did not terminate after %d pages", pages)
		}
	}

	if len(seen) != total {
		t.Errorf("paged over %d distinct notes, want %d", len(seen), total)
	}

	for id, count := range seen {
		if count != 1 {
			t.Errorf("note %s appeared %d times, want once", id, count)
		}
	}
}

// A cursor must survive the round trip through its opaque encoding, because
// that is the form the frontend actually hands back.
func TestListResumesFromAnEncodedCursor(t *testing.T) {
	t.Parallel()

	store := openTemp(t)
	base := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)

	insert(t, store, "oldest", base)
	middle := insert(t, store, "middle", base.Add(time.Second))
	insert(t, store, "newest", base.Add(2*time.Second))

	encoded := entity.Cursor{CreatedAt: middle.CreatedAt, ID: middle.ID}.Encode()

	decoded, err := entity.ParseCursor(encoded)
	if err != nil {
		t.Fatalf("ParseCursor(%q) error: %v", encoded, err)
	}

	notes, err := store.Notes().List(t.Context(), decoded, 10)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	// Strictly after the cursor: the cursor row itself must not repeat.
	if len(notes) != 1 {
		t.Fatalf("List() after the middle note returned %d notes, want 1", len(notes))
	}

	if notes[0].Title != "oldest" {
		t.Errorf("note after the cursor = %q, want %q", notes[0].Title, "oldest")
	}
}

// Atomic must roll back everything when the callback fails — the whole reason
// it exists rather than a bare sequence of writes.
func TestAtomicRollsBackOnError(t *testing.T) {
	t.Parallel()

	store := openTemp(t)
	sentinel := errors.New("deliberate failure")

	err := store.Atomic(t.Context(), func(tx repository.DataStore) error {
		_, insertErr := tx.Notes().Insert(t.Context(), entity.Note{Title: "doomed"})
		if insertErr != nil {
			return fmt.Errorf("insert: %w", insertErr)
		}

		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Atomic() error = %v, want the callback's own error", err)
	}

	notes, err := store.Notes().List(t.Context(), entity.Cursor{}, 10)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(notes) != 0 {
		t.Errorf("after a rolled-back Atomic there are %d notes, want 0", len(notes))
	}
}

// inFlightTitle is the note written and read back inside one transaction.
const inFlightTitle = "in flight"

// A read inside Atomic must see the writes made inside it. If the callback's
// DataStore were backed by the pool instead of the transaction, this would find
// nothing — which is exactly the bug Querier prevents.
func TestAtomicSeesItsOwnWrites(t *testing.T) {
	t.Parallel()

	store := openTemp(t)

	err := store.Atomic(t.Context(), func(tx repository.DataStore) error {
		saved, insertErr := tx.Notes().Insert(t.Context(), entity.Note{Title: inFlightTitle})
		if insertErr != nil {
			return fmt.Errorf("insert: %w", insertErr)
		}

		got, getErr := tx.Notes().GetByID(t.Context(), saved.ID)
		if getErr != nil {
			return fmt.Errorf("read inside the transaction: %w", getErr)
		}

		if got.Title != inFlightTitle {
			return fmt.Errorf("read back %q, want %q", got.Title, inFlightTitle)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Atomic() error: %v", err)
	}

	// And it committed.
	notes, err := store.Notes().List(t.Context(), entity.Cursor{}, 10)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(notes) != 1 {
		t.Errorf("after a committed Atomic there are %d notes, want 1", len(notes))
	}
}

// Nesting is rejected rather than quietly joined to the outer transaction,
// which would make the inner "commit" a lie.
func TestAtomicRejectsNesting(t *testing.T) {
	t.Parallel()

	err := openTemp(t).Atomic(t.Context(), func(tx repository.DataStore) error {
		return tx.Atomic(context.Background(), func(repository.DataStore) error {
			return nil
		})
	})

	if !errors.Is(err, repository.ErrNestedTransaction) {
		t.Errorf("nested Atomic() error = %v, want repository.ErrNestedTransaction", err)
	}
}

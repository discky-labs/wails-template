package services_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/raphaeldiscky/wails-template/apps/desktop/internal/entity"
	"github.com/raphaeldiscky/wails-template/apps/desktop/internal/repository"
	"github.com/raphaeldiscky/wails-template/apps/desktop/services"
)

// newNotesService wires the service to a real database in a temp directory.
//
// The dependency is substituted, not Wails: the service is exercised through
// its exported methods with a genuine store underneath, so the SQL, the cursor
// encoding and the service's own logic are all covered by one test.
func newNotesService(t *testing.T) *services.NotesService {
	t.Helper()

	store, err := repository.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("repository.Open() error: %v", err)
	}

	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close() error: %v", err)
		}
	})

	return services.NewNotesService(store)
}

// The frontend is untrusted input, so validation happens before the database.
func TestAddRejectsEmptyTitle(t *testing.T) {
	t.Parallel()

	_, err := newNotesService(t).Add(t.Context(), "   ", "body")
	if !errors.Is(err, entity.ErrNoteTitleEmpty) {
		t.Errorf("Add(blank) error = %v, want entity.ErrNoteTitleEmpty", err)
	}
}

// trimmedTitle is what "  spaced  " must become.
const trimmedTitle = "spaced"

func TestAddTrimsAndStores(t *testing.T) {
	t.Parallel()

	svc := newNotesService(t)

	saved, err := svc.Add(t.Context(), "  spaced  ", "  body  ")
	if err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	if saved.Title != trimmedTitle {
		t.Errorf("Title = %q, want %q", saved.Title, trimmedTitle)
	}

	if saved.ID == "" {
		t.Error("Add() returned a note with no ID")
	}

	got, err := svc.Get(t.Context(), saved.ID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	if got.Title != trimmedTitle {
		t.Errorf("Get() Title = %q, want %q", got.Title, trimmedTitle)
	}
}

// A missing note must reach the caller as the entity sentinel, through the
// service's own wrapping.
func TestGetMissingIsSentinel(t *testing.T) {
	t.Parallel()

	_, err := newNotesService(t).Get(t.Context(), "no-such-id")
	if !errors.Is(err, entity.ErrNoteNotFound) {
		t.Errorf("Get() error = %v, want entity.ErrNoteNotFound", err)
	}
}

// NextCursor is set only while there is more to fetch, and empty on the last
// page — which is the only signal a caller has to stop.
func TestListSignalsMoreWithNextCursor(t *testing.T) {
	t.Parallel()

	svc := newNotesService(t)

	for i := range 3 {
		if _, err := svc.Add(t.Context(), fmt.Sprintf("note-%d", i), ""); err != nil {
			t.Fatalf("Add() error: %v", err)
		}
	}

	first, err := svc.List(t.Context(), "", 2)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	// The probe row must be dropped, not returned.
	if len(first.Notes) != 2 {
		t.Fatalf("first page has %d notes, want 2", len(first.Notes))
	}

	if first.NextCursor == "" {
		t.Fatal("first page has no NextCursor, but a third note exists")
	}

	last, err := svc.List(t.Context(), first.NextCursor, 2)
	if err != nil {
		t.Fatalf("List(cursor) error: %v", err)
	}

	if len(last.Notes) != 1 {
		t.Fatalf("last page has %d notes, want 1", len(last.Notes))
	}

	if last.NextCursor != "" {
		t.Errorf("last page NextCursor = %q, want empty", last.NextCursor)
	}
}

// Regression test. The service asks the repository for limit+1 rows to detect a
// next page. If the repository ALSO clamped the limit to MaxNotePageSize, that
// probe row would be swallowed at exactly the maximum page size and pagination
// would stop one page early — silently, and only at the boundary.
func TestListPaginatesAtTheMaximumPageSize(t *testing.T) {
	t.Parallel()

	svc := newNotesService(t)
	base := time.Date(2026, time.May, 5, 0, 0, 0, 0, time.UTC)

	// One more note than a full page holds. Two batches, because MaxNoteBatch
	// is not larger than MaxNotePageSize.
	notes := make([]entity.Note, 0, entity.MaxNotePageSize+1)
	for i := range entity.MaxNotePageSize + 1 {
		notes = append(notes, entity.Note{
			Title:     fmt.Sprintf("note-%03d", i),
			CreatedAt: base.Add(time.Duration(i) * time.Millisecond),
		})
	}

	for chunk := 0; chunk < len(notes); chunk += entity.MaxNoteBatch {
		end := min(chunk+entity.MaxNoteBatch, len(notes))

		if _, err := svc.AddMany(t.Context(), notes[chunk:end]); err != nil {
			t.Fatalf("AddMany() error: %v", err)
		}
	}

	page, err := svc.List(t.Context(), "", entity.MaxNotePageSize)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(page.Notes) != entity.MaxNotePageSize {
		t.Fatalf("page has %d notes, want %d", len(page.Notes), entity.MaxNotePageSize)
	}

	if page.NextCursor == "" {
		t.Error("NextCursor is empty at the maximum page size, but one more note exists")
	}
}

// An over-large limit is clamped, not rejected: it is a hint from the UI.
func TestListClampsAnAbsurdLimit(t *testing.T) {
	t.Parallel()

	svc := newNotesService(t)

	for i := range 3 {
		if _, err := svc.Add(t.Context(), fmt.Sprintf("note-%d", i), ""); err != nil {
			t.Fatalf("Add() error: %v", err)
		}
	}

	page, err := svc.List(t.Context(), "", 1_000_000)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(page.Notes) != 3 {
		t.Errorf("List(1e6) returned %d notes, want all 3", len(page.Notes))
	}
}

// A malformed cursor is a bug or tampering, never something a user typed, so it
// is an error rather than a silent reset to the first page.
func TestListRejectsAMalformedCursor(t *testing.T) {
	t.Parallel()

	_, err := newNotesService(t).List(t.Context(), "not-a-cursor", 10)
	if !errors.Is(err, entity.ErrCursorInvalid) {
		t.Errorf("List(bad cursor) error = %v, want entity.ErrCursorInvalid", err)
	}
}

// One invalid note must sink the whole batch, and store none of it.
func TestAddManyIsAllOrNothing(t *testing.T) {
	t.Parallel()

	svc := newNotesService(t)

	_, err := svc.AddMany(t.Context(), []entity.Note{
		{Title: "fine"},
		{Title: ""},
	})
	if !errors.Is(err, entity.ErrNoteTitleEmpty) {
		t.Fatalf("AddMany() error = %v, want entity.ErrNoteTitleEmpty", err)
	}

	page, err := svc.List(t.Context(), "", 10)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(page.Notes) != 0 {
		t.Errorf("after a rejected batch there are %d notes, want 0", len(page.Notes))
	}
}

func TestAddManyRejectsAnOversizedBatch(t *testing.T) {
	t.Parallel()

	notes := make([]entity.Note, entity.MaxNoteBatch+1)
	for i := range notes {
		notes[i] = entity.Note{Title: "bulk"}
	}

	_, err := newNotesService(t).AddMany(t.Context(), notes)
	if !errors.Is(err, entity.ErrBatchTooLarge) {
		t.Errorf("AddMany(oversized) error = %v, want entity.ErrBatchTooLarge", err)
	}
}

// An empty batch is a no-op, not an error — and must not return a nil slice
// alongside a nil error.
func TestAddManyAcceptsAnEmptyBatch(t *testing.T) {
	t.Parallel()

	stored, err := newNotesService(t).AddMany(t.Context(), nil)
	if err != nil {
		t.Fatalf("AddMany(nil) error: %v", err)
	}

	if stored == nil {
		t.Error("AddMany(nil) returned a nil slice; want an empty one")
	}
}

func TestDeleteRemovesTheNote(t *testing.T) {
	t.Parallel()

	svc := newNotesService(t)

	saved, err := svc.Add(t.Context(), "doomed", "")
	if err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	if err := svc.Delete(t.Context(), saved.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	if _, err := svc.Get(t.Context(), saved.ID); !errors.Is(err, entity.ErrNoteNotFound) {
		t.Errorf("Get() after Delete error = %v, want entity.ErrNoteNotFound", err)
	}
}
